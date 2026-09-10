# Build, Run, Verify

How to compile, run, and verify the project. Use this skill any time you
change code and need to confirm it still works.

## When to use this skill

- After editing any `.go` file.
- Before declaring a task complete.
- When the user asks how to run the binary, regenerate the sample, or
  point at a different config.

## Toolchain

- Go ≥ the version in `go.mod` (currently `go 1.24`). Confirm with
  `go version`.
- No third-party tools required for build/test (only `github.com/BurntSushi/toml`).
- Optional: `golangci-lint` for `make lint`.

## Make targets (`Makefile`)

```bash
make build      # go build -o kgraph ./cmd/kgraph
make run        # build + run with data/industrial-revolution.txt
make sample     # build + run --test (no LLM needed) → sample_graph.html
make debug      # build + run with --debug (raw LLM output)
make test       # go test ./...
make lint       # golangci-lint run ./...   (optional, tool not required)
make clean      # rm kgraph *.html *.json
```

`make sample` is the fastest verification loop — it bypasses the LLM and
exercises `graph.Build`, `viz.Render`, and the JSON writer. Use it after
non-LLM changes.

`make run` requires a live LLM at `cfg.BaseURL`. If the user has Ollama
running locally, it works out of the box; otherwise the call will fail
at the extract step and the error is logged via `slog.Warn`.

## CLI flags

| Flag | Default | Purpose |
|------|---------|---------|
| `--input FILE` | (required unless `--test`) | Input text file. |
| `--output FILE` | `knowledge_graph.html` | Output HTML file. |
| `--config FILE` | `config.toml` | TOML config path. |
| `--test` | off | Render the sample data defined in `cmd/kgraph/main.go`. |
| `--debug` | off | Enable `slog.LevelDebug` and print per-chunk raw LLM JSON. |
| `--no-standardize` | off | Disable the standardization stage. |
| `--no-inference` | off | Disable the inference stage. |

`--no-standardize` and `--no-inference` mutate the loaded config in
memory; the on-disk `config.toml` is untouched.

## Direct Go invocations (no Make)

```bash
go build -o kgraph ./cmd/kgraph
./kgraph --test --output sample_graph.html
./kgraph --input data/industrial-revolution.txt --output out.html
go test ./...
go vet ./...
```

## Verification checklist

After a non-trivial change, run **all** of the following from the project
root and confirm each is clean before reporting done:

1. `go build ./...` — must compile.
2. `go vet ./...` — must be silent.
3. `go test ./...` — exits 0 (currently `[no test files]` for every package).
4. `./kgraph --test --output sample_graph.html` — must produce a
   non-empty HTML file. `head -c 200 sample_graph.html` should show the
   `<!DOCTYPE html>` and `<title>` lines.
5. `open sample_graph.html` (or `file://...`) — visual check, only if
   the change touched `viz/`, `graph/`, or `pipeline/visualizeStep`.

For changes that touched only the LLM client or prompts, skip step 5
and run `./kgraph --debug --input data/industrial-revolution.txt` once
to confirm the request and response cycle still works end-to-end.

## Sample data

- `data/industrial-revolution.txt` — short Wikipedia-style paragraph used
  by `make run` and `make debug`. Good for quick smoke tests.
- `cmd/kgraph/main.go` → `sampleTriples()` — hand-crafted triples used by
  `--test`. Update both this and the corresponding Python original if
  adding sample data.

## Troubleshooting

- **"Failed to load config from config.toml"** — confirm you're in the
  project root. The CLI resolves `--config` relative to CWD, not the
  binary location.
- **"API error (status 404)"** — `cfg.BaseURL` is wrong. It should be
  the full chat-completions URL, e.g.
  `http://localhost:11434/v1/chat/completions`.
- **Empty triples, no errors** — usually the LLM returned text outside
  of a JSON array. Run with `--debug` to see the raw output.
- **`go test` shows failures** — there are no tests yet; any failure
  indicates a build problem. Run `go build ./...` first.
