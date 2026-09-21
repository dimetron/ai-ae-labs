// Package main реалізує Google ADK Go v2 Model Expert Agent із підтримкою multi-provider LLM
// через бібліотеку pimodels (github.com/dimetron/pi-go/pimodels) та вбудованим каталогом models.dev.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"

	"github.com/dimetron/ai-eng-course/demo/adk-quickstart/internal/tools"
	"github.com/dimetron/ai-eng-course/demo/adk-quickstart/internal/utils"
	"github.com/dimetron/pi-go/pimodels"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
)

// //go:embed компілює зовнішні файли (промпт та правила AGENTS.md) безпосередньо у бінарник.
// Завдяки цьому промпти версіонуються у Markdown, а бінарник залишається повністю автономним.
//
//go:embed resources/system_prompt.md
var systemPrompt string

//go:embed AGENTS.md
var agentsDoc string

const (
	defaultModel = "gemini-3.8-flash"
	agentName    = "model_expert_agent"
	agentDesc    = "Експертний агент з вибору, аналізу характеристик та рекомендацій LLM-моделей (каталог models.dev)."
)

// NewAgent створює та налаштовує екземпляр LLM-агента ADK за допомогою pimodels.
func NewAgent(ctx context.Context, modelName string, opts ...pimodels.Option) (agent.Agent, error) {
	if modelName == "" {
		modelName = defaultModel
	}

	info, err := pimodels.Resolve(modelName, opts...)
	if err != nil {
		return nil, fmt.Errorf("помилка визначення моделі %q: %w", modelName, err)
	}

	m, err := pimodels.New(ctx, modelName, opts...)
	if err != nil {
		envVar := pimodels.APIKeyEnvVar(info.Provider)
		return nil, fmt.Errorf("помилка створення моделі %s (%s): %w (перевірте ключ у %s)", modelName, info.Provider, err, envVar)
	}

	// Поєднуємо системний промпт та AGENTS.md інструкції
	fullInstruction := systemPrompt + "\n\n---\n## Контекст репозиторію та правила (AGENTS.md)\n" + agentsDoc

	return llmagent.New(llmagent.Config{
		Name:        agentName,
		Model:       m,
		Description: agentDesc,
		Instruction: fullInstruction,
		Tools:       tools.All(),
	})
}

func main() {
	utils.LoadDotEnv()
	ctx := context.Background()

	modelName := os.Getenv("MODEL")
	if modelName == "" {
		modelName = os.Getenv("GEMINI_MODEL")
	}

	var opts []pimodels.Option
	if k := os.Getenv("AGENTGATEWAY_API_KEY"); k != "" {
		opts = append(opts, pimodels.WithAPIKey(k))
	}
	if u := os.Getenv("BASE_URL"); u != "" {
		opts = append(opts, pimodels.WithBaseURL(u))
	}

	a, err := NewAgent(ctx, modelName, opts...)
	if err != nil {
		log.Fatalf("Помилка ініціалізації: %v", err)
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, &launcher.Config{AgentLoader: agent.NewSingleLoader(a)}, os.Args[1:]); err != nil {
		log.Fatalf("Помилка виконання: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
