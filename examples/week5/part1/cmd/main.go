// Agent for Week 5, Part 1: EventSourcedMemoryAgent
// Two-layer memory with StateDelta for event-sourced state management.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/workflow"

	"github.com/dimetron/ai-eng-course/courses/AI_Agents_Engineering/code/internal/adkrun"
)

// Fact represents a remembered fact.
type Fact struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// MemoryStore is a simple in-memory long-term memory store.
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]string
}

var longTermMemory = &MemoryStore{data: make(map[string]string)}

func (m *MemoryStore) Get(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	return v, ok
}

func (m *MemoryStore) Set(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

func (m *MemoryStore) Search(prefix string) []Fact {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []Fact
	for k, v := range m.data {
		if strings.HasPrefix(k, prefix) {
			results = append(results, Fact{Key: k, Value: v, Timestamp: time.Now()})
		}
	}
	return results
}

// rememberFact stores a fact in long-term memory.
func rememberFact(ctx agent.Context, input string) (string, error) {
	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("expected format: key=value")
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	longTermMemory.Set(key, value)
	return fmt.Sprintf("Remembered: %s = %s", key, value), nil
}

// recallFact retrieves a fact from long-term memory.
func recallFact(ctx agent.Context, input string) (string, error) {
	if value, ok := longTermMemory.Get(input); ok {
		return fmt.Sprintf("%s = %s", input, value), nil
	}
	// Try prefix search.
	results := longTermMemory.Search(input)
	if len(results) > 0 {
		var b strings.Builder
		for _, f := range results {
			b.WriteString(fmt.Sprintf("%s = %s\n", f.Key, f.Value))
		}
		return strings.TrimSpace(b.String()), nil
	}
	return fmt.Sprintf("No memory found for: %s", input), nil
}

// formatMemory formats all memories as JSON.
func formatMemory(ctx agent.Context, input string) (string, error) {
	longTermMemory.mu.RLock()
	defer longTermMemory.mu.RUnlock()
	b, err := json.MarshalIndent(longTermMemory.data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("event-sourced-memory", flag.ContinueOnError)
	action := fs.String("action", "remember", "action: remember, recall, list")
	key := fs.String("key", "", "memory key")
	value := fs.String("value", "", "memory value")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var a agent.Agent
	var err error
	var input string

	switch *action {
	case "remember":
		a, err = adkrun.Pipeline(
			"EventSourcedMemoryAgent",
			"Two-layer memory with StateDelta for event-sourced state",
			workflow.NewFunctionNode[string, string]("remember", rememberFact, adkrun.NodeConfig()),
		)
		input = *key + "=" + *value
	case "recall":
		a, err = adkrun.Pipeline(
			"EventSourcedMemoryAgent",
			"Recall facts from long-term memory",
			workflow.NewFunctionNode[string, string]("recall", recallFact, adkrun.NodeConfig()),
		)
		input = *key
	case "list":
		a, err = adkrun.Pipeline(
			"EventSourcedMemoryAgent",
			"List all stored memories",
			workflow.NewFunctionNode[string, string]("list", formatMemory, adkrun.NodeConfig()),
		)
		input = ""
	default:
		fmt.Fprintf(os.Stderr, "unknown action: %s\n", *action)
		return 1
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	out, err := adkrun.Run(context.Background(), a, input, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Println(out)
	return 0
}
