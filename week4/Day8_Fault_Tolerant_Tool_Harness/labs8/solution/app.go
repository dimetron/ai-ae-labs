package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/worker"
	"google.golang.org/adk/v2/model"

	"github.com/dimetron/ai-eng-course/labs/internal/modelcfg"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/agentdb"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/graphmemory"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/graphmemory/backend"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/ledger"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/llm"
)

// TaskQueue is where the worker listens.
const TaskQueue = "labs8"

const usage = `usage:
  solution worker                                   run the durable agent worker
  solution start [-user U] [-session S] MESSAGE     run one turn and print its event log
  solution counter                                  print LLM calls and refund cases from agent.db

env: TEMPORAL_ADDRESS (localhost:7233)  AGENT_DB (.data/agent.db)
     MEMORY_BACKEND (dgraph)  DGRAPH_ADDR (localhost:9080)
     LABS8_MODEL (live|demo, live)  LABS8_DEMO_SLOPPY (1|0, 1; demo only)
     live = the course provider from apps/.env (internal/modelcfg); demo = offline rules
     LEDGER_FAULTS (e.g. 503,503,timeout,429)  LEDGER_LATENCY (e.g. 15s)`

// Config is everything the commands read from the environment.
type Config struct {
	TemporalAddress string
	AgentDB         string
	Model           string // demo | live
	Sloppy          bool
	Faults          []ledger.Fault
	Latency         time.Duration
	Memory          backend.Config
}

// ConfigFromEnv parses the environment through getenv.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	c := Config{
		TemporalAddress: or(getenv("TEMPORAL_ADDRESS"), client.DefaultHostPort),
		AgentDB:         or(getenv("AGENT_DB"), agentdb.DefaultPath),
		Model:           or(getenv("LABS8_MODEL"), "live"),
		Sloppy:          getenv("LABS8_DEMO_SLOPPY") != "0",
		Memory:          backend.ConfigFromEnv(getenv),
	}
	if c.Model != "demo" && c.Model != "live" {
		return c, fmt.Errorf("LABS8_MODEL=%q: want demo or live", c.Model)
	}
	faults, err := ParseFaults(getenv("LEDGER_FAULTS"))
	if err != nil {
		return c, err
	}
	c.Faults = faults
	if v := getenv("LEDGER_LATENCY"); v != "" {
		if c.Latency, err = time.ParseDuration(v); err != nil {
			return c, fmt.Errorf("LEDGER_LATENCY=%q: %w", v, err)
		}
	}
	return c, nil
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// ParseFaults reads a comma-separated fault list: 503, timeout, 429.
func ParseFaults(s string) ([]ledger.Fault, error) {
	var out []ledger.Fault
	for f := range strings.SplitSeq(s, ",") {
		switch strings.TrimSpace(strings.ToLower(f)) {
		case "":
		case "503", "unavailable":
			out = append(out, ledger.Unavailable)
		case "timeout":
			out = append(out, ledger.Timeout)
		case "429", "ratelimit":
			out = append(out, ledger.RateLimited)
		default:
			return nil, fmt.Errorf("LEDGER_FAULTS: unknown fault %q (want 503, timeout, 429)", f)
		}
	}
	return out, nil
}

// ModelFactory builds the worker-side model for googleadk: the demo rules or
// the course's provider choice, always behind the durable call counter.
func ModelFactory(c Config, rec llm.Recorder, live func(context.Context) (model.LLM, error)) googleadk.ModelFactory {
	return func(ctx context.Context, _ string) (model.LLM, error) {
		var inner model.LLM = llm.Demo{Sloppy: c.Sloppy}
		if c.Model == "live" {
			m, err := live(ctx)
			if err != nil {
				return nil, err
			}
			inner = m
		}
		return llm.Counting{Inner: inner, Recorder: rec}, nil
	}
}

// liveModel is the course's provider choice (apps/.env, MODEL,
// DEFAULT_MODEL_PROVIDER). Without a provider the error says how to run
// offline instead.
func liveModel(ctx context.Context) (model.LLM, error) {
	m, _, err := modelcfg.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("live model: %w (configure a provider in apps/.env, or run offline with LABS8_MODEL=demo)", err)
	}
	return m, nil
}

