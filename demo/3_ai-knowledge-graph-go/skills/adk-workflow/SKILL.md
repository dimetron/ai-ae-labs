# ADK v2 Workflow (local implementation)

The pipeline is a typed sequential workflow graph. The types live in
`internal/pipeline/workflow.go` and intentionally mirror the Google ADK v2
`workflow.Graph` API so the local types can be swapped for the real module
later (see README "ADK v2 migration").

## When to use this skill

- Adding, removing, or reordering pipeline stages.
- Writing a new `FunctionNode` step.
- Replacing the local workflow with `google.golang.org/adk/v2/workflow`.

## Core types

```go
// Graph is a typed sequential workflow. S is the state type threaded through nodes.
type Graph[S any] struct { /* unexported */ }

func NewGraph[S any](name string) *Graph[S]
func (g *Graph[S]) AddNode(node *FunctionNode[S])
func (g *Graph[S]) AddEdge(from, to string)
func (g *Graph[S]) SetEntrypoint(name string)
func (g *Graph[S]) Execute(ctx context.Context, state S) error

type FunctionNode[S any] struct { /* unexported */ }

func NewFunctionNode[S any](name string, fn func(context.Context, S) (S, error)) *FunctionNode[S]
```

A `FunctionNode` wraps a pure-ish function `(ctx, S) → (S, error)`. State is
passed by value through the graph (the current implementation uses a
`*domain.PipelineState` pointer, so mutations stick — match that pattern).

## Adding a new stage

In `internal/pipeline/pipeline.go`, three edits are required:

1. **Define the step method** on `*Pipeline`:
   ```go
   func (p *Pipeline) myStep(ctx context.Context, state *domain.PipelineState) (*domain.PipelineState, error) {
       // ... mutate state, log via slog, return state, nil ...
   }
   ```
   Match the existing step signatures: take `ctx`, return the same state type,
   and an error. Use `slog.Info` for phase markers, `slog.Warn` for recoverable
   issues, `fmt.Println` only for the human-facing phase banners (existing
   pattern).

2. **Register the node in `buildWorkflow`**:
   ```go
   myNode := NewFunctionNode("myname", p.myStep)
   wf.AddNode(myNode)
   ```

3. **Wire edges** so the node sits in the chain:
   ```go
   wf.AddEdge("prev", "myname")
   wf.AddEdge("myname", "next")
   ```
   The graph is sequential — only the **first** outgoing edge is followed
   (see `Execute` in `workflow.go`). Don't add branches; if you need a branch,
   design two graphs.

## Execution semantics

`Execute` walks the graph from the entrypoint, following `g.edges[current][0]`.
A node with no outgoing edges is terminal — the loop ends. Any error from a
node's function is wrapped with workflow and node context and returned.

There is no parallelism, no back-edges, no conditional routing. The current
design is intentionally minimal. The ADK v2 real module supports more, but
this local mirror is enough for the linear pipeline.

## Replacing with the real ADK

When `google.golang.org/adk/v2/workflow` is available:

1. Add the dependency to `go.mod`.
2. Delete `internal/pipeline/workflow.go`.
3. In `pipeline.go`, add `import "google.golang.org/adk/v2/workflow"`.
4. Update `buildWorkflow` to call `workflow.NewGraph[*domain.PipelineState]`,
   `workflow.NewFunctionNode(...)`, etc. The local constructors are
   drop-in-compatible by design.

The `FunctionNode` function signature
`func(context.Context, S) (S, error)` matches the ADK v2 surface, so the
step methods (`p.chunkStep`, `p.extractStep`, ...) do not need to change.

## Anti-patterns

- **Don't** add fields to `Graph[S]` — it's a structural mirror of ADK v2.
  If you need extra behavior, add a method on `Pipeline` and call it from
  the step function instead.
- **Don't** use `g.edges[current][1:]` — only the first edge is followed.
  Sequential is the only supported topology here.
- **Don't** add a step that returns a *different* state type. The graph is
  generic over one state type for the whole workflow.
