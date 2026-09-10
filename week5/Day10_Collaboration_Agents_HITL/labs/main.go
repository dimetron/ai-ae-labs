// Стартовий шаблон ДЗ 10 — станом на ADK Go v2.0.0 (липень 2026), перевірте актуальність API.
//
// Запуск:
//
//	export GOOGLE_API_KEY=...
//	go run . console
//
// Основано на sources/github/adk-go/examples/multiagent/collaboration/main.go
// (усі три режими LLM-агентів: chat / single_turn / task).
package main

import (
	"context"
	"log"
	"os"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/geminitool"
)

func main() {
	ctx := context.Background()

	model, err := gemini.NewModel(ctx, "gemini-flash-latest", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// Спеціаліст 1: автономний пошуковець джерел (single_turn) —
	// відпрацьовує ланцюжок інструментів і сам повертається до координатора.
	sourceHunter, err := llmagent.New(llmagent.Config{
		Name:        "source_hunter",
		Model:       model,
		Description: "Шукає джерела за темою і повертає короткий список із посиланнями.",
		Mode:        llmagent.ModeSingleTurn,
		Tools: []tool.Tool{
			geminitool.GoogleSearch{},
		},
		// TODO(студент): інструкція — знайти 3–5 джерел, одним рядком кожне.
		Instruction: "ЗАМІНИ: інструкція пошуковця джерел.",
	})
	if err != nil {
		log.Fatalf("Failed to create source_hunter: %v", err)
	}

	// TODO(студент): спеціаліст 2 у режимі task — збирач структурованих
	// даних: Mode: llmagent.ModeTask + InputSchema/OutputSchema через
	// *genai.Schema (скопіюйте патерн flight_booker із
	// examples/multiagent/collaboration/main.go) + власні functiontool.

	// TODO(студент): паралельний етап — fan-out двох спеціалістів через
	// workflowagent + eb.AddFanOut/AddFanIn із workflow.NewJoinNode
	// (зразок: examples/workflow/complex/main.go).

	// TODO(студент): HITL-пауза перед ризиковою дією — вузол із
	// workflow.NewRequestInputEvent + workflow.ErrNodeInterrupted
	// (зразок: examples/workflow/hitl_simple/main.go).

	coordinator, err := llmagent.New(llmagent.Config{
		Name:        "research_coordinator",
		Model:       model,
		Description: "Координатор дослідницької команди: делегує задачі спеціалістам.",
		SubAgents: []agent.Agent{
			sourceHunter,
			// TODO(студент): + збирач даних (task), + інші спеціалісти.
		},
		// TODO(студент): інструкція координатора — кому що делегувати,
		// самому інструменти спеціалістів не викликати.
		Instruction: "ЗАМІНИ: інструкція координатора.",
	})
	if err != nil {
		log.Fatalf("Failed to create coordinator: %v", err)
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, &launcher.Config{
		AgentLoader: agent.NewSingleLoader(coordinator),
	}, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
