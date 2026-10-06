package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.temporal.io/sdk/client"
	"google.golang.org/adk/v2/model"

	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/graphmemory"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/graphmemory/graphmemorytest"

	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/agentdb"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/ledger"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/llm"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestConfigFromEnv(t *testing.T) {
	t.Parallel()
	c, err := ConfigFromEnv(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.TemporalAddress != "localhost:7233" || c.AgentDB != ".data/agent.db" || c.Model != "live" || !c.Sloppy || c.Memory.Backend != "dgraph" {
		t.Errorf("defaults = %+v", c)
	}
	c, err = ConfigFromEnv(env(map[string]string{
		"TEMPORAL_ADDRESS": "t:1", "AGENT_DB": "x.db", "LABS8_MODEL": "live", "LABS8_DEMO_SLOPPY": "0",
		"LEDGER_FAULTS": "503, timeout,429", "LEDGER_LATENCY": "15s", "MEMORY_BACKEND": "dgraph",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.TemporalAddress != "t:1" || c.Sloppy || c.Latency != 15*time.Second ||
		!slices.Equal(c.Faults, []ledger.Fault{ledger.Unavailable, ledger.Timeout, ledger.RateLimited}) {
		t.Errorf("explicit = %+v", c)
	}
	for _, bad := range []map[string]string{
		{"LABS8_MODEL": "gpt"},
		{"LEDGER_FAULTS": "meteor"},
		{"LEDGER_LATENCY": "soon"},
	} {
		if _, err := ConfigFromEnv(env(bad)); err == nil {
			t.Errorf("%v: want error", bad)
		}
	}
}

func TestModelFactory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := agentdb.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	never := func(context.Context) (model.LLM, error) { t.Fatal("live model built in demo mode"); return nil, nil }
	m, err := ModelFactory(Config{Model: "demo", Sloppy: true}, db, never)(ctx, ModelName)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := m.(llm.Counting)
	if !ok || c.Inner != (llm.Demo{Sloppy: true}) {
		t.Fatalf("demo model = %#v", m)
	}

	live := llm.Demo{} // any model will do as the "live" one
	m, err = ModelFactory(Config{Model: "live"}, db, func(context.Context) (model.LLM, error) { return live, nil })(ctx, ModelName)
	if err != nil || m.(llm.Counting).Inner != live {
		t.Fatalf("live model = %#v, %v", m, err)
	}
	if _, err := liveModel(ctx); err != nil && !strings.Contains(err.Error(), "LABS8_MODEL=demo") {
		t.Errorf("live model error must say how to run offline: %v", err)
	}
	boom := errors.New("no key")
	if _, err := ModelFactory(Config{Model: "live"}, db, func(context.Context) (model.LLM, error) { return nil, boom })(ctx, ModelName); !errors.Is(err, boom) {
		t.Errorf("live error = %v", err)
	}
}

func TestRunCommands(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "agent.db")
	base := map[string]string{"AGENT_DB": dbPath, "TEMPORAL_ADDRESS": "127.0.0.1:1", "DGRAPH_ADDR": "127.0.0.1:1", "LABS8_MODEL": "demo"}
	with := func(k, v string) func(string) string {
		m := map[string]string{k: v}
		for kk, vv := range base {
			if _, ok := m[kk]; !ok {
				m[kk] = vv
			}
		}
		return env(m)
	}
	var out bytes.Buffer

	if err := run(ctx, nil, env(base), &out); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Errorf("no args: %v", err)
	}
	if err := run(ctx, []string{"dance"}, env(base), &out); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("unknown: %v", err)
	}
	if err := run(ctx, []string{"counter"}, with("LABS8_MODEL", "nope"), &out); err == nil {
		t.Error("bad config must fail before any command runs")
	}

	out.Reset()
	if err := run(ctx, []string{"counter"}, env(base), &out); err != nil || !strings.Contains(out.String(), "llm_calls=0 refund_cases=0") {
		t.Errorf("counter = %q, %v", out.String(), err)
	}
	if err := run(ctx, []string{"counter"}, with("AGENT_DB", "/dev/null/x.db"), &out); err == nil {
		t.Error("counter on an unopenable file must fail")
	}

	if err := run(ctx, []string{"start"}, env(base), &out); err == nil || !strings.Contains(err.Error(), "MESSAGE") {
		t.Errorf("start without message: %v", err)
	}
	if err := run(ctx, []string{"start", "-nope"}, env(base), &out); err == nil {
		t.Error("bad flag must fail")
	}
	if err := run(ctx, []string{"start", "hello"}, env(base), &out); err == nil || !strings.Contains(err.Error(), "temporal") {
		t.Errorf("start without server: %v", err)
	}

	// The worker checks its dependencies in order and names the missing one.
	if err := run(ctx, []string{"worker"}, with("AGENT_DB", "/dev/null/x.db"), &out); err == nil {
		t.Error("worker with unopenable agent.db must fail")
	}
	if err := run(ctx, []string{"worker"}, env(base), &out); err == nil || !strings.Contains(err.Error(), "DGRAPH_ADDR") {
		t.Errorf("worker without dgraph: %v", err)
	}
}

