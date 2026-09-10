# Go Conventions

Apply these Go-specific rules when editing or creating code in this project. Match the
existing style — do not invent new patterns.

## When to use this skill

- Editing or creating any `.go` file in `cmd/` or `internal/`.
- Adding a new package under `internal/`.
- Reviewing Go code for style/correctness.

## Go version and modules

- Module: `github.com/dimetron/ai-knowledge-graph-go` (see `go.mod`).
- Go version: read from `go.mod` (`go 1.24`). Use features up to that version.
- Dependencies: keep them minimal. The project only uses `github.com/BurntSushi/toml`.
  Do not add new third-party deps unless the user explicitly asks.
- Prefer the standard library. `strings`, `sort`, `encoding/json`, `log/slog`,
  `context`, `os`, `io`, `net/http` are all in active use.

## Package layout

- `cmd/kgraph/` — CLI entry point only. Parses flags, loads config, dispatches to
  `pipeline.New(...).Run(...)` or the `--test` sample path.
- `internal/<pkg>/` — one package per concern. No package imports a sibling package
  it does not need. `internal/domain` is the shared types module — keep it free of
  logic.
- `internal/pipeline/` is the only package that orchestrates other packages.

## Error handling

- Always wrap errors with `%w`: `fmt.Errorf("foo: %w", err)`.
- Return errors, do not `log.Fatal` or `os.Exit` inside library packages. The CLI
  (`cmd/kgraph/main.go`) is the only place that calls `os.Exit`.
- Use `errors`-style sentinel errors sparingly. Prefer wrapped errors with context.
- Validate inputs at package boundaries; trust internal callers.

## Logging

- Use `log/slog` (the structured logger), never `log.Printf` or `fmt.Println` for
  diagnostics. Reserve `fmt.Println` for the human-facing phase banners in
  `internal/pipeline/pipeline.go` (existing pattern).
- Default handler is set in `cmd/kgraph/main.go`. Library code should call
  `slog.Info`, `slog.Warn`, `slog.Debug` directly — do not create package-local
  loggers.
- Debug-only output (raw LLM JSON, etc.) is gated by `state.Debug` and goes to
  `fmt.Printf`, not slog. Match that pattern.

## Naming

- Package names: short, lowercase, no underscores (`chunker`, `extractor`, `viz`).
- Exported types/functions: `PascalCase`. Unexported: `camelCase`.
- Test files: `<file>_test.go` in the same package. Use `package_test` for
  black-box tests.
- Acronyms are all-caps in identifiers (`LLMConfig`, `URL`, not `LlMConfig`).

## Types and structs

- Struct tags use backtick syntax. TOML: `toml:"snake_case"`. JSON:
  `json:"snake_case,omitempty"`.
- Pointer receivers are used in `Pipeline` (because the methods mutate state) and
  `Extractor` (holds a client). Value receivers are used for stateless helpers.
- Prefer `map[K]V` with `bool` values for sets:
  `existing := make(map[string]bool)`. The codebase uses this pattern consistently.

## Slices and maps

- Initialize maps with capacity when size is known: `make(map[string]int, len(items))`.
- Slices grow with `append`. Don't pre-allocate unless length is known.
- For deterministic output, sort slices before iteration. Use `sort.Slice` with a
  less function, or `sort.Strings` for string slices.

## JSON

- `json.MarshalIndent` for human-readable output (used for both `.json` artifacts
  and debug dumps).
- Wrap parsing errors with context. The `extractor` package has a `parseTriples`
  helper that handles malformed LLM output — use it instead of writing your own
  parser.

## Common pitfalls in this codebase

- The `predicates` map in `internal/inference/inference.go` uses `[2]string` keys
  (ordered pair). Be careful: `["a","b"]` and `["b","a"]` are different keys.
- `viz.Render` accepts `edgeSmooth` as `interface{}` because TOML may decode it
  as `bool` or `string`. Call `resolveEdgeSmooth` (already inside the package)
  to normalize.
- `config.Load` applies defaults for zero values. Don't re-default in callers.

## Verification

After any Go edit, run from the project root:

```bash
go build ./...
go vet ./...
go test ./...
```

`go vet` must be clean. The project has no test files yet — `go test` exits 0
with `[no test files]`, which is expected.
