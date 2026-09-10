// Agent for Week 5, Part 2: CollaborativeWorkflowEngine
// Coordinator + specialists + HITL with Plan-Execute routing.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/dimetron/ai-eng-course/courses/AI_Agents_Engineering/code/internal/adkrun"
)

// Task represents a work item for a specialist.
type Task struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // "risk", "compliance", "integration"
	Query   string `json:"query"`
	Payload string `json:"payload,omitempty"`
}

// TaskResult holds a specialist's output.
type TaskResult struct {
	TaskID   string `json:"task_id"`
	Type     string `json:"type"`
	Summary  string `json:"summary"`
	Findings string `json:"findings"`
}

// planDecomposition breaks a complex query into specialist tasks.
func planDecomposition(ctx agent.Context, input string) (string, error) {
	tasks := []Task{
		{ID: "task-1", Type: "risk", Query: input, Payload: "Analyze migration risks"},
		{ID: "task-2", Type: "compliance", Query: input, Payload: "Assess compliance impact"},
		{ID: "task-3", Type: "integration", Query: input, Payload: "Review merchant integrations"},
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Decomposed into %d tasks:\n", len(tasks)))
	for _, t := range tasks {
		b.WriteString(fmt.Sprintf("- %s: %s (%s)\n", t.ID, t.Type, t.Payload))
	}
	return b.String(), nil
}

// riskAnalyst simulates a risk analysis specialist.
func riskAnalyst(ctx agent.Context, input string) (TaskResult, error) {
	return TaskResult{
		TaskID: "task-1", Type: "risk",
		Summary:  "Risk Analysis Complete",
		Findings: "Key risks: settlement downtime during cutover (severity: critical), payout queue backlog (est. 4h), rollback window narrower than one settlement cycle",
	}, nil
}

// complianceAnalyst simulates a compliance impact specialist.
func complianceAnalyst(ctx agent.Context, input string) (TaskResult, error) {
	return TaskResult{
		TaskID: "task-2", Type: "compliance",
		Summary:  "Compliance Review Complete",
		Findings: "PCI DSS re-scoping required for the new processor; NBU 2026 reporting obligation applies from the first settlement day; GDPR data-residency unchanged",
	}, nil
}

// integrationAnalyst simulates a merchant-integration specialist.
func integrationAnalyst(ctx agent.Context, input string) (TaskResult, error) {
	return TaskResult{
		TaskID: "task-3", Type: "integration",
		Summary:  "Integration Review Complete",
		Findings: "Merchant A-114 is still on the legacy payout API; acquirer contract #2024-ACME-17 signed by L. Doroshenko, Head of Partnerships, Acme Bank, renewal clause in Q1 2027",
	}, nil
}

// joinResults aggregates all specialist results.
func joinResults(ctx agent.Context, results map[string]any) (string, error) {
	var b strings.Builder
	b.WriteString("## Migration Plan Summary\n\n")
	for _, v := range results {
		if r, ok := v.(TaskResult); ok {
			b.WriteString(fmt.Sprintf("### %s\n%s\n\n", r.Summary, r.Findings))
		}
	}
	b.WriteString("---\nRequires human approval before sending to board.\n")
	return b.String(), nil
}

// requestApproval is a HITL pause node.
func requestApproval(ctx agent.Context, input string, emit func(*session.Event) error) (string, error) {
	// Emit a request for human input.
	ev := session.NewEvent(ctx, ctx.InvocationID())
	ev.RequestedInput = &session.RequestInput{
		InterruptID: "board-approval-" + ctx.InvocationID(),
		Message:     "Approve migration plan? (yes/no):\n\n" + input,
	}
	if err := emit(ev); err != nil {
		return "", err
	}
	return "", workflow.ErrNodeInterrupted
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("collaborative-workflow", flag.ContinueOnError)
	query := fs.String("query", "prepare a migration plan off the legacy processor for the board: risks, compliance impact, merchants on legacy", "executive query")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Build the collaborative workflow with fan-out.
	planner := workflow.NewFunctionNode[string, string]("planner", planDecomposition, adkrun.NodeConfig())
	risk := workflow.NewFunctionNode[string, TaskResult]("risk_analyst", riskAnalyst, adkrun.NodeConfig())
	compliance := workflow.NewFunctionNode[string, TaskResult]("compliance_analyst", complianceAnalyst, adkrun.NodeConfig())
	integration := workflow.NewFunctionNode[string, TaskResult]("integration_analyst", integrationAnalyst, adkrun.NodeConfig())
	joiner := workflow.NewJoinNode("joiner")
	approval := workflow.NewEmittingFunctionNode[string, string]("approval", requestApproval, adkrun.NodeConfig())

	eb := workflow.NewEdgeBuilder()
	eb.Add(workflow.Start, planner)
	eb.Add(planner, risk)
	eb.Add(planner, compliance)
	eb.Add(planner, integration)
	eb.Add(risk, joiner)
	eb.Add(compliance, joiner)
	eb.Add(integration, joiner)
	eb.Add(joiner, approval)

	a, err := adkrun.Pipeline(
		"CollaborativeWorkflowEngine",
		"Coordinator + specialists + HITL with Plan-Execute routing",
		approval,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	out, err := adkrun.Run(context.Background(), a, *query, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Println(out)
	return 0
}