// fakeStarter and fakeRun are function-based doubles for the Temporal client.
type fakeStarter struct {
	opts client.StartWorkflowOptions
	req  Request
	err  error
	run  fakeRun
}

func (f *fakeStarter) ExecuteWorkflow(_ context.Context, opts client.StartWorkflowOptions, _ any, args ...any) (client.WorkflowRun, error) {
	f.opts, f.req = opts, args[0].(Request)
	if f.err != nil {
		return nil, f.err
	}
	return f.run, nil
}

type fakeRun struct {
	reply Reply
	err   error
}

func (fakeRun) GetID() string                  { return "labs8-taras-s1" }
func (fakeRun) GetRunID() string               { return "run-1" }
func (fakeRun) GetFirstExecutionRunID() string { return "run-1" }
func (r fakeRun) Get(_ context.Context, v any) error {
	*(v.(*Reply)) = r.reply
	return r.err
}
func (r fakeRun) GetWithOptions(ctx context.Context, v any, _ client.WorkflowRunGetOptions) error {
	return r.Get(ctx, v)
}

func TestStartTurn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	req := Request{UserID: "taras", SessionID: "s1", Message: "hi"}

	var out bytes.Buffer
	ok := &fakeStarter{run: fakeRun{reply: Reply{Log: []string{"[ev:answer] hello", "[ev:success]"}}}}
	if err := startTurn(ctx, ok, req, &out); err != nil {
		t.Fatal(err)
	}
	if ok.opts.ID != "labs8-taras-s1" || ok.opts.TaskQueue != TaskQueue || ok.req != req {
		t.Errorf("started %+v with %+v", ok.opts, ok.req)
	}
	if want := "workflow labs8-taras-s1 run run-1\n[ev:answer] hello\n[ev:success]\n"; out.String() != want {
		t.Errorf("out = %q", out.String())
	}

	out.Reset()
	failed := &fakeStarter{run: fakeRun{reply: Reply{Log: []string{"[ev:failed] limit"}}, err: errors.New("limit")}}
	if err := startTurn(ctx, failed, req, &out); err == nil || !strings.Contains(out.String(), "[ev:failed] limit") {
		t.Errorf("a failed turn must still print its log: %q, %v", out.String(), err)
	}
	if err := startTurn(ctx, &fakeStarter{err: errors.New("already running")}, req, &out); err == nil {
		t.Error("start error swallowed")
	}
}

func TestBuildWorkerRegistersEverything(t *testing.T) {
	t.Parallel()
	db, err := agentdb.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// A lazy client never dials, so the worker can be built without a server.
	cl, err := client.NewLazyClient(client.Options{HostPort: "127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cl.Close)
	c, _ := ConfigFromEnv(env(map[string]string{"LEDGER_FAULTS": "503"}))
	w, err := buildWorker(cl, c, db, graphmemory.New(&graphmemorytest.Fake{}), nil)
	if err != nil || w == nil {
		t.Fatalf("buildWorker = %v, %v", w, err)
	}
}
