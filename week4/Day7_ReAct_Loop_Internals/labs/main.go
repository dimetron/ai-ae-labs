// Стартовий шаблон ДЗ 7 — станом на ADK Go v2.0.0 (липень 2026), перевірте актуальність API.
//
// Графова версія ReAct-циклу (без LLM — запускається БЕЗ API-ключа;
// LLM-крок додасте самі). Безфреймворкову версію пишіть окремим файлом.
//
//	go run . console
//
// Основано на sources/github/adk-go/examples/workflow/dynamic/basic/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

// maxIterations — жорсткий ліміт циклу. Після вичерпання — явна помилка.
const maxIterations = 5

// StepResult — результат одного кроку ReAct.
type StepResult struct {
	Thought     string `json:"thought"`
	Action      string `json:"action"` // назва інструмента або "final"
	Observation string `json:"observation"`
	Final       bool   `json:"final"`
}

func main() {
	ctx := context.Background()

	// Один крок циклу: Thought → Action → Observation.
	// TODO(студент): реалізуйте крок — виберіть інструмент (≥2 на вибір
	// агента!), викличте його, поверніть Observation. Спершу можна без LLM
	// (правила/евристика), потім підключіть модель.
	stepNode := workflow.NewFunctionNode("react_step",
		func(_ agent.Context, input string) (StepResult, error) {
			return StepResult{
				Thought:     "TODO: міркування",
				Action:      "final",
				Observation: "TODO: спостереження для входу " + input,
				Final:       true, // заглушка завершується одразу
			}, nil
		},
		workflow.NodeConfig{},
	)

	// Цикл у графі: динамічний вузол викликає stepNode через RunNode,
	// доки не Final або не вичерпано maxIterations.
	reactLoop := workflow.NewDynamicNode[string, string]("react_loop",
		func(ctx agent.Context, input string, _ func(*session.Event) error) (string, error) {
			current := input
			for i := 0; i < maxIterations; i++ {
				step, err := workflow.RunNode[StepResult](ctx, stepNode, current)
				if err != nil {
					return "", fmt.Errorf("ітерація %d: %w", i+1, err)
				}
				// TODO(студент, ++): детекція зациклення — повторний Action
				// з ідентичними аргументами має переривати цикл достроково.
				if step.Final {
					return step.Observation, nil
				}
				current = step.Observation
			}
			return "", fmt.Errorf("вичерпано ліміт %d ітерацій без фінальної відповіді", maxIterations)
		},
		workflow.NodeConfig{},
	)

	wa, err := workflowagent.New(workflowagent.Config{
		Name:        "react_graph_agent",
		Description: "ReAct-цикл як динамічний вузол workflow-графа з лімітом ітерацій.",
		Edges:       workflow.Chain(workflow.Start, reactLoop),
	})
	if err != nil {
		log.Fatalf("workflowagent.New: %v", err)
	}

	l := full.NewLauncher()
	if err := l.Execute(ctx, &launcher.Config{
		AgentLoader: agent.NewSingleLoader(wa),
	}, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
