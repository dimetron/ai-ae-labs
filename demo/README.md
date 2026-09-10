# Collection of AI Agentic projects

Hands-on demos accompanying the course, from platform plumbing to full agentic systems.

| # | Project | Status | What it covers |
|---|---------|--------|----------------|
| 1 | [`1_ai-gateway`](1_ai-gateway/) | config-verified | agentgateway v1.4.1 + Jaeger/Prometheus/Grafana: one OpenAI-compatible entry on `:4000` routing to a keyless mock, OpenAI, Anthropic, Gemini and Ollama Cloud, with per-request USD cost |
| 2 | `2_ai-daily-news-mail` | planned | daily news/mail digest agent |
| 3 | [`3_ai-knowledge-graph-go`](3_ai-knowledge-graph-go/) | working | text → LLM triple extraction → standardize → infer → interactive HTML graph |
| 4 | `4_coding_agent` | planned | coding agent |
| 5 | `5_personal_agent` | planned | personal assistant agent |
| 6 | `6_agentic_teams` | planned | multi-agent team / orchestration / Kanban MCP | SDLC Dev Team
| 7 | [`7_adk-go-evals`](7_adk-go-evals/) | working | Agent evals у ADK Go без нативного API: 4 методи (scripted-model unit, golden trajectory log, live LLM-as-judge, evalset runner з results_*.json) + бібліотека adkeval (TrajScore/Rouge1F1/verdict-enum) |
| — | `adk-quickstart-sso` | working | Week 1 starter: ADK Go v2 agent + two typed tools, Google SSO via ADC (API-key fallback) |

See each subfolder's `README.md` for build and run instructions.

> `adk-quickstart-sso` is deliberately unnumbered: it is the Week 1 course
> starter students download and run, not a stage in the 1→6 progression above.
> Give it a number only if it earns a slot in that sequence.
