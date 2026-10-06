package main

import (
	"context"
	"errors"
	"fmt"

	"go.temporal.io/sdk/contrib/googleadk"
	"google.golang.org/adk/v2/session"

	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/graphmemory"
	"github.com/dimetron/ai-eng-course/labs/week4/Day8_Fault_Tolerant_Tool_Harness/labs8/internal/ledger"
)

// Activities is everything that touches the outside world: the acquirer, the
// SQLite session store, the memory graph. The workflow only orchestrates; it
// reaches these through workflow.ExecuteActivity, so each call is retried,
// timed out and recorded in history.
type Activities struct {
	Desk     *ledger.Desk
	Sessions session.Service
	Memory   *graphmemory.Service
}

// SessionKey names one conversation.
type SessionKey struct {
	AppName   string
	UserID    string
	SessionID string
}

// OpenRefundCase is the side-effecting tool. Its error is classified here,
// where the Go error type still exists.
func (a *Activities) OpenRefundCase(ctx context.Context, in ledger.RefundCaseInput) (ledger.RefundCaseOutput, error) {
	out, err := a.Desk.OpenRefundCase(ctx, in)
	return out, activityError(err)
}

// GetMerchantPayouts is the read-only tool.
func (a *Activities) GetMerchantPayouts(ctx context.Context, in ledger.PayoutsInput) (ledger.PayoutsOutput, error) {
	out, err := ledger.GetMerchantPayouts(ctx, in)
	return out, activityError(err)
}

// LoadSession returns the stored conversation, or nil for a new one.
func (a *Activities) LoadSession(ctx context.Context, k SessionKey) (*googleadk.SessionSnapshot, error) {
	_, err := a.Sessions.Get(ctx, &session.GetRequest{AppName: k.AppName, UserID: k.UserID, SessionID: k.SessionID})
	if errors.Is(err, session.ErrNotFound) {
		return nil, nil // a new conversation is not a failure
	}
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	snap, err := googleadk.ExportSession(ctx, a.Sessions, k.AppName, k.UserID, k.SessionID)
	if err != nil {
		return nil, fmt.Errorf("export session: %w", err)
	}
	return snap, nil
}

// PersistSession appends to the stored conversation the events of the
// snapshot it does not have yet.
//
// Append-only, keyed by event ID: the stored events are always a subset of the
// snapshot the workflow holds (the workflow started from them), so a retry —
// or a crash halfway through — appends exactly the missing tail and nothing
// twice. Deleting and re-importing would not work: the database service's
// Delete removes the session row but keeps its events.
//
// Event IDs are unique across runs because googleadk draws them from a random
// stream that Temporal seeds with the run ID.
func (a *Activities) PersistSession(ctx context.Context, snap *googleadk.SessionSnapshot) error {
	if snap == nil {
		return errors.New("persist session: nil snapshot")
	}
	got, err := a.Sessions.Get(ctx, &session.GetRequest{AppName: snap.AppName, UserID: snap.UserID, SessionID: snap.SessionID})
	var s session.Session
	switch {
	case errors.Is(err, session.ErrNotFound):
		created, cerr := a.Sessions.Create(ctx, &session.CreateRequest{
			AppName: snap.AppName, UserID: snap.UserID, SessionID: snap.SessionID, State: snap.State,
		})
		if cerr != nil {
			return fmt.Errorf("persist session: create: %w", cerr)
		}
		s = created.Session
	case err != nil:
		return fmt.Errorf("persist session: %w", err)
	default:
		s = got.Session
	}
	stored := map[string]bool{}
	for ev := range s.Events().All() {
		stored[ev.ID] = true
	}
	for _, ev := range snap.Events {
		if ev == nil || ev.Partial || stored[ev.ID] {
			continue
		}
		if err := a.Sessions.AppendEvent(ctx, s, ev); err != nil {
			return fmt.Errorf("persist session: append %s: %w", ev.ID, err)
		}
	}
	return nil
}

// RememberSession ingests the stored conversation into long-term memory.
// AddSessionToMemory is idempotent by event ID, so a retry adds nothing.
func (a *Activities) RememberSession(ctx context.Context, k SessionKey) error {
	got, err := a.Sessions.Get(ctx, &session.GetRequest{AppName: k.AppName, UserID: k.UserID, SessionID: k.SessionID})
	if err != nil {
		return fmt.Errorf("remember session: %w", err)
	}
	return a.Memory.AddSessionToMemory(ctx, got.Session)
}

// RecallArgs is what the memory tools send. The tenant (app, user) is set by
// the workflow from the request — never by the model.
type RecallArgs struct {
	AppName string
	UserID  string
	Query   string
	Names   []string
	Limit   int
}

// SearchNodes is the reference memory server's search_nodes.
func (a *Activities) SearchNodes(ctx context.Context, in RecallArgs) (graphmemory.KnowledgeGraph, error) {
	return a.Memory.SearchNodes(ctx, in.AppName, in.UserID, in.Query, in.Limit)
}

// OpenNodes is the reference memory server's open_nodes.
func (a *Activities) OpenNodes(ctx context.Context, in RecallArgs) (graphmemory.KnowledgeGraph, error) {
	return a.Memory.OpenNodes(ctx, in.AppName, in.UserID, in.Names, in.Limit)
}
