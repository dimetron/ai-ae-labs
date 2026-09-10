// Agent for Week 6, Part 2: PersonalResearchAgent
// Autoresearch pattern — self-improving agent with bounded surface, fixed budget,
// single fitness metric, auditable strategy, and full experiment log.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/dimetron/ai-eng-course/courses/AI_Agents_Engineering/code/internal/adkrun"
)

const (
	maxExperiments = 20
	evalBudget     = 60 // seconds per experiment
)

// Experiment records one self-improvement trial.
type Experiment struct {
	ID        int       `json:"id"`
	Strategy  string    `json:"strategy"`
	Code      string    `json:"code"`
	Metric    float64   `json:"metric"` // lower is better (e.g. validation loss)
	Duration  float64   `json:"duration_seconds"`
	Kept      bool      `json:"kept"`
	Timestamp time.Time `json:"timestamp"`
}

// ExperimentLog stores the full audit trail.
type ExperimentLog struct {
	mu         sync.Mutex
	experiments []Experiment
	bestMetric float64
	bestCode   string
}

var log = &ExperimentLog{bestMetric: 1e9}

func (l *ExperimentLog) Add(exp Experiment) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.experiments = append(l.experiments, exp)
	if exp.Metric < l.bestMetric && exp.Kept {
		l.bestMetric = exp.Metric
		l.bestCode = exp.Code
	}
}

func (l *ExperimentLog) All() []Experiment {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]Experiment, len(l.experiments))
	copy(result, l.experiments)
	return result
}

// generateExperiment creates a new experiment variant.
func generateExperiment(ctx agent.Context, input string) (string, error) {
	// Simulate generating a code variant.
	return fmt.Sprintf("experiment_variant for: %s\nfunc optimized(data string) string {\n\t// Optimized version\n\treturn data\n}", input), nil
}

// evaluateExperiment runs the experiment and returns a fitness metric.
func evaluateExperiment(ctx agent.Context, code string) (string, error) {
	// Simulate evaluation: return a metric (lower is better).
	metric := 0.1 + float64(time.Now().UnixNano()%100)/1000.0
	return fmt.Sprintf("metric=%.4f", metric), nil
}

// decideKeep decides whether to keep the experiment based on the metric.
func decideKeep(ctx agent.Context, input string) (string, error) {
	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("expected format: metric=<value>")
	}
	var metric float64
	fmt.Sscanf(parts[1], "%f", &metric)

	kept := metric < log.bestMetric
	if kept {
		return fmt.Sprintf("KEPT (metric=%.4f, best=%.4f)", metric, log.bestMetric), nil
	}
	return fmt.Sprintf("DISCARDED (metric=%.4f, best=%.4f)", metric, log.bestMetric), nil
}

// updateStrategy updates the improvement strategy based on experiment results.
func updateStrategy(ctx agent.Context, input string) (string, error) {
	exps := log.All()
	if len(exps) == 0 {
		return "strategy: initial exploration phase", nil
	}

	// Simple strategy adaptation: if last 3 experiments were kept, try more aggressive changes.
	recent := 0
	for i := len(exps) - 1; i >= 0 && i > len(exps)-4; i-- {
		if exps[i].Kept {
			recent++
		}
	}

	if recent >= 3 {
		return "strategy: aggressive optimization (increasing mutation rate)", nil
	}
	return "strategy: conservative refinement (small changes)", nil
}

// researchLoop is a DynamicNode implementing the autoresearch pattern.
func researchLoop(ctx agent.Context, input string, emit func(*session.Event) error) (string, error) {
	strategy := "initial exploration"

	for expID := 1; expID <= maxExperiments; expID++ {
		// 1. Generate experiment.
		code, err := generateExperiment(ctx, input)
		if err != nil {
			return "", fmt.Errorf("generate failed: %w", err)
		}

		// 2. Evaluate.
		evalResult, err := evaluateExperiment(ctx, code)
		if err != nil {
			return "", fmt.Errorf("evaluate failed: %w", err)
		}

		// 3. Parse metric.
		var metric float64
		fmt.Sscanf(evalResult, "metric=%f", &metric)

		// 4. Decide keep/discard.
		kept := metric < log.bestMetric
		if kept {
			log.bestMetric = metric
			log.bestCode = code
		}

		// 5. Log experiment.
		exp := Experiment{
			ID:        expID,
			Strategy:  strategy,
			Code:      code,
			Metric:    metric,
			Duration:  evalBudget,
			Kept:      kept,
			Timestamp: time.Now(),
		}
		log.Add(exp)

		// 6. Update strategy.
		strategyResult, _ := updateStrategy(ctx, input)
		strategy = strategyResult

		// Emit progress.
		emit(session.NewEvent(ctx, ctx.InvocationID()))

		// Early stop if metric is good enough.
		if metric < 0.05 {
			return fmt.Sprintf("✅ Early stop: reached target metric %.4f after %d experiments\nBest metric: %.4f", metric, expID, log.bestMetric), nil
		}
	}

	return fmt.Sprintf("Completed %d experiments. Best metric: %.4f", maxExperiments, log.bestMetric), nil
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("research-agent", flag.ContinueOnError)
	task := fs.String("task", "optimize data processing function", "research task")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	loopNode := workflow.NewDynamicNode[string, string]("research_loop", researchLoop, adkrun.NodeConfig())

	a, err := adkrun.Pipeline(
		"PersonalResearchAgent",
		"Autoresearch pattern — self-improving agent with bounded surface, fixed budget, single metric",
		loopNode,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	out, err := adkrun.Run(context.Background(), a, *task, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Println(out)
	return 0
}
