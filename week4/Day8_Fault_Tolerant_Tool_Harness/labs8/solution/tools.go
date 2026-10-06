package main

import (
	"errors"
	"time"

	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"

	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/graphmemory"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/ledger"
)

// Activity options. One place, because the retry policy IS the transient-error
// half of the harness: Temporal retries what classifyError called transient,
// and the non-retryable types below never get a second attempt.
var (
	sideEffectOptions = workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        200 * time.Millisecond,
			BackoffCoefficient:     2,
			MaximumInterval:        5 * time.Second,
			MaximumAttempts:        4,
			NonRetryableErrorTypes: []string{ErrTypeSemantic, ErrTypeFatal},
		},
	}
	readOptions = workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        200 * time.Millisecond,
			MaximumAttempts:        3,
			NonRetryableErrorTypes: []string{ErrTypeSemantic, ErrTypeFatal},
		},
	}
	// Storage must eventually succeed: if Dgraph or the disk is down, the turn
	// waits for it instead of losing the conversation. MaximumAttempts 0 means
	// "until it works".
	storeOptions = workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: 500 * time.Millisecond,
			MaximumInterval: 30 * time.Second,
		},
	}
)

// a is a nil receiver used only to name Activities in ExecuteActivity calls;
// the worker registers the real value.
var a *Activities

var errNotInWorkflow = errors.New("tool called outside a Temporal workflow")

// RefundResult is what open_refund_case returns to the model: the case on
// success, the observation otherwise. Flat on purpose: embedding two structs
// that both carry "status" makes encoding/json drop the field silently.
type RefundResult struct {
	Status      string `json:"status"`
	CaseID      string `json:"case_id,omitempty"`
	Replayed    bool   `json:"replayed,omitempty"`
	Field       string `json:"field,omitempty"`
	Observation string `json:"observation,omitempty"`
	Attempt     int    `json:"attempt,omitempty"`
}

func refundFromObservation(o Observation) RefundResult {
	return RefundResult{Status: o.Status, Field: o.Field, Observation: o.Observation, Attempt: o.Attempt}
}

// RecallInput is what the model may send to the memory tools. No user id:
// the tenant comes from the workflow request.
type RecallInput struct {
	Query string   `json:"query,omitempty" jsonschema:"Keywords or an id to look for in earlier conversations"`
	Names []string `json:"names,omitempty" jsonschema:"Exact ids to open, e.g. [\"A-114\"]"`
	Limit int      `json:"limit,omitempty" jsonschema:"Max remembered turns, default 20"`
}

// toolset builds the agent's tools for one run.
type toolset struct {
	key    SessionKey
	budget *budget
}

func (ts *toolset) all() ([]tool.Tool, error) {
	refund, err := functiontool.New(functiontool.Config{
		Name:        "open_refund_case",
		Description: "Opens a refund case for one transaction of one merchant. Needs transaction_id and merchant_id exactly as the user wrote them. May return status invalid_argument with an observation: read it, fix the named field, call again.",
	}, ts.openRefundCase)
	if err != nil {
		return nil, err
	}
	payouts, err := functiontool.New(functiontool.Config{
		Name:        "get_merchant_payouts",
		Description: "Read-only. Returns the payout status of one merchant.",
	}, ts.getMerchantPayouts)
	if err != nil {
		return nil, err
	}
	search, err := functiontool.New(functiontool.Config{
		Name:        "search_nodes",
		Description: "Searches what the user said in earlier conversations. Returns entities (merchants, transactions, cases, tickets) with dated observations.",
	}, ts.searchNodes)
	if err != nil {
		return nil, err
	}
	open, err := functiontool.New(functiontool.Config{
		Name:        "open_nodes",
		Description: "Returns everything remembered about the given ids, e.g. names [\"A-114\"], and the relations between them.",
	}, ts.openNodes)
	if err != nil {
		return nil, err
	}
	return []tool.Tool{refund, payouts, search, open}, nil
}

func (ts *toolset) openRefundCase(ctx agent.Context, in ledger.RefundCaseInput) (RefundResult, error) {
	wctx, ok := googleadk.WorkflowContext(ctx)
	if !ok {
		return RefundResult{}, errNotInWorkflow
	}
	if ts.budget.used > maxSelfCorrections {
		return RefundResult{Status: "stopped", Observation: "STOP: the self-correction limit is reached."}, nil
	}
	var out ledger.RefundCaseOutput
	err := workflow.ExecuteActivity(workflow.WithActivityOptions(wctx, sideEffectOptions), a.OpenRefundCase, in).Get(wctx, &out)
	if err != nil {
		obs, ferr := ts.budget.observe(err)
		return refundFromObservation(obs), ferr
	}
	return RefundResult{Status: out.Status, CaseID: out.CaseID, Replayed: out.Replayed}, nil
}

// PayoutsResult is what get_merchant_payouts returns to the model.
type PayoutsResult struct {
	Status      string `json:"status"`
	MerchantID  string `json:"merchant_id,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Observation string `json:"observation,omitempty"`
}

func (ts *toolset) getMerchantPayouts(ctx agent.Context, in ledger.PayoutsInput) (PayoutsResult, error) {
	wctx, ok := googleadk.WorkflowContext(ctx)
	if !ok {
		return PayoutsResult{}, errNotInWorkflow
	}
	var out ledger.PayoutsOutput
	err := workflow.ExecuteActivity(workflow.WithActivityOptions(wctx, readOptions), a.GetMerchantPayouts, in).Get(wctx, &out)
	if err != nil {
		obs, ferr := ts.budget.observe(err)
		return PayoutsResult{Status: obs.Status, Observation: obs.Observation}, ferr
	}
	return PayoutsResult{Status: "ok", MerchantID: out.MerchantID, Summary: out.Summary}, nil
}

func (ts *toolset) recall(ctx agent.Context, activity any, in RecallInput) (graphmemory.KnowledgeGraph, error) {
	wctx, ok := googleadk.WorkflowContext(ctx)
	if !ok {
		return graphmemory.KnowledgeGraph{}, errNotInWorkflow
	}
	args := RecallArgs{AppName: ts.key.AppName, UserID: ts.key.UserID, Query: in.Query, Names: in.Names, Limit: in.Limit}
	var g graphmemory.KnowledgeGraph
	err := workflow.ExecuteActivity(workflow.WithActivityOptions(wctx, readOptions), activity, args).Get(wctx, &g)
	return g, err
}

func (ts *toolset) searchNodes(ctx agent.Context, in RecallInput) (graphmemory.KnowledgeGraph, error) {
	return ts.recall(ctx, a.SearchNodes, in)
}

func (ts *toolset) openNodes(ctx agent.Context, in RecallInput) (graphmemory.KnowledgeGraph, error) {
	return ts.recall(ctx, a.OpenNodes, in)
}
