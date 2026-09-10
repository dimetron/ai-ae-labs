// Agent for Week 6, Part 1: MultiCriticCodingAgent
// Dynamic graph with maker/critic loop — code generation with multi-critic review.
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

const maxAttempts = 5

// CriticVerdict holds a critic's evaluation.
type CriticVerdict struct {
	Critic   string `json:"critic"`
	Approved bool   `json:"approved"`
	Feedback string `json:"feedback"`
}

// codeMaker generates code based on a specification.
func codeMaker(ctx agent.Context, input string) (string, error) {
	return fmt.Sprintf("// Generated code for: %s\nfunc Process(data string) string {\n\treturn fmt.Sprintf(\"processed: %%s\", data)\n}", input), nil
}

// correctnessCritic checks code for correctness.
func correctnessCritic(ctx agent.Context, code string) (CriticVerdict, error) {
	if strings.Contains(code, "Process") {
		return CriticVerdict{Critic: "correctness", Approved: true, Feedback: "Code compiles and has correct function signature"}, nil
	}
	return CriticVerdict{Critic: "correctness", Approved: false, Feedback: "Missing required function"}, nil
}

// styleCritic checks code style.
func styleCritic(ctx agent.Context, code string) (CriticVerdict, error) {
	if strings.Contains(code, "fmt.Sprintf") {
		return CriticVerdict{Critic: "style", Approved: true, Feedback: "Good use of fmt package"}, nil
	}
	return CriticVerdict{Critic: "style", Approved: false, Feedback: "Use fmt.Sprintf instead of string concatenation"}, nil
}

// performanceCritic checks code performance.
func performanceCritic(ctx agent.Context, code string) (CriticVerdict, error) {
	if !strings.Contains(code, "strings.Builder") && strings.Count(code, "+") > 2 {
		return CriticVerdict{Critic: "performance", Approved: false, Feedback: "Use strings.Builder for string concatenation in loops"}, nil
	}
	return CriticVerdict{Critic: "performance", Approved: true, Feedback: "Performance looks good"}, nil
}

// multiCriticLoop is a DynamicNode that implements the maker/critic cycle.
func multiCriticLoop(ctx agent.Context, input string, emit func(*session.Event) error) (string, error) {
	critics := []struct {
		name string
		fn   func(ctx agent.Context, code string) (CriticVerdict, error)
	}{
		{"correctness", correctnessCritic},
		{"style", styleCritic},
		{"performance", performanceCritic},
	}

	var lastCode string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Maker step.
		code, err := codeMaker(ctx, input)
		if err != nil {
			return "", fmt.Errorf("maker failed: %w", err)
		}
		lastCode = code

		// Run all critics in sequence (simulating fan-out).
		allApproved := true
		var feedback strings.Builder
		feedback.WriteString(fmt.Sprintf("Attempt %d:\n", attempt))

		for _, critic := range critics {
			verdict, err := critic.fn(ctx, code)
			if err != nil {
				return "", fmt.Errorf("critic %s failed: %w", critic.name, err)
			}
			status := "✅"
			if !verdict.Approved {
				status = "❌"
				allApproved = false
			}
			feedback.WriteString(fmt.Sprintf("  %s %s: %s\n", status, verdict.Critic, verdict.Feedback))
		}

		// Emit progress.
		emit(session.NewEvent(ctx, ctx.InvocationID()))

		if allApproved {
			return fmt.Sprintf("✅ All critics approved on attempt %d\n\nCode:\n%s\n\nFeedback:\n%s", attempt, code, feedback.String()), nil
		}

		// Update input with feedback for next iteration.
		input = fmt.Sprintf("%s\nFeedback: %s", input, feedback.String())
	}

	return "", fmt.Errorf("failed after %d attempts. Last code:\n%s", maxAttempts, lastCode)
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("multi-critic", flag.ContinueOnError)
	spec := fs.String("spec", "write a function that processes input data", "code specification")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	loopNode := workflow.NewDynamicNode[string, string]("multi_critic_loop", multiCriticLoop, adkrun.NodeConfig())

	a, err := adkrun.Pipeline(
		"MultiCriticCodingAgent",
		"Dynamic graph with maker/critic loop — code generation with multi-critic review",
		loopNode,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	out, err := adkrun.Run(context.Background(), a, *spec, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Println(out)
	return 0
}