// run dispatches a subcommand.
func run(ctx context.Context, args []string, getenv func(string) string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	if err := modelcfg.LoadEnv("."); err != nil {
		fmt.Fprintf(out, "warning: %v\n", err)
	}
	c, err := ConfigFromEnv(getenv)
	if err != nil {
		return err
	}
	switch args[0] {
	case "worker":
		return runWorker(ctx, c, out)
	case "start":
		return runStart(ctx, c, args[1:], out)
	case "counter":
		return runCounter(ctx, c, out)
	}
	return fmt.Errorf("unknown command %q\n%s", args[0], usage)
}

func runCounter(ctx context.Context, c Config, out io.Writer) error {
	db, err := agentdb.Open(c.AgentDB)
	if err != nil {
		return err
	}
	defer db.Close()
	calls, err := db.CountLLMCalls(ctx)
	if err != nil {
		return err
	}
	cases, err := db.CountCases(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "llm_calls=%d refund_cases=%d\n", calls, cases)
	return nil
}

func runStart(ctx context.Context, c Config, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(out)
	user := fs.String("user", "taras", "user id")
	sess := fs.String("session", "s1", "session id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	msg := strings.Join(fs.Args(), " ")
	if msg == "" {
		return errors.New("start: MESSAGE is required")
	}
	cl, err := client.Dial(client.Options{HostPort: c.TemporalAddress})
	if err != nil {
		return fmt.Errorf("temporal at %s (is `temporal server start-dev` running?): %w", c.TemporalAddress, err)
	}
	defer cl.Close()
	return startTurn(ctx, cl, Request{UserID: *user, SessionID: *sess, Message: msg}, out)
}

// starter is the one client method startTurn needs.
type starter interface {
	ExecuteWorkflow(ctx context.Context, opts client.StartWorkflowOptions, workflow any, args ...any) (client.WorkflowRun, error)
}

// startTurn runs one turn and prints its event log, also when it failed.
//
// The workflow ID is the session: two turns of one conversation must not race
// on the stored session, so a second start while one runs is rejected.
func startTurn(ctx context.Context, s starter, req Request, out io.Writer) error {
	run, err := s.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "labs8-" + req.UserID + "-" + req.SessionID,
		TaskQueue: TaskQueue,
	}, AgentWorkflow, req)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "workflow %s run %s\n", run.GetID(), run.GetRunID())
	var reply Reply
	err = run.Get(ctx, &reply)
	for _, l := range reply.Log {
		fmt.Fprintln(out, l)
	}
	return err
}

func runWorker(ctx context.Context, c Config, out io.Writer) error {
	db, err := agentdb.Open(c.AgentDB)
	if err != nil {
		return err
	}
	defer db.Close()
	mem, closer, err := backend.Open(ctx, c.Memory)
	if err != nil {
		return err
	}
	defer closer.Close()
	cl, err := client.Dial(client.Options{HostPort: c.TemporalAddress})
	if err != nil {
		return fmt.Errorf("temporal at %s (is `temporal server start-dev` running?): %w", c.TemporalAddress, err)
	}
	defer cl.Close()
	// Build the model once up front: a missing key should stop the worker now,
	// not fail the first turn after it has already called tools.
	m, err := ModelFactory(c, db, liveModel)(ctx, ModelName)
	if err != nil {
		return err
	}
	w, err := buildWorker(cl, c, db, mem, liveModel)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "worker: task queue %s, model %s (%s), memory %s, agent.db %s\n", TaskQueue, c.Model, m.Name(), c.Memory.Backend, c.AgentDB)
	return w.Run(worker.InterruptCh())
}

// buildWorker registers the workflow, the Activities and the googleadk plugin
// (which registers the model Activity) on a worker for cl.
func buildWorker(cl client.Client, c Config, db *agentdb.DB, mem *graphmemory.Service, live func(context.Context) (model.LLM, error)) (worker.Worker, error) {
	faults := &ledger.FaultPlan{}
	faults.Push(c.Faults...)
	plugin, err := googleadk.NewPlugin(googleadk.Config{
		Models: map[string]googleadk.ModelFactory{ModelName: ModelFactory(c, db, live)},
	})
	if err != nil {
		return nil, err
	}
	w := worker.New(cl, TaskQueue, worker.Options{Plugins: []worker.Plugin{plugin}})
	w.RegisterWorkflow(AgentWorkflow)
	w.RegisterActivity(&Activities{
		Desk:     &ledger.Desk{Cases: db.Cases(), Faults: faults, Latency: c.Latency},
		Sessions: db.Sessions(),
		Memory:   mem,
	})
	return w, nil
}
