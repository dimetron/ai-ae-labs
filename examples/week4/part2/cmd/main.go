// Agent for Week 4, Part 2: FaultTolerantToolHarness
// Error classification + retry harness with transient vs semantic error handling.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/workflow"

	"github.com/dimetron/ai-eng-course/courses/AI_Agents_Engineering/code/internal/adkrun"
)

// Sentinel errors for tool classification.
var (
	ErrTransient = errors.New("transient: retryable error")
	ErrSemantic  = errors.New("semantic: invalid input, no retry")
)

// ToolResult holds the result of a tool execution.
type ToolResult struct {
	Success    bool   `json:"success"`
	Output     string `json:"output"`
	ErrorClass string `json:"error_class,omitempty"` // "transient" or "semantic"
	Attempts   int    `json:"attempts"`
}

// classifyError classifies an error as transient or semantic.
func classifyError(err error) string {
	if errors.Is(err, ErrTransient) {
		return "transient"
	}
	if errors.Is(err, ErrSemantic) {
		return "semantic"
	}
	return "transient" // default to transient for unknown errors
}

// flakyCheck simulates a flaky tool with controlled failure probability.
func flakyCheck(ctx agent.Context, input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("%w: input cannot be empty", ErrSemantic)
	}
	if strings.Contains(input, "undefined") {
		return "", fmt.Errorf("%w: received 'undefined' as argument", ErrSemantic)
	}
	if rand.Float64() < 0.3 {
		return "", fmt.Errorf("%w: service temporarily unavailable", ErrTransient)
	}
	return fmt.Sprintf("OK: processed %q", input), nil
}

// toolHarness wraps a tool with error classification and retry logic.
func toolHarness(ctx agent.Context, input string) (ToolResult, error) {
	maxAttempts := 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := flakyCheck(ctx, input)
		if err == nil {
			slog.Info("tool succeeded", "input", input, "attempt", attempt)
			return ToolResult{Success: true, Output: result, Attempts: attempt}, nil
		}

		lastErr = err
		errorClass := classifyError(err)
		slog.Warn("tool failed",
			"input", input,
			"attempt", attempt,
			"error_class", errorClass,
			"error", err,
		)

		if errorClass == "semantic" {
			// Don't retry semantic errors — return as observation for the model.
			return ToolResult{
				Success:    false,
				Output:     fmt.Sprintf("Error: %v", err),
				ErrorClass: "semantic",
				Attempts:   attempt,
			}, nil
		}

		// Transient: retry with backoff.
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
		}
	}

	return ToolResult{
		Success:    false,
		Output:     fmt.Sprintf("Failed after %d attempts: %v", maxAttempts, lastErr),
		ErrorClass: "transient",
		Attempts:   maxAttempts,
	}, nil
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("fault-tolerant-harness", flag.ContinueOnError)
	input := fs.String("input", "test refund case", "tool input")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	a, err := adkrun.Pipeline(
		"FaultTolerantToolHarness",
		"Error classification + retry harness with transient vs semantic error handling",
		workflow.NewFunctionNode[string, ToolResult]("harness", toolHarness, adkrun.NodeConfig()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	out, err := adkrun.Run(context.Background(), a, *input, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Println(out)
	return 0
}
