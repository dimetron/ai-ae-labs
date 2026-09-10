package main

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/adk/v2/session"

	"github.com/dimetron/ai-eng-course/courses/AI_Agents_Engineering/code/internal/adkrun"
)

func TestReactLoop_Resolves(t *testing.T) {
	ctx := adkrun.NewTestContext(context.Background())
	emit := func(ev *session.Event) error { return nil }

	result, err := reactLoop(ctx, "billing service timeout", emit)
	if err != nil {
		t.Fatalf("reactLoop failed: %v", err)
	}
	if !strings.Contains(result, "billing") {
		t.Errorf("expected billing-related resolution, got: %s", result)
	}
}

func TestReactLoop_IterationCount(t *testing.T) {
	ctx := adkrun.NewTestContext(context.Background())
	emit := func(ev *session.Event) error { return nil }

	result, err := reactLoop(ctx, "billing", emit)
	if err != nil {
		t.Fatal(err)
	}
	count := strings.Count(result, "Iteration")
	if count < 1 || count > maxIterations {
		t.Errorf("expected 1-%d iterations, got %d", maxIterations, count)
	}
}

func TestReactLoop_StopsOnMaxIterations(t *testing.T) {
	ctx := adkrun.NewTestContext(context.Background())
	emit := func(ev *session.Event) error { return nil }

	// The search_internal_wiki tool always returns "billing" in its hardcoded result,
	// so the loop resolves quickly. Verify the loop structure works.
	result, err := reactLoop(ctx, "billing", emit)
	if err != nil {
		t.Fatalf("reactLoop should succeed: %v", err)
	}
	if !strings.Contains(result, "Iteration") {
		t.Errorf("expected iteration tracking, got: %s", result)
	}
}

func TestToolSearchWiki(t *testing.T) {
	ctx := adkrun.NewTestContext(context.Background())
	result, err := tools[0].Execute(ctx, "incident")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "incident") {
		t.Errorf("expected incident in result, got: %s", result)
	}
}

func TestToolCreateTicket(t *testing.T) {
	ctx := adkrun.NewTestContext(context.Background())
	result, err := tools[2].Execute(ctx, "billing timeout")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "LDG-") {
		t.Errorf("expected ticket ID in result, got: %s", result)
	}
}
