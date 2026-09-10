# Project Architecture

The project is an LLM-driven pipeline that turns unstructured text into an
interactive HTML knowledge graph. It is a Go re-implementation of
[robert-mcdermott/ai-knowledge-graph](https://github.com/robert-mcdermott/ai-knowledge-graph)
modeled on the Google ADK v2 `workflow.Graph` API.

## When to use this skill

- Onboarding to the codebase for the first time.
- Adding a new pipeline stage.
- Tracing where data lives and who owns it.
- Refactoring across package boundaries.

## The 5-stage pipeline

```
Chunk → Extract → Standardize → Infer → Visualize
```

Each stage is a `FunctionNode` (see `skills/adk-workflow`) operating on a shared
`*domain.PipelineState` value.

| # | Stage | File | What it does |
|---|-------|------|--------------|
| 0 | `chunk` | `internal/chunker/chunker.go` | Splits input text into overlapping word-based chunks. |
| 1 | `extract` | `internal/extractor/extractor.go` | Sends each chunk to an OpenAI-compatible LLM and parses S-P-O triples from JSON. |
| 2 | `standardize` | `internal/standardizer/standardizer.go` | Normalizes entity names. Optionally uses LLM for resolution. |
| 3 | `infer` | `internal/inference/inference.go` | Discovers cross-community, within-community, transitive, and lexical relationships. |
| 4 | `visualize` | `internal/graph/graph.go` + `internal/viz/viz.go` | Builds the graph, runs community detection, renders interactive HTML via vis-network. |

## Package dependency map

```
cmd/kgraph/main.go
  └─► internal/pipeline
        ├─► internal/chunker
        ├─► internal/extractor
        │     └─► internal/prompts
        ├─► internal/standardizer
        │     └─► internal/extractor  (LimitPredicateLength only)
        ├─► internal/inference
        │     └─► internal/extractor  (LimitPredicateLength only)
        ├─► internal/graph
        │     └─► internal/domain
        └─► internal/viz
              ├─► internal/graph
              └─► internal/domain
```

- `internal/domain` is the leaf: it defines `Triple`, `GraphStats`, `PipelineState`.
  It has zero imports from other `internal/` packages.
- `internal/pipeline` is the only orchestrator. No other package imports it.
- `cmd/kgraph` is the only package that imports `internal/pipeline`.

## Data flow

1. CLI reads `--input` file → `string`.
2. `pipeline.New(cfg).Run(ctx, text, output, debug)` builds a workflow and
   `Execute`s it.
3. The workflow threads `*domain.PipelineState` through the stages:
   - `InputText` is set up front.
   - `Chunks` is filled by `chunkStep`.
   - `Triples` is filled by `extractStep`, mutated by `standardizeStep`, then
     appended to by `inferStep`.
   - `Stats` is filled by the visualize step's call to `graph.Build(...)`.
   - `OutputFile` and `Debug` are constants carried from the CLI.
4. The terminal `visualize` node writes `<output>.html` and a sibling
   `<output>.json` (triples only) to disk.

## Configuration

`config.toml` (loaded by `internal/config`) carries four sections:
- `[llm]` — model, base URL, API key, max tokens, temperature.
- `[chunking]` — `chunk_size`, `overlap` (words).
- `[standardization]` — `enabled`, `use_llm_for_entities`.
- `[inference]` — `enabled`, `use_llm_for_inference`, `apply_transitive`.
- `[visualization]` — `edge_smooth` (bool or string for vis-network `smooth.type`).

CLI flags override config (`--no-standardize`, `--no-inference`). The
`--test` mode bypasses the pipeline entirely and renders the sample triples
defined in `cmd/kgraph/main.go`.

## Where to look

- I want to change **what the LLM is asked to do** → `internal/prompts/prompts.go`.
- I want to change **how LLM JSON is parsed** → `internal/extractor/extractor.go`
  (`parseTriples`, `extractJSONArray`, `fixJSON`).
- I want to change **how the graph is built** → `internal/graph/graph.go`
  (`Build`, `detectCommunities`, `computeBetweenness`).
- I want to change **how the HTML looks** → `internal/viz/viz.go` (`generateHTML`).
- I want to change **which stages run and in what order** →
  `internal/pipeline/pipeline.go` (`buildWorkflow`).
