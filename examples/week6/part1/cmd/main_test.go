package main

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"

	"github.com/dimetron/ai-eng-course/courses/AI_Agents_Engineering/code/internal/adkrun"
)

func TestCodeMaker(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	code, err := codeMaker(ctx, "process user data")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "Process") {
		t.Errorf("expected generated function, got: %s", code)
	}
}

func TestCorrectnessCritic_Approved(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	verdict, err := correctnessCritic(ctx, "func Process() {}")
	if err != nil {
		t.Fatal(err)
	}
	if !verdict.Approved {
		t.Error("expected approved for valid code")
	}
}

func TestCorrectnessCritic_Rejected(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	// Use a function name that does NOT contain "Process" as a substring.
	verdict, err := correctnessCritic(ctx, "func Handle() {}")
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Approved {
		t.Error("expected rejected for missing Process function")
	}
}

func TestStyleCritic_Approved(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	verdict, err := styleCritic(ctx, `fmt.Sprintf("hello")`)
	if err != nil {
		t.Fatal(err)
	}
	if !verdict.Approved {
		t.Error("expected approved for good style")
	}
}

func TestPerformanceCritic(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	// With 3+ concatenations and no strings.Builder, it should warn.
	verdict, err := performanceCritic(ctx, `func() string { return "a" + "b" + "c" + "d" }`)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Approved {
		t.Error("expected performance warning for excessive string concatenation")
	}
}

func TestMultiCriticLoop_Success(t *testing.T) {
	ctx := adkrun.NewTestContext(context.Background())
	emit := func(ev *session.Event) error { return nil }

	result, err := multiCriticLoop(ctx, "write a simple process function", emit)
	if err != nil {
		t.Fatalf("multiCriticLoop failed: %v", err)
	}
	if !strings.Contains(result, "All critics approved") {
		t.Errorf("expected success message, got: %s", result)
	}
}

func TestMultiCriticLoop_IterationCount(t *testing.T) {
	ctx := adkrun.NewTestContext(context.Background())
	emit := func(ev *session.Event) error { return nil }

	result, err := multiCriticLoop(ctx, "write a simple process function", emit)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Attempt 1") {
		t.Errorf("expected first attempt, got: %s", result)
	}
}
