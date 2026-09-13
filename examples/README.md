# ADK 2.0 Go Agents — AI Agents Engineering Course

Код лабораторних робіт курсу **«AI Agents Engineering»** для Prometheus (prometheus.org.ua).  
Кожна частина тижня має власного ADK 2.0 Go-агента, що демонструє ключові концепції лекції.

## Структура

```
code/
├── go.mod                          # Module: google.golang.org/adk/v2 v2.4.0
├── go.sum
├── internal/adkrun/
│   ├── adkrun.go                   # Shared headless runner (Pipeline + Run)
│   └── testctx.go                  # TestContext for DynamicNode tests
├── week1/part1/cmd/                # ModelBenchmarkAgent
├── week1/part2/cmd/                # StrictSchemaEnforcer
├── week2/part1/cmd/                # WorkflowGraphAgent
├── week2/part2/cmd/                # AgentService (HTTP)
├── week3/part1/cmd/                # OntologyAwareKnowledgeBuilder
├── week3/part2/cmd/                # AgenticGraphRAGEngine
├── week4/part1/cmd/                # NativeReActAgent
├── week4/part2/cmd/                # FaultTolerantToolHarness
├── week5/part1/cmd/                # EventSourcedMemoryAgent
├── week5/part2/cmd/                # CollaborativeWorkflowEngine
├── week6/part1/cmd/                # MultiCriticCodingAgent
└── week6/part2/cmd/                # PersonalResearchAgent
```

## Agents Overview

| Week | Part | Agent | Topic | ADK Pattern | Key Concepts |
|------|------|-------|-------|-------------|--------------|
| 1 | 1 | **ModelBenchmarkAgent** | Models & Frameworks Landscape | `FunctionNode` + `Chain` | Cross-model benchmark: latency, cost, quality; decision framework axes |
| 1 | 2 | **StrictSchemaEnforcer** | Structured Output & Function Calling | `FunctionNode` + typed structs | JSON Schema validation, typed Go contracts, fail path |
| 2 | 1 | **WorkflowGraphAgent** | First ADK2 Agent Workflow Graph | `FunctionNode` + `Chain` | Explicit workflow graph, typed nodes, event log |
| 2 | 2 | **AgentService** | Agent as Service Deploy | `FunctionNode` + HTTP wrapper | Health/readiness, streaming, graceful shutdown, `FROM scratch` |
| 3 | 1 | **OntologyAwareKnowledgeBuilder** | Lossless PDF Parsing & Chunking | `FunctionNode` + `Chain` | Ontology-driven chunking, entity resolution, provenance |
| 3 | 2 | **AgenticGraphRAGEngine** | Reranking & Semantic Cache | `EdgeBuilder` + `JoinNode` | Hybrid retrieval, query planner, cross-encoder rerank, semantic cache |
| 4 | 1 | **NativeReActAgent** | ReAct Loop Internals | `DynamicNode` | Manual ReAct cycle, `maxIterations` guard, tool selection |
| 4 | 2 | **FaultTolerantToolHarness** | Fault Tolerant Tool Harness | `FunctionNode` + error classification | `ErrTransient` vs `ErrSemantic`, retry with backoff, slog logging |
| 5 | 1 | **EventSourcedMemoryAgent** | Agent Memory & Event-Sourced State | `FunctionNode` + `Chain` | Two-layer memory, `StateDelta`, concurrent-safe store |
| 5 | 2 | **CollaborativeWorkflowEngine** | Collaboration Agents & HITL | `EdgeBuilder` + `JoinNode` + `EmittingFunctionNode` | Plan-Execute, fan-out/join, HITL pause, coordinator pattern |
| 6 | 1 | **MultiCriticCodingAgent** | Dynamic Workflows & Multi-Critic | `DynamicNode` | Maker/critic cycle, correctness/style/performance critics, `maxAttempts` |
| 6 | 2 | **PersonalResearchAgent** | Self-Improving Agents | `DynamicNode` | Autoresearch pattern: bounded surface, fixed budget, single metric, audit log |

## ADK 2.0 Patterns Used

| Pattern | Agents | Description |
|---------|--------|-------------|
| `workflow.Chain` | W1P1, W1P2, W2P1, W3P1, W4P2, W5P1 | Linear pipeline of typed nodes |
| `workflow.NewFunctionNode` | All 12 | Typed function node with auto-inferred JSON Schema |
| `workflow.NewDynamicNode` | W4P1, W6P1, W6P2 | Imperative Go inside declarative shell |
| `workflow.NewJoinNode` | W3P2, W5P2 | Fan-in barrier for parallel results |
| `workflow.NewEmittingFunctionNode` | W5P2 | Streaming node with HITL pause support |
| `workflow.EdgeBuilder` | W3P2, W5P2 | Fluent API for fan-out/fan-in edges |
| `workflow.NodeConfig` + `DefaultRetryConfig` | All 12 | Per-node retry policy |
| `session.InMemoryService` | All (via `adkrun.Run`) | In-memory session storage |
| `agent.StrictContextMock` | All tests | Strict test double for agent.Context |

## Running

```bash
# Build all agents
go build ./...

# Run all tests
go test ./...

# Run a specific agent
go run ./week1/part1/cmd --query "compare top models"
go run ./week1/part2/cmd --txn txn-2026-07-118845 --merchant A-114 --reason customer_request --amount-minor 145000
go run ./week2/part1/cmd --input "py-pro-2025-09,student@example.com,Andrii"
go run ./week2/part2/cmd --addr :8080
go run ./week3/part1/cmd --input "# Q3 Report\nAcme Corp signed."
go run ./week3/part2/cmd --query "who signed contract with Vendor X"
go run ./week4/part1/cmd --query "investigate billing service timeout"
go run ./week4/part2/cmd --input "test enrollment"
go run ./week5/part1/cmd --action remember --key user:alice:topic --value "AI for Lawyers"
go run ./week5/part2/cmd --query "prepare board presentation"
go run ./week6/part1/cmd --spec "write a function that processes data"
go run ./week6/part2/cmd --task "optimize data processing function"
```

## Dependencies

- `google.golang.org/adk/v2` v2.4.0 — Google ADK Go SDK
- `google.golang.org/genai` — GenAI content types (indirect)

Станом на 07/2026. Перед записом лекцій звірити версії з поточним релізом.
