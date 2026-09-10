# AI Knowledge Graph — Go ADK 2.0

A Go reimplementation of [robert-mcdermott/ai-knowledge-graph](https://github.com/robert-mcdermott/ai-knowledge-graph) using an ADK v2-style workflow graph architecture.

## What it does

Takes unstructured text, sends it through an LLM pipeline to extract subject-predicate-object triples, standardizes entities, infers additional relationships, and produces an interactive HTML visualization using vis.js.

## Architecture

The pipeline is modeled as a **workflow graph** mirroring the ADK v2 `workflow.Graph` API:

```
Chunk → Extract → Standardize → Infer → Visualize
```

Each stage is a `FunctionNode` operating on shared `PipelineState`:

| Node | Type | Description |
|------|------|-------------|
| `chunk` | FunctionNode | Splits input text into overlapping word-based chunks |
| `extract` | FunctionNode + LLM | Sends each chunk to an OpenAI-compatible LLM for triple extraction |
| `standardize` | FunctionNode + LLM | Normalizes entity names, optionally using LLM for resolution |
| `infer` | FunctionNode + LLM | Discovers cross-community, within-community, transitive, and lexical relationships |
| `visualize` | FunctionNode | Builds graph structure, detects communities, renders interactive HTML |

### Project structure

```
cmd/kgraph/main.go             CLI entry point
internal/
  config/config.go              TOML configuration
  domain/triple.go              Core types (Triple, PipelineState)
  chunker/chunker.go            Text chunking with overlap
  extractor/extractor.go        LLM triple extraction + JSON parsing
  standardizer/standardizer.go  Entity name normalization
  inference/inference.go        Relationship inference (4 strategies)
  graph/graph.go                Graph building + community detection
  viz/viz.go                    HTML visualization generation
  prompts/prompts.go            LLM prompt templates
  pipeline/
    pipeline.go                 ADK-style workflow orchestration
    workflow.go                 Local workflow graph types (ADK v2 API mirror)
```

## Quick start

```bash
# Build
make build

# Generate sample visualization (no LLM needed)
make sample
open sample_graph.html

# Run with text input (requires an OpenAI-compatible LLM)
make run

# Run with debug output
make debug
```

## Configuration

Edit `config.toml`:

```toml
[llm]
model = "gemma3"
api_key = "sk-1234"
base_url = "http://localhost:11434/v1/chat/completions"
max_tokens = 8192
temperature = 0.8

[chunking]
chunk_size = 100
overlap = 20

[standardization]
enabled = true
use_llm_for_entities = true

[inference]
enabled = true
use_llm_for_inference = true
apply_transitive = true

[visualization]
edge_smooth = false
```

Works with any OpenAI-compatible API: Ollama, LiteLLM, vLLM, OpenAI, Anthropic (via proxy), etc.

## CLI flags

```
--input FILE      Input text file (required unless --test)
--output FILE     Output HTML file (default: knowledge_graph.html)
--config FILE     Config file path (default: config.toml)
--test            Generate sample visualization with built-in data
--debug           Print raw LLM responses and extracted JSON
--no-standardize  Disable entity standardization phase
--no-inference    Disable relationship inference phase
```

## ADK v2 migration

The `internal/pipeline/workflow.go` file provides a local implementation of the ADK v2 `workflow.Graph` API. When the real ADK Go module is available:

1. Add `google.golang.org/adk/v2` to `go.mod`
2. Delete `workflow.go`
3. In `pipeline.go`, replace the local types with ADK imports:
   ```go
   import "google.golang.org/adk/v2/workflow"
   ```
4. Update `buildWorkflow()` to use `workflow.NewGraph`, `workflow.NewFunctionNode`, etc.

## Credits

- Original Python version: [robert-mcdermott/ai-knowledge-graph](https://github.com/robert-mcdermott/ai-knowledge-graph)
- Visualization: [vis.js Network](https://visjs.github.io/vis-network/)
