package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/platform"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

// AppName scopes sessions and memory.
const AppName = "ledgerworks"

// ModelName is the logical model the workflow asks for. Only this name
// crosses into workflow history; the worker maps it to a real model.
const ModelName = "ledger-model"

// instruction confines the agent to its domain and tells it how to use the
// observations the harness produces.
const instruction = `You are the LEDGERWORKS refund desk agent.
You handle only: refund cases, merchant payouts, and what this user told you in earlier conversations about merchants, transactions, cases and tickets.
For anything else, refuse in one sentence.

Rules:
- Copy ids exactly as the user wrote them: merchants look like A-114, transactions like txn-2026-07-118845. Never send placeholders such as "undefined".
- If a tool returns status "invalid_argument", read the observation, fix the named field from the user's message, and call the tool again.
- If a tool returns status "unavailable" or "stopped", do not call it again; tell the user what happened.
- If the user refers to something from an earlier conversation, call open_nodes (for a known id) or search_nodes (for keywords) before answering. If nothing is found, say there is no record. Do not guess.`

var modelOptions = workflow.ActivityOptions{
	StartToCloseTimeout: 2 * time.Minute,
	RetryPolicy:         &temporal.RetryPolicy{InitialInterval: time.Second, MaximumAttempts: 3},
}

// Request is one user turn.
type Request struct {
	UserID    string
	SessionID string
	Message   string
}

// Reply is the result of one turn, with the event log that answers "what did
// the agent do?".
type Reply struct {
	Answer          string
	Log             []string
	SelfCorrections int
}

// AgentWorkflow runs one turn of the refund agent durably.
//
// The workflow orchestrates and never does I/O itself: every LLM call
// (googleadk.NewModel) and every tool, session and memory operation is an
// Activity. After a crash, Temporal replays this function from history and
// hands back the recorded results of finished Activities — a finished LLM call
// is not made again.
func AgentWorkflow(ctx workflow.Context, req Request) (Reply, error) {
	key := SessionKey{AppName: AppName, UserID: req.UserID, SessionID: req.SessionID}
	sctx := workflow.WithActivityOptions(ctx, storeOptions)

	var snap *googleadk.SessionSnapshot
	if err := workflow.ExecuteActivity(sctx, a.LoadSession, key).Get(sctx, &snap); err != nil {
		return Reply{}, err
	}

	adkCtx := platform.WithUUIDProvider(googleadk.NewContext(ctx), eventIDs(ctx))
	sessions := session.InMemoryService() // durable via history; SQLite is written by PersistSession
	if snap != nil {
		if _, err := googleadk.ImportSession(adkCtx, sessions, snap); err != nil {
			return Reply{}, fmt.Errorf("import session: %w", err)
		}
	}

	ts := &toolset{key: key, budget: &budget{}}
	tools, err := ts.all()
	if err != nil {
		return Reply{}, err
	}
	root, err := llmagent.New(llmagent.Config{
		Name:        "refund_agent",
		Description: "Opens refund cases and recalls earlier conversations for LEDGERWORKS.",
		Model:       googleadk.NewModel(ModelName, googleadk.WithModelActivityOptions(modelOptions)),
		Instruction: instruction,
		Tools:       tools,
	})
	if err != nil {
		return Reply{}, err
	}
	// No MemoryService on purpose: memory is reached through Activities only.
	r, err := runner.New(runner.Config{AppName: AppName, Agent: root, SessionService: sessions, AutoCreateSession: true})
	if err != nil {
		return Reply{}, err
	}

	var reply Reply
	msg := genai.NewContentFromText(req.Message, genai.RoleUser)
	for ev, err := range r.Run(adkCtx, req.UserID, req.SessionID, msg, agent.RunConfig{}) {
		if err != nil {
			return reply, err
		}
		reply.record(ev)
	}
	reply.SelfCorrections = ts.budget.used

	out, err := googleadk.ExportSession(adkCtx, sessions, AppName, req.UserID, req.SessionID)
	if err != nil {
		return reply, fmt.Errorf("export session: %w", err)
	}
	if err := workflow.ExecuteActivity(sctx, a.PersistSession, out).Get(sctx, nil); err != nil {
		return reply, err
	}
	if err := workflow.ExecuteActivity(sctx, a.RememberSession, key).Get(sctx, nil); err != nil {
		return reply, err
	}
	if err := ts.budget.exhausted(); err != nil {
		reply.Log = append(reply.Log, "[ev:failed] "+err.Error())
		return reply, err
	}
	reply.Log = append(reply.Log, "[ev:success]")
	return reply, nil
}

// eventIDs names ADK events after the workflow execution that created them.
//
// Event IDs are the idempotency keys of PersistSession and of memory
// ingestion, so they must be unique across runs, not only within one.
// googleadk's own provider is deterministic per run but draws from a stream
// seeded by the run ID alone; hashing workflow ID + run ID + a counter keeps
// replay determinism and makes the identity explicit. Read-only contexts
// (queries) get random IDs, as in googleadk.
func eventIDs(ctx workflow.Context) platform.UUIDProvider {
	exec := workflow.GetInfo(ctx).WorkflowExecution
	var n atomic.Uint64
	return func() string {
		if workflow.IsReadOnly(ctx) {
			return uuid.NewString()
		}
		name := fmt.Sprintf("%s/%s/%d", exec.ID, exec.RunID, n.Add(1))
		return uuid.NewSHA1(uuid.NameSpaceOID, []byte(name)).String()
	}
}

// record appends one event to the log in the shape the README shows.
func (r *Reply) record(ev *session.Event) {
	if ev == nil || ev.Content == nil {
		return
	}
	calls := false
	for _, p := range ev.Content.Parts {
		if p != nil && p.FunctionCall != nil {
			calls = true
		}
	}
	for _, p := range ev.Content.Parts {
		switch {
		case p == nil:
		case p.FunctionCall != nil:
			r.Log = append(r.Log, "[ev:tool_call] "+p.FunctionCall.Name+" "+compact(p.FunctionCall.Args))
		case p.FunctionResponse != nil:
			tag := "[ev:tool_observation] "
			if s, _ := p.FunctionResponse.Response["status"].(string); s == "invalid_argument" || s == "unavailable" || s == "stopped" {
				tag = "[ev:tool_observation ERROR] "
			}
			r.Log = append(r.Log, tag+p.FunctionResponse.Name+" "+compact(p.FunctionResponse.Response))
		case strings.TrimSpace(p.Text) != "" && (calls || p.Thought):
			r.Log = append(r.Log, "[ev:reasoning] "+strings.TrimSpace(p.Text))
		case strings.TrimSpace(p.Text) != "":
			r.Answer = strings.TrimSpace(p.Text)
			r.Log = append(r.Log, "[ev:answer] "+r.Answer)
		}
	}
}

// compact renders a map as one line of JSON with sorted keys (deterministic).
func compact(v map[string]any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}
