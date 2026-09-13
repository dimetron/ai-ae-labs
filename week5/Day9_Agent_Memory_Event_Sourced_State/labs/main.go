// Стартовий шаблон ДЗ 9 — станом на ADK Go v2.4.0 (вересень 2026), перевірте актуальність API.
//
// Запуск:
//
//	export GOOGLE_API_KEY=...
//	go run . console
//
// Повний сценарій із двома сесіями («амнезія» vs пам'ять) —
// див. sources/github/adk-go/examples/tools/loadmemory/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/adk/v2/tool/loadmemorytool"
	"google.golang.org/adk/v2/tool/preloadmemorytool"
)

// FactInput — факт про користувача для довгострокової пам'яті.
type FactInput struct {
	Key   string `json:"key" jsonschema:"Коротка назва факту, напр. recommended_path"`
	Value string `json:"value" jsonschema:"Значення факту"`
}

type FactOutput struct {
	Saved bool `json:"saved"`
}

// rememberFact пише факт у стан сесії. Оновлення пройде через event log
// як StateDelta — знайдіть цю подію і додайте фрагмент у README.
func rememberFact(ctx agent.Context, in FactInput) (FactOutput, error) {
	if in.Key == "" {
		return FactOutput{}, fmt.Errorf("key обов'язковий")
	}
	// TODO(студент): перевірте у документації ADK префікси ключів стану
	// (user:/app:/temp:) — факт про КОРИСТУВАЧА має переживати сесію.
	if err := ctx.Session().State().Set("user:"+in.Key, in.Value); err != nil {
		return FactOutput{}, fmt.Errorf("не вдалося зберегти факт: %w", err)
	}
	return FactOutput{Saved: true}, nil
}

func main() {
	ctx := context.Background()

	model, err := gemini.NewModel(ctx, "gemini-flash-latest", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	rememberTool, err := functiontool.New(functiontool.Config{
		Name:        "remember_fact",
		Description: "Зберігає важливий факт про користувача у довгострокову пам'ять.",
	}, rememberFact)
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "memory_agent",
		Model:       model,
		Description: "Агент із коротко- та довгостроковою пам'яттю.",
		// TODO(студент): інструкція — коли викликати remember_fact
		// (нова стала інформація про користувача), коли load_memory
		// (питання про минулі розмови).
		Instruction: "ЗАМІНИ: інструкція роботи з пам'яттю.",
		Tools: []tool.Tool{
			preloadmemorytool.New(),
			loadmemorytool.New(),
			rememberTool,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// TODO(студент): демо «з амнезією і без» — програмний сценарій із двома
	// сесіями: session.InMemoryService() + memory.InMemoryService() +
	// AddSessionToMemory(першу сесію в пам'ять) — скопіюйте каркас із
	// examples/tools/loadmemory/main.go і адаптуйте під свій домен.
	l := full.NewLauncher()
	if err = l.Execute(ctx, &launcher.Config{
		AgentLoader: agent.NewSingleLoader(a),
	}, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
