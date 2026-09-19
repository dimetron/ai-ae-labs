package fakellm

import (
	"context"
	"errors"
	"sync"
	"testing"

	"google.golang.org/adk/v2/model"
)

// Compile-time proof that Model satisfies the interface ADK actually requires.
// If a future ADK release changes model.LLM, this line fails to build — which
// is the point: the course would rather break at `go build` than on camera.
var _ model.LLM = (*Model)(nil)

// collect drains the iterator into slices, mirroring how ADK consumes it.
func collect(t *testing.T, m *Model, req *model.LLMRequest) ([]*model.LLMResponse, []error) {
	t.Helper()
	var responses []*model.LLMResponse
	var errs []error
	for resp, err := range m.GenerateContent(context.Background(), req, false) {
		if err != nil {
			errs = append(errs, err)
			continue
		}
		responses = append(responses, resp)
	}
	return responses, errs
}

func TestTextTurn(t *testing.T) {
	m := New("fake", TextTurn("привіт"))

	responses, errs := collect(t, m, &model.LLMRequest{Model: "fake"})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(responses) != 1 {
		t.Fatalf("got %d responses, want 1", len(responses))
	}
	parts := responses[0].Content.Parts
	if len(parts) != 1 || parts[0].Text != "привіт" {
		t.Errorf("got parts %+v, want single text part %q", parts, "привіт")
	}
	if responses[0].Content.Role != "model" {
		t.Errorf("role = %q, want %q", responses[0].Content.Role, "model")
	}
}

func TestCallTurn(t *testing.T) {
	m := New("fake", CallTurn("get_rate", map[string]any{"base": "UAH", "target": "EUR"}))

	responses, errs := collect(t, m, &model.LLMRequest{Model: "fake"})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	call := responses[0].Content.Parts[0].FunctionCall
	if call == nil {
		t.Fatal("FunctionCall = nil, want a call")
	}
	if call.Name != "get_rate" {
		t.Errorf("call.Name = %q, want %q", call.Name, "get_rate")
	}
	if call.Args["base"] != "UAH" || call.Args["target"] != "EUR" {
		t.Errorf("call.Args = %v, want base=UAH target=EUR", call.Args)
	}
}

func TestErrTurn(t *testing.T) {
	sentinel := errors.New("429 rate limited")
	m := New("fake", Turn{Err: sentinel})

	_, errs := collect(t, m, &model.LLMRequest{Model: "fake"})
	if len(errs) != 1 || !errors.Is(errs[0], sentinel) {
		t.Fatalf("errs = %v, want one %v", errs, sentinel)
	}
}

func TestScriptExhaustedIsLoud(t *testing.T) {
	// A silent zero value here would let a runaway agent loop look successful.
	m := New("fake", TextTurn("one"))

	if _, errs := collect(t, m, &model.LLMRequest{}); len(errs) != 0 {
		t.Fatalf("first call errored: %v", errs)
	}
	_, errs := collect(t, m, &model.LLMRequest{})
	if len(errs) != 1 || !errors.Is(errs[0], ErrScriptExhausted) {
		t.Fatalf("errs = %v, want ErrScriptExhausted", errs)
	}
}

func TestRequestsRecorded(t *testing.T) {
	m := New("fake", TextTurn("a"), TextTurn("b"))

	collect(t, m, &model.LLMRequest{Model: "first"})
	collect(t, m, &model.LLMRequest{Model: "second"})

	got := m.Requests()
	if len(got) != 2 {
		t.Fatalf("got %d requests, want 2", len(got))
	}
	if got[0].Model != "first" || got[1].Model != "second" {
		t.Errorf("request order = [%q %q], want [first second]", got[0].Model, got[1].Model)
	}
	if m.CallCount() != 2 {
		t.Errorf("CallCount() = %d, want 2", m.CallCount())
	}
	if m.Remaining() != 0 {
		t.Errorf("Remaining() = %d, want 0", m.Remaining())
	}
}

func TestRequestsIsACopy(t *testing.T) {
	m := New("fake", TextTurn("a"))
	collect(t, m, &model.LLMRequest{Model: "first"})

	got := m.Requests()
	got[0] = nil // mutate the caller's slice

	if m.Requests()[0] == nil {
		t.Error("Requests() returned the internal slice; callers can corrupt it")
	}
}

// Week 6 runs critic nodes concurrently. If the fake races, the lab's failures
// look like ADK bugs. Run with -race.
func TestConcurrentUse(t *testing.T) {
	const n = 16
	turns := make([]Turn, n)
	for i := range turns {
		turns[i] = TextTurn("ok")
	}
	m := New("fake", turns...)

	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collect(t, m, &model.LLMRequest{Model: "concurrent"})
		}()
	}
	wg.Wait()

	if m.CallCount() != n {
		t.Errorf("CallCount() = %d, want %d", m.CallCount(), n)
	}
	if m.Remaining() != 0 {
		t.Errorf("Remaining() = %d, want 0", m.Remaining())
	}
}
