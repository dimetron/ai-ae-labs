// Стартовий шаблон ДЗ 11 — станом на ADK Go v2.2.0 (08/2026), перевірте актуальність API.
//
// Скелет без LLM — запускається БЕЗ API-ключа; generate/критиків на LLM
// підключите самі (workflow.NewAgentNode або виклик моделі у вузлі).
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

// maxAttempts — бюджет циклу виправлень.
const maxAttempts = 4

// Draft — робота maker'а разом із зауваженнями критиків.
type Draft struct {
	Task     string   `json:"task"`
	Code     string   `json:"code"`
	Feedback []string `json:"feedback,omitempty"`
}

// Verdict — типізований вердикт критика.
// TODO(студент): валідуйте структуру вердикта LLM-критика —
// несумісна відповідь відсікається, а не «якось інтерпретується».
type Verdict struct {
	Critic   string   `json:"critic"`
	Passed   bool     `json:"passed"`
	Comments []string `json:"comments,omitempty"`
}

func main() {
	ctx := context.Background()

	// Maker: пише/виправляє код за завданням та зауваженнями.
	// TODO(студент): замініть заглушку на виклик LLM (див.
	// examples/workflow/dynamic/llm — AgentNode усередині RunNode).
	generate := workflow.NewFunctionNode("generate",
		func(_ agent.Context, d Draft) (Draft, error) {
			d.Code = "// TODO: код, згенерований maker'ом для задачі: " + d.Task
			return d, nil
		},
		workflow.NodeConfig{},
	)

	// Критик 1: коректність — ОБ'ЄКТИВНА перевірка.
	// TODO(студент): запишіть d.Code у тимчасовий файл і запустіть
	// go build / go test (os/exec) з таймаутом; Passed=true лише якщо зелено.
	criticCorrectness := workflow.NewFunctionNode("critic_correctness",
		func(_ agent.Context, d Draft) (Verdict, error) {
			return Verdict{Critic: "correctness", Passed: false, Comments: []string{"TODO: перевірка компіляції/тестів"}}, nil
		},
		workflow.NodeConfig{},
	)

	// TODO(студент): критик 2 (стиль/ідіоматичність) і критик 3
	// (продуктивність) — LLM-критики з різними промптами.

	// Цикл maker/checker: generate → критики → feedback → generate…
	loop := workflow.NewDynamicNode[string, string]("multi_critic_loop",
		func(ctx agent.Context, task string, _ func(*session.Event) error) (string, error) {
			draft := Draft{Task: task}
			for attempt := 1; attempt <= maxAttempts; attempt++ {
				d, err := workflow.RunNode[Draft](ctx, generate, draft)
				if err != nil {
					return "", fmt.Errorf("generate (спроба %d): %w", attempt, err)
				}
				verdict, err := workflow.RunNode[Verdict](ctx, criticCorrectness, d)
				if err != nil {
					return "", fmt.Errorf("critic (спроба %d): %w", attempt, err)
				}
				// TODO(студент): запустіть УСІХ критиків, зберіть усі вердикти
				// (++: конкурентно), агрегуйте Passed && Passed && Passed.
				if verdict.Passed {
					return d.Code, nil
				}
				draft = d
				draft.Feedback = verdict.Comments
			}
			return "", fmt.Errorf("бюджет вичерпано: %d спроб без проходження всіх критиків", maxAttempts)
		},
		workflow.NodeConfig{},
	)

	wa, err := workflowagent.New(workflowagent.Config{
		Name:        "multi_critic_coding_agent",
		Description: "Динамічний цикл maker/checker: генерація коду до проходження критиків.",
		Edges:       workflow.Chain(workflow.Start, loop),
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
