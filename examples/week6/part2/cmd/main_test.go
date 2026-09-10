package main

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"

	"github.com/dimetron/ai-eng-course/courses/AI_Agents_Engineering/code/internal/adkrun"
)

func TestGenerateExperiment(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	code, err := generateExperiment(ctx, "optimize data processing")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "experiment_variant") {
		t.Errorf("expected experiment variant, got: %s", code)
	}
}

func TestEvaluateExperiment(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	result, err := evaluateExperiment(ctx, "func test() {}")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "metric=") {
		t.Errorf("expected metric in result, got: %s", result)
	}
}

func TestDecideKeep_Better(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	log.bestMetric = 0.5
	result, err := decideKeep(ctx, "metric=0.3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "KEPT") {
		t.Errorf("expected KEPT for better metric, got: %s", result)
	}
}

func TestDecideKeep_Worse(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	log.bestMetric = 0.1
	result, err := decideKeep(ctx, "metric=0.5")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "DISCARDED") {
		t.Errorf("expected DISCARDED for worse metric, got: %s", result)
	}
}

func TestUpdateStrategy(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	strategy, err := updateStrategy(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strategy, "strategy:") {
		t.Errorf("expected strategy prefix, got: %s", strategy)
	}
}

func TestExperimentLog(t *testing.T) {
	// Reset log for clean test.
	log.bestMetric = 1e9
	log.experiments = nil

	log.Add(Experiment{ID: 1, Metric: 0.5, Kept: true})
	log.Add(Experiment{ID: 2, Metric: 0.3, Kept: true})
	all := log.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 experiments, got %d", len(all))
	}
	if log.bestMetric != 0.3 {
		t.Errorf("expected best metric 0.3, got %f", log.bestMetric)
	}
}

func TestResearchLoop_EarlyStop(t *testing.T) {
	ctx := adkrun.NewTestContext(context.Background())
	emit := func(ev *session.Event) error { return nil }

	// Reset log for clean test.
	log.bestMetric = 1e9
	log.experiments = nil

	result, err := researchLoop(ctx, "optimize test function", emit)
	if err != nil {
		t.Fatalf("researchLoop failed: %v", err)
	}
	if !strings.Contains(result, "experiments") && !strings.Contains(result, "Early stop") {
		t.Errorf("expected completion message, got: %s", result)
	}
}

func TestResearchLoop_ExperimentCount(t *testing.T) {
	ctx := adkrun.NewTestContext(context.Background())
	emit := func(ev *session.Event) error { return nil }

	// Reset log for clean test.
	log.bestMetric = 1e9
	log.experiments = nil

	_, err := researchLoop(ctx, "test", emit)
	if err != nil {
		t.Fatal(err)
	}
	all := log.All()
	if len(all) == 0 {
		t.Error("expected at least 1 experiment")
	}
}
