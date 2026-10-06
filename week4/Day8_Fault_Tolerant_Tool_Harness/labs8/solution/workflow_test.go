package main

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/nexus-rpc/sdk-go/nexus"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/converter"
	tlog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"

	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/agentdb"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/graphmemory"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/graphmemory/graphmemorytest"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/ledger"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/llm"
)

const refundRequest = "Open a refund for txn-2026-07-118845 at merchant A-114"

// world is the durable state that outlives one workflow run: the SQLite file
// and the memory store. Each run gets a fresh test environment over it, the
// way each turn gets a fresh workflow in production.
type world struct {
	db     *agentdb.DB
	mem    *graphmemorytest.Fake
	faults *ledger.FaultPlan
	runs   atomic.Int64
}

func newWorld(t *testing.T) *world {
	t.Helper()
	db, err := agentdb.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &world{db: db, mem: &graphmemorytest.Fake{}, faults: &ledger.FaultPlan{}}
}

// registry adapts the test environment to worker.Registry for googleadk.
type registry struct {
	*testsuite.TestWorkflowEnvironment
}

func (registry) RegisterNexusService(*nexus.Service) {}

type counter struct {
	mu sync.Mutex
	n  map[string]int
}

func (c *counter) get(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n[name]
}

// runTurn executes one AgentWorkflow over w with the given model.
func (w *world) runTurn(t *testing.T, m model.LLM, req Request) (Reply, error, *counter) {
	t.Helper()
	var s testsuite.WorkflowTestSuite
	s.SetLogger(tlog.NewStructuredLogger(slog.New(slog.DiscardHandler)))
	env := s.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(AgentWorkflow)
	env.RegisterActivity(&Activities{
		Desk:     &ledger.Desk{Cases: w.db.Cases(), Faults: w.faults},
		Sessions: w.db.Sessions(),
		Memory:   graphmemory.New(w.mem),
	})
	acts, err := googleadk.NewActivities(googleadk.Config{Models: map[string]googleadk.ModelFactory{
		ModelName: func(context.Context, string) (model.LLM, error) {
			return llm.Counting{Inner: m, Recorder: w.db}, nil
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	acts.Register(registry{env})
	c := &counter{n: map[string]int{}}
	env.SetOnActivityStartedListener(func(info *activity.Info, _ context.Context, _ converter.EncodedValues) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.n[info.ActivityType.Name]++
	})

	// A distinct workflow ID per run, as production has: event IDs derive from it.
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{ID: fmt.Sprintf("labs8-%s-%s-%d", req.UserID, req.SessionID, w.runs.Add(1))})
	env.ExecuteWorkflow(AgentWorkflow, req)
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	var reply Reply
	if werr := env.GetWorkflowError(); werr != nil {
		return reply, werr, c
	}
	if err := env.GetWorkflowResult(&reply); err != nil {
		t.Fatal(err)
	}
	return reply, nil, c
}

func hasLine(log []string, prefix string) bool {
	return slices.ContainsFunc(log, func(l string) bool { return strings.HasPrefix(l, prefix) })
}

func llmCalls(t *testing.T, w *world) int64 {
	t.Helper()
	n, err := w.db.CountLLMCalls(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func cases(t *testing.T, w *world) int64 {
	t.Helper()
	n, err := w.db.CountCases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestTransientFailuresAreRetriedByTheEngine(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.faults.Push(ledger.Unavailable, ledger.Unavailable)

	reply, err, c := w.runTurn(t, llm.Demo{}, Request{UserID: "taras", SessionID: "s1", Message: refundRequest})
	if err != nil {
		t.Fatal(err)
	}
	if got := c.get("OpenRefundCase"); got != 3 {
		t.Errorf("OpenRefundCase attempts = %d, want 3 (two 503s retried by Temporal)", got)
	}
	if hasLine(reply.Log, "[ev:tool_observation ERROR]") {
		t.Errorf("the model must not see a transient failure:\n%s", strings.Join(reply.Log, "\n"))
	}
	if !strings.Contains(reply.Answer, "rc-txn-2026-07-118845-A-114 is open") || reply.SelfCorrections != 0 {
		t.Errorf("reply = %+v", reply)
	}
	if n := cases(t, w); n != 1 {
		t.Errorf("refund cases = %d, want 1 (retries must not duplicate)", n)
	}
	if n := llmCalls(t, w); n != 2 {
		t.Errorf("LLM calls = %d, want 2 (one tool call, one answer)", n)
	}
}

func TestSemanticErrorBecomesObservationAndSelfCorrects(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	reply, err, c := w.runTurn(t, llm.Demo{Sloppy: true}, Request{UserID: "taras", SessionID: "s1", Message: refundRequest})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`[ev:tool_call] open_refund_case {"merchant_id":"undefined"`,
		`[ev:tool_observation ERROR] open_refund_case {"attempt":1,"field":"merchant_id"`,
		`[ev:reasoning] I should re-extract merchant_id`,
		`[ev:tool_call] open_refund_case {"merchant_id":"A-114"`,
		`[ev:tool_observation] open_refund_case {"case_id":"rc-txn-2026-07-118845-A-114"`,
		`[ev:answer] Refund case`,
		`[ev:success]`,
	}
	if len(reply.Log) != len(want) {
		t.Fatalf("log:\n%s", strings.Join(reply.Log, "\n"))
	}
	for i, p := range want {
		if !strings.HasPrefix(reply.Log[i], p) {
			t.Errorf("log[%d] = %q, want prefix %q", i, reply.Log[i], p)
		}
	}
	if got := c.get("OpenRefundCase"); got != 2 {
		t.Errorf("OpenRefundCase attempts = %d, want 2 (a semantic error is never retried by the engine)", got)
	}
	if reply.SelfCorrections != 1 || cases(t, w) != 1 {
		t.Errorf("self-corrections = %d, cases = %d", reply.SelfCorrections, cases(t, w))
	}
}

func TestSelfCorrectionLimitFailsExplicitly(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	bad := map[string]any{"transaction_id": "txn-2026-07-118845", "merchant_id": "undefined"}
	var script []*model.LLMResponse
	for i := range maxSelfCorrections + 2 {
		script = append(script, googleadk.FunctionCallResponse("c"+string(rune('1'+i)), "open_refund_case", bad))
	}
	script = append(script, googleadk.TextResponse("giving up"))

	reply, err, c := w.runTurn(t, googleadk.NewFakeModel(script...), Request{UserID: "taras", SessionID: "s1", Message: refundRequest})
	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) || appErr.Type() != ErrTypeSelfCorrection {
		t.Fatalf("err = %v, want ApplicationError %s", err, ErrTypeSelfCorrection)
	}
	if got := c.get("OpenRefundCase"); got != maxSelfCorrections+1 {
		t.Errorf("OpenRefundCase attempts = %d, want %d (the call after the limit is refused in-workflow)", got, maxSelfCorrections+1)
	}
	// A failed workflow returns no result; the audit trail is the persisted
	// session, which the harness writes before failing the run.
	if c.get("PersistSession") != 1 {
		t.Fatal("a failed turn must still persist its session — it is the audit log")
	}
	stored, err := w.db.Sessions().Get(context.Background(), &session.GetRequest{AppName: AppName, UserID: "taras", SessionID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	var stops int
	for ev := range stored.Session.Events().All() {
		for _, p := range ev.Content.Parts {
			if p.FunctionResponse != nil && p.FunctionResponse.Response["status"] == "stopped" {
				stops++
			}
		}
	}
	if stops != 2 {
		t.Errorf("stored STOP observations = %d, want 2 (the call that hit the limit and the one after)", stops)
	}
	_ = reply
}

func TestDependencyDownIsReportedNotRetriedForever(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	for range 10 {
		w.faults.Push(ledger.Timeout)
	}
	reply, err, c := w.runTurn(t, llm.Demo{}, Request{UserID: "taras", SessionID: "s1", Message: refundRequest})
	if err != nil {
		t.Fatal(err)
	}
	if got := c.get("OpenRefundCase"); got != int(sideEffectOptions.RetryPolicy.MaximumAttempts) {
		t.Errorf("attempts = %d, want MaximumAttempts", got)
	}
	if !strings.Contains(reply.Answer, "unavailable") || cases(t, w) != 0 || reply.SelfCorrections != 0 {
		t.Errorf("reply = %+v, cases = %d", reply, cases(t, w))
	}
}

func TestSessionAndMemorySurviveAcrossRuns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newWorld(t)

	if _, err, _ := w.runTurn(t, llm.Demo{}, Request{UserID: "taras", SessionID: "s1", Message: "Merchant A-114 integration is legacy"}); err != nil {
		t.Fatal(err)
	}
	// Turn 2 of the same session: the model must receive turn 1 as history,
	// which only happens if LoadSession brought it back from SQLite.
	//
	rec := &recorder{inner: llm.Demo{}}
	if _, err, c := w.runTurn(t, rec, Request{UserID: "taras", SessionID: "s1", Message: refundRequest}); err != nil || c.get("LoadSession") != 1 {
		t.Fatal(err)
	}
	if !strings.Contains(rec.firstPrompt, "Merchant A-114 integration is legacy") {
		t.Errorf("turn 2 did not see turn 1; first prompt:\n%s", rec.firstPrompt)
	}
	got, err := w.db.Sessions().Get(ctx, &session.GetRequest{AppName: AppName, UserID: "taras", SessionID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if n := got.Session.Events().Len(); n != 6 {
		t.Errorf("stored session has %d events, want 6 (2 from turn 1, 4 from turn 2)", n)
	}

	// A brand-new session recalls the fact from long-term memory.
	reply, err, c := w.runTurn(t, llm.Demo{}, Request{UserID: "taras", SessionID: "s2", Message: "What do you remember about A-114?"})
	if err != nil {
		t.Fatal(err)
	}
	if c.get("OpenNodes") != 1 || !strings.Contains(reply.Answer, "integration is legacy") {
		t.Errorf("recall reply = %+v", reply)
	}

	// Another user sees nothing of it.
	other, err, _ := w.runTurn(t, llm.Demo{}, Request{UserID: "iryna", SessionID: "s1", Message: "What do you remember about A-114?"})
	if err != nil || other.Answer != "I have no record of that." {
		t.Errorf("tenant leak: %+v, %v", other, err)
	}
}

func TestPersistSessionReplacesInsteadOfAppending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newWorld(t)
	if _, err, _ := w.runTurn(t, llm.Demo{}, Request{UserID: "taras", SessionID: "s1", Message: "Merchant A-114 is legacy"}); err != nil {
		t.Fatal(err)
	}
	acts := &Activities{Sessions: w.db.Sessions(), Memory: graphmemory.New(w.mem)}
	key := SessionKey{AppName, "taras", "s1"}
	snap, err := acts.LoadSession(ctx, key)
	if err != nil || snap == nil {
		t.Fatalf("LoadSession = %v, %v", snap, err)
	}
	for range 2 { // what a retried Activity does
		if err := acts.PersistSession(ctx, snap); err != nil {
			t.Fatal(err)
		}
		if err := acts.RememberSession(ctx, key); err != nil {
			t.Fatal(err)
		}
	}
	again, err := acts.LoadSession(ctx, key)
	if err != nil || len(again.Events) != len(snap.Events) {
		t.Fatalf("events after double persist = %d, want %d (%v)", len(again.Events), len(snap.Events), err)
	}
	if err := acts.PersistSession(ctx, nil); err == nil {
		t.Error("nil snapshot must be rejected")
	}
	if none, err := acts.LoadSession(ctx, SessionKey{AppName, "taras", "missing"}); err != nil || none != nil {
		t.Errorf("missing session = %v, %v; want nil, nil", none, err)
	}
	if err := acts.RememberSession(ctx, SessionKey{AppName, "taras", "missing"}); err == nil {
		t.Error("remembering a missing session must fail")
	}
}

// recorder captures the text of the first request it serves.
type recorder struct {
	inner       model.LLM
	mu          sync.Mutex
	firstPrompt string
}

func (r *recorder) Name() string { return r.inner.Name() }

func (r *recorder) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	r.mu.Lock()
	if r.firstPrompt == "" {
		var b strings.Builder
		for _, c := range req.Contents {
			for _, p := range c.Parts {
				b.WriteString(p.Text)
				b.WriteString("\n")
			}
		}
		r.firstPrompt = b.String()
	}
	r.mu.Unlock()
	return r.inner.GenerateContent(ctx, req, stream)
}

func TestReadOnlyToolAndKeywordRecall(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	reply, err, c := w.runTurn(t, llm.Demo{}, Request{UserID: "taras", SessionID: "s1", Message: "Show payouts for A-114"})
	if err != nil {
		t.Fatal(err)
	}
	if c.get("GetMerchantPayouts") != 1 || !strings.Contains(reply.Answer, "#418") {
		t.Errorf("payouts reply = %+v", reply)
	}

	if _, err, _ := w.runTurn(t, llm.Demo{}, Request{UserID: "taras", SessionID: "s2", Message: "Ticket LDG-1 is about payout delays"}); err != nil {
		t.Fatal(err)
	}
	reply, err, c = w.runTurn(t, llm.Demo{}, Request{UserID: "taras", SessionID: "s3", Message: "What do you remember about delays?"})
	if err != nil {
		t.Fatal(err)
	}
	if c.get("SearchNodes") != 1 || !strings.Contains(reply.Answer, "LDG-1") {
		t.Errorf("keyword recall reply = %+v", reply)
	}
}

func TestReadOnlySemanticErrorIsObserved(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	script := googleadk.NewFakeModel(
		googleadk.FunctionCallResponse("c1", "get_merchant_payouts", map[string]any{"merchant_id": "undefined"}),
		googleadk.TextResponse("could not read payouts"),
	)
	reply, err, c := w.runTurn(t, script, Request{UserID: "taras", SessionID: "s1", Message: "payouts?"})
	if err != nil {
		t.Fatal(err)
	}
	if c.get("GetMerchantPayouts") != 1 || !hasLine(reply.Log, `[ev:tool_observation ERROR] get_merchant_payouts`) || reply.SelfCorrections != 1 {
		t.Errorf("reply = %+v", reply)
	}
}

func TestToolsRefuseToRunOutsideAWorkflow(t *testing.T) {
	t.Parallel()
	ts := &toolset{key: SessionKey{AppName, "taras", "s1"}, budget: &budget{}}
	if _, err := ts.openRefundCase(nil, ledger.RefundCaseInput{}); !errors.Is(err, errNotInWorkflow) {
		t.Errorf("refund: %v", err)
	}
	if _, err := ts.getMerchantPayouts(nil, ledger.PayoutsInput{}); !errors.Is(err, errNotInWorkflow) {
		t.Errorf("payouts: %v", err)
	}
	if _, err := ts.searchNodes(nil, RecallInput{}); !errors.Is(err, errNotInWorkflow) {
		t.Errorf("search: %v", err)
	}
	if compact(map[string]any{"bad": make(chan int)}) == "" {
		t.Error("compact must fall back to fmt for unencodable values")
	}
}
