// Agent for Week 4, Part 1: NativeReActAgent
// Manual ReAct loop + ADK DynamicNode equivalent with maxIterations guard.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/dimetron/ai-eng-course/courses/AI_Agents_Engineering/code/internal/adkrun"
)

const maxIterations = 10

// Tool represents an available tool.
type Tool struct {
	Name        string
	Description string
	Execute     func(ctx agent.Context, args string) (string, error)
}

// tools available to the agent.
var tools = []Tool{
	{
		Name: "search_internal_wiki", Description: "search internal wiki for information",
		Execute: func(ctx agent.Context, args string) (string, error) {
			return fmt.Sprintf("Wiki results for: %s\n- Incident report #423: auth timeout in billing service", args), nil
		},
	},
	{
		Name: "get_merchant_payouts", Description: "read recent payouts for a merchant",
		Execute: func(ctx agent.Context, args string) (string, error) {
			return fmt.Sprintf("Payouts for: %s\n- A-114: 2 payouts pending since 03:10, batch #423 delayed", args), nil
		},
	},
	{
		Name: "create_jira_ticket", Description: "create a Jira ticket",
		Execute: func(ctx agent.Context, args string) (string, error) {
			return fmt.Sprintf("Created ticket LDG-%d", len(args)), nil
		},
	},
}

// ReActState holds the loop state.
type ReActState struct {
	Thought     string   `json:"thought"`
	Action      string   `json:"action"`
	Observation string   `json:"observation"`
	Iteration   int      `json:"iteration"`
	History     []string `json:"history"`
	Done        bool     `json:"done"`
}

// reactLoop is a DynamicNode that implements the ReAct pattern.
func reactLoop(ctx agent.Context, input string, emit func(*session.Event) error) (string, error) {
	state := ReActState{
		Thought:   fmt.Sprintf("Starting investigation: %s", input),
		Iteration: 0,
		Done:      false,
	}

	for state.Iteration < maxIterations && !state.Done {
		state.Iteration++

		// Thought step: decide what to do.
		state.Thought = fmt.Sprintf("Iteration %d: analyzing results for '%s'", state.Iteration, input)
		state.History = append(state.History, fmt.Sprintf("Thought: %s", state.Thought))

		// Action step: pick a tool.
		tool := tools[state.Iteration%len(tools)]
		state.Action = fmt.Sprintf("%s(query=%q)", tool.Name, input)
		state.History = append(state.History, fmt.Sprintf("Action: %s", state.Action))

		// Execute tool.
		result, err := tool.Execute(ctx, input)
		if err != nil {
			state.Observation = fmt.Sprintf("Error: %v", err)
		} else {
			state.Observation = result
		}
		state.History = append(state.History, fmt.Sprintf("Observation: %s", state.Observation))

		// Check if we have enough information.
		if strings.Contains(state.Observation, "billing") || strings.Contains(state.Observation, "LDG-") {
			state.Done = true
		}

		// Emit progress.
		emit(session.NewEvent(ctx, ctx.InvocationID()))
	}

	if !state.Done {
		return "", fmt.Errorf("max iterations (%d) reached without resolution", maxIterations)
	}

	return strings.Join(state.History, "\n"), nil
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("react-agent", flag.ContinueOnError)
	query := fs.String("query", "investigate billing service timeout", "incident query")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Build a workflow with a DynamicNode for the ReAct loop.
	loopNode := workflow.NewDynamicNode[string, string]("react_loop", reactLoop, adkrun.NodeConfig())

	a, err := adkrun.Pipeline(
		"NativeReActAgent",
		"Manual ReAct loop with maxIterations guard",
		loopNode,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	out, err := adkrun.Run(context.Background(), a, *query, os.Stdout)
	if err != nil {
		// Check for max iterations error.
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "max iterations") {
			fmt.Fprintf(os.Stderr, "agent hit iteration limit: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Println(out)
	return 0
}
