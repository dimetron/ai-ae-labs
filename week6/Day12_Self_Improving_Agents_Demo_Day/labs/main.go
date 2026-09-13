// Стартовий шаблон ДЗ 12 (фінал) — станом на ADK Go v2.4.0 (вересень 2026), перевірте актуальність API.
//
// Каркас зшивки Personal Research Agent. Скелет без LLM — запускається
// БЕЗ API-ключа; ваші вузли з ДЗ 5–11 підключаються на місця TODO.
//
//	go run . console
//
// Основано на sources/github/adk-go/examples/workflow/complex/main.go (fan-out/JoinNode)
// та examples/workflow/dynamic/basic/main.go (цикл).
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
	"google.golang.org/adk/v2/workflow"
)

// selfImproveBudget — принцип (б) autoresearch: фіксований бюджет ітерацій.
const selfImproveBudget = 5

// loadStrategy — принцип (а)+(г): агент редагує РІВНО ОДИН файл, і цей файл
// (дослідницька стратегія) — людиночитний артефакт, який редагуєте і ви.
func loadStrategy() (string, error) {
	data, err := os.ReadFile("strategy.md")
	if err != nil {
		return "", fmt.Errorf("створіть strategy.md поряд із бінарником: %w", err)
	}
	return string(data), nil
}

// logExperiment — принцип (д): повний лог кожного експерименту.
// TODO(студент): пишіть в experiments.log: стара/нова стратегія,
// метрика до/після, вердикт keep/discard, час.
func logExperiment(entry string) error {
	f, err := os.OpenFile("experiments.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, entry)
	return err
}

func main() {
	ctx := context.Background()

	strategy, err := loadStrategy()
	if err != nil {
		log.Fatalf("loadStrategy: %v", err)
	}
	log.Printf("Стратегію завантажено (%d байт)", len(strategy))

	// --- Гілки паралельного збору (ДЗ 5–6 + ДЗ 10) ---
	// TODO(студент): замініть заглушки на РЕАЛЬНІ вузли:
	//   webSearch — пошуковець джерел (single_turn агент або AgentNode);
	//   kbSearch  — retrieval-конвеєр із ДЗ 6 по базі знань із ДЗ 5.
	webSearch := workflow.NewFunctionNode("web_search",
		func(_ agent.Context, query string) (string, error) {
			return "TODO: результати веб-пошуку для: " + query, nil
		}, workflow.NodeConfig{})
	kbSearch := workflow.NewFunctionNode("kb_search",
		func(_ agent.Context, query string) (string, error) {
			return "TODO: результати з бази знань для: " + query, nil
		}, workflow.NodeConfig{})

	// Fan-in барʼєр: чекає обидві гілки, віддає map[імʼя вузла]результат.
	gather := workflow.NewJoinNode("gather")

	// Чернетка звіту з зібраного.
	// TODO(студент): LLM-синтез (AgentNode, як SynthesisAgent у complex) +
	// далі цикл критиків із ДЗ 11 (DynamicNode) + HITL-пауза перед фіналом
	// (RequestInput, як у hitl_simple) + самополіпшення strategy.md у межах
	// selfImproveBudget з єдиною метрикою по вашому eval-набору.
	draft := workflow.NewFunctionNode("draft_report",
		func(_ agent.Context, gathered map[string]any) (string, error) {
			if err := logExperiment("draft: зібрано " + fmt.Sprint(len(gathered)) + " гілок"); err != nil {
				return "", err
			}
			return fmt.Sprintf("Чернетка звіту з %d джерел. TODO: критики → HITL → фінал (бюджет самополіпшення: %d).",
				len(gathered), selfImproveBudget), nil
		}, workflow.NodeConfig{})

	// Граф: START ─┬→ web_search ─┐
	//              └→ kb_search ──┴→ gather → draft_report
	eb := workflow.NewEdgeBuilder()
	eb.AddFanOut(workflow.Start, webSearch, kbSearch)
	eb.AddFanIn(gather, webSearch, kbSearch)
	eb.Add(gather, draft)

	wa, err := workflowagent.New(workflowagent.Config{
		Name:        "personal_research_agent",
		Description: "Фінальна зшивка: паралельний збір → звіт → критики → HITL → самополіпшення.",
		Edges:       eb.Build(),
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
