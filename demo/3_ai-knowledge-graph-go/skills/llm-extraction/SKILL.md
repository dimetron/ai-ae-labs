# LLM Extraction

This project calls an OpenAI-compatible chat-completions API
(Ollama, LiteLLM, vLLM, OpenAI, etc.) to extract subject-predicate-object
triples from text. The LLM client and JSON parsing live in
`internal/extractor/extractor.go`; prompt templates are in
`internal/prompts/prompts.go`.

## When to use this skill

- Adding a new LLM-driven stage.
- Debugging bad or missing triples.
- Switching the API surface (e.g., function-calling, structured outputs).
- Tightening the JSON parser.

## Client shape

```go
type Extractor struct {
    cfg    config.LLMConfig  // model, base_url, api_key, max_tokens, temperature
    client *http.Client      // uses default http.DefaultClient settings
}
```

`callLLM(ctx, systemPrompt, userPrompt) (string, error)` does the wire work:

- POSTs `chatRequest` (model + messages + max_tokens + temperature) to
  `cfg.BaseURL`.
- Sets `Content-Type: application/json` and `Authorization: Bearer <api_key>`.
- Reads the full body, returns the first `choices[0].message.content`.
- Surfaces non-200 responses as
  `fmt.Errorf("API error (status %d): %s", ...)`.

`cfg.BaseURL` is the **full** chat-completions URL
(`http://localhost:11434/v1/chat/completions`), not just an origin. Do not
append `/v1/chat/completions` in code.

## Public LLM-driven methods

All take a `context.Context` and return either a slice of triples or a
map of entity aliases. They all use `callLLM` under the hood.

| Method | Used by stage | Returns |
|--------|---------------|---------|
| `Extract(ctx, text)` | `extract` | `[]Triple` (subject/predicate/object only) |
| `ResolveEntities(ctx, entityList)` | `standardize` (LLM path) | `map[string][]string` (canonical → variants) |
| `InferRelationships(ctx, e1, e2, triplesText)` | `infer` (cross-community) | `[]Triple` (all marked `Inferred: true`) |
| `InferWithinCommunity(ctx, pairsText, triplesText)` | `infer` (intra-community) | `[]Triple` (all marked `Inferred: true`) |

All four funnel through `parseTriples` (or `extractJSONObject` for the
entity-resolution case), so JSON-recovery logic is shared.

## JSON recovery pipeline

LLMs are unreliable JSON emitters. The parser does three passes:

1. **`extractJSONArray(text)`** (or `extractJSONObject` for entity maps)
   - Strips ` ```json ... ``` ` code-fence wrappers.
   - Trims whitespace.
   - If the cleaned text starts with `[` (or `{`), returns it as-is.
   - Otherwise finds the matching bracket by depth-counting.
   - If the array is truncated (no closing `]`), calls
     `reconstructArray` to rebuild from complete `{...}` objects.

2. **`json.Unmarshal`** on the result. On error, fall through.

3. **`fixJSON(text)`** + retry `json.Unmarshal`. Fixes:
   - Unquoted keys: `foo: "bar"` → `"foo": "bar"`.
   - Trailing commas: `,]` and `,}` are stripped.

If both attempts fail, the error wraps both: the second error is primary,
the first is secondary context.

## Predicate length limit

`extractor.LimitPredicateLength(predicate, 3)` is the canonical post-processor:

- Splits on whitespace, keeps the first 3 words.
- Strips a trailing stopword ("a", "the", "of", ...) if present.
- Logs the truncation at debug level.

Apply it everywhere a predicate is produced or modified. The
`extractor`, `standardizer`, and `inference` packages all call it. If
you add a new pipeline stage that produces triples, call it too.

## Adding a new LLM call

1. Add a prompt builder to `internal/prompts/prompts.go` (a `System` and
   a `User` function pair — match the naming convention).
2. Add a method on `*Extractor` that:
   - Calls `e.callLLM(ctx, sys, user)`.
   - Parses with `parseTriples` (or `extractJSONObject` + `json.Unmarshal`).
   - For triples, calls `LimitPredicateLength` on every predicate.
   - Returns the typed result and any error wrapped with context.
3. If the call should mark results as inferred, set
   `triples[i].Inferred = true` after parsing.

Don't add new HTTP transports or SDK dependencies — the project is stdlib
HTTP only.

## Debugging tips

- Run `make debug` to enable `slog.LevelDebug` and have the extract step
  print raw LLM JSON per chunk.
- If the parser fails, paste the raw response into a JSON linter. Most
  failures are: trailing commas, single-quoted strings, or markdown
  fences around a partial array.
- If the LLM hallucinates predicates > 3 words, check
  `LimitPredicateLength` is being called for the new code path.
