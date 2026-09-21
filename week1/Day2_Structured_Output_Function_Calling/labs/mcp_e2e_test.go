package main

// TestE2EMonoMCPConnects proves MonoMCPToolset resolves the real
// mono-go-mcp binary and lists monobank tools over stdio — the exact
// production path the agent takes when -no-mcp is not set. This is an
// integration test: skipped when the server binary is not installed.

import (
	"context"
	"testing"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

// stubReadonlyCtx satisfies agent.ReadonlyContext for Toolset.Tools().
// Mono tools never read state; only the embedded context is used.
type stubReadonlyCtx struct {
	context.Context
	agent.ReadonlyContext
}

// Deadline disambiguates the Deadline promoted from the two embeds.
func (s *stubReadonlyCtx) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

// Done disambiguates the Done promoted from the two embeds.
func (s *stubReadonlyCtx) Done() <-chan struct{} {
	return s.Context.Done()
}

// Err disambiguates the Err promoted from the two embeds.
func (s *stubReadonlyCtx) Err() error {
	return s.Context.Err()
}

// Value disambiguates the Value promoted from the two embeds.
func (s *stubReadonlyCtx) Value(key any) any {
	return s.Context.Value(key)
}

// --- ReadonlyContext's own methods: mono tools never read agent state. ---

func (s *stubReadonlyCtx) UserContent() *genai.Content          { return nil }
func (s *stubReadonlyCtx) InvocationID() string                 { return "e2e" }
func (s *stubReadonlyCtx) AgentName() string                    { return "currency_agent" }
func (s *stubReadonlyCtx) ReadonlyState() session.ReadonlyState { return nil }
func (s *stubReadonlyCtx) UserID() string                       { return "e2e" }
func (s *stubReadonlyCtx) AppName() string                      { return "labs" }
func (s *stubReadonlyCtx) SessionID() string                    { return "e2e" }
func (s *stubReadonlyCtx) Branch() string                       { return "" }

func TestE2EMonoMCPConnects(t *testing.T) {
	ts, err := MonoMCPToolset()
	if err != nil {
		t.Skipf("mono-go-mcp not available: %v", err)
	}
	rc := &stubReadonlyCtx{Context: context.Background()}
	list, err := ts.Tools(rc)
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	var names []string
	for _, tl := range list {
		names = append(names, tl.Name())
	}
	t.Logf("mcp tools via ADK toolset: %v", names)
	want := []string{
		"mono_currency_rates", "mono_bank_sync", "mono_client_info",
		"mono_statement", "mono_set_webhook",
	}
	for _, w := range want {
		found := false
		for _, n := range names {
			if n == w {
				found = true
			}
		}
		if !found {
			t.Errorf("tool %q missing from tools/list; got %v", w, names)
		}
	}
}
