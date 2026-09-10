package main

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/adk/v2/agent"
)

func TestPlanDecomposition(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	result, err := planDecomposition(ctx, "plan the migration off the legacy processor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "task-1") {
		t.Errorf("expected task-1 in decomposition, got: %s", result)
	}
	if !strings.Contains(result, "risk") {
		t.Errorf("expected risk task in decomposition, got: %s", result)
	}
}

func TestRiskAnalyst(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	result, err := riskAnalyst(ctx, "migration risks")
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != "risk" {
		t.Errorf("expected type 'risk', got %q", result.Type)
	}
	if !strings.Contains(result.Findings, "settlement") {
		t.Errorf("expected settlement risk in findings, got: %s", result.Findings)
	}
}

func TestComplianceAnalyst(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	result, err := complianceAnalyst(ctx, "compliance impact")
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != "compliance" {
		t.Errorf("expected type 'compliance', got %q", result.Type)
	}
	if !strings.Contains(result.Findings, "PCI DSS") {
		t.Errorf("expected PCI DSS in findings, got: %s", result.Findings)
	}
}

func TestIntegrationAnalyst(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	result, err := integrationAnalyst(ctx, "merchants on legacy")
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != "integration" {
		t.Errorf("expected type 'integration', got %q", result.Type)
	}
	if !strings.Contains(result.Findings, "A-114") {
		t.Errorf("expected merchant A-114 in findings, got: %s", result.Findings)
	}
}

func TestJoinResults(t *testing.T) {
	ctx := &agent.StrictContextMock{Ctx: context.Background()}
	results := map[string]any{
		"risk":        TaskResult{TaskID: "t1", Type: "risk", Summary: "Risk Summary", Findings: "risk data"},
		"compliance":  TaskResult{TaskID: "t2", Type: "compliance", Summary: "Compliance Summary", Findings: "compliance data"},
		"integration": TaskResult{TaskID: "t3", Type: "integration", Summary: "Integration Summary", Findings: "integration data"},
	}
	out, err := joinResults(ctx, results)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Migration Plan") {
		t.Errorf("expected migration plan header, got: %s", out)
	}
	if !strings.Contains(out, "human approval") {
		t.Errorf("expected approval notice, got: %s", out)
	}
}
