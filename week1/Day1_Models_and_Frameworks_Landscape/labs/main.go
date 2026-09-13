// Стартовий шаблон ДЗ 1 — звірено з google.golang.org/adk/v2 v2.4.0 (Go 1.27),
// станом на 08/2026. Перевірте актуальність API перед записом.
//
// Запуск:
//
//	export GOOGLE_API_KEY=...
//	go run . console
//
// Без аргументів launcher за замовчуванням бере console (перший sublauncher
// у full.NewLauncher), а console читає os.Stdin — тому запит можна подати
// пайпом, без інтерактиву. Зручно для міні-бенчмарку з Завдання 5:
//
//	echo "Яка погода у Львові?" | MODEL=gemini-3.7-flash go run .
//
// Основано на sources/adk-go/examples/quickstart/main.go
package main

import (
	"context"
	"log"
	"os"
	"strings"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/geminitool"
)

// defaultModel — свідомо ПЛАВАЮЧИЙ alias, а не датований id на кшталт
// "gemini-3.7-flash": стартер, який пінить конкретну версію моделі, ламається
// для всіх студентів у той день, коли провайдер її виводить з обігу.
const defaultModel = "gemini-flash-latest"

// resolveModelName повертає ім'я моделі зі змінної MODEL або дефолт.
//
// Винесено в окрему функцію рівно заради тесту: на цьому інваріанті тримається
// Завдання 5 (прогін кількох моделей зміною ОДНІЄЇ змінної). Якби MODEL
// мовчки ігнорувалась, бенчмарк порівнював би модель саму з собою й видав
// цілком правдоподібну таблицю — найгірший різновид помилки, бо нічого не падає.
func resolveModelName(env string) string {
	if strings.TrimSpace(env) == "" {
		return defaultModel
	}
	return env
}

func main() {
	ctx := context.Background()

	// TODO(студент): для міні-бенчмарку запустіть агента по черзі на 2–3 моделях
	// («станом на серпень 2026»: gemini-flash-latest, gemini-3.7-flash, ...)
	// і зафіксуйте latency/якість/приблизну вартість у таблиці в README.
	// Поруч додайте фундаментальну карту: LLM, prompt, context window, tool,
	// agent, RAG і MCP — що означає кожен термін саме в цій лабі.
	//
	// Модель береться зі змінної MODEL саме для цього: прогнати той самий
	// запит на кількох моделях має бути зміною однієї змінної, а не правкою
	// коду — інакше ви порівнюєте дві різні програми, а не дві моделі.
	modelName := resolveModelName(os.Getenv("MODEL"))
	model, err := gemini.NewModel(ctx, modelName, &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "weekend_planner",
		Model:       model,
		Description: "Агент-планувальник вихідних у моєму місті.",
		// TODO(студент): напишіть власну інструкцію:
		//  1) ваше місто та формат відповіді (ранок/день/вечір, бюджет);
		//  2) агент МУСИТЬ використовувати пошук для актуальних подій/погоди;
		//  3) агент МУСИТЬ відмовлятися відповідати поза своїм доменом.
		//  4) у README перевірте context-stress сценарій: коли інформація є
		//     в prompt, а коли агент має чесно сказати, що потрібен пошук/RAG.
		Instruction: "Ти — планувальник вихідних. ЗАМІНИ ЦЕЙ ТЕКСТ власною інструкцією.",
		Tools: []tool.Tool{
			geminitool.GoogleSearch{},
		},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(a),
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
