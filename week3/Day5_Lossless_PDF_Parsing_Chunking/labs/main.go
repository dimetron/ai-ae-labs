// Стартовий шаблон ДЗ 5 — ADK Go v2.2.0, Go 1.27 (пін модуля лаб —
// courses/AI_Agents_Engineering/lectures/go.mod, звірено 27.08.2026). Перевірте
// актуальність API перед стартом: лінія релізів рухається щомісяця.
//
// Граф без LLM — запускається БЕЗ API-ключа. Введіть шлях до документа:
//
//	go run . console
//	go test ./...   // скелет табличних тестів — main_test.go
//
// Основано на sources/github/adk-go/examples/workflow/basic/main.go
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

// Chunk — одиниця майбутньої бази знань Research Agent.
type Chunk struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id,omitempty"` // порожній для батьківських чанків
	Level    int    `json:"level"`               // 0 = розділ, 1 = підрозділ, 2 = абзац/таблиця
	Kind     string `json:"kind"`                // "heading" | "text" | "table" | "list"
	Text     string `json:"text"`
}

// load читає документ. Вхід — шлях до файлу з повідомлення користувача.
func load(_ agent.Context, path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("не вдалося прочитати %q: %w", path, err)
	}
	// TODO(студент): для PDF — витягніть текстовий шар (бібліотека на вибір)
	// або конвертуйте у Markdown заздалегідь; таблиці мають лишитися таблицями.
	return string(data), nil
}

// chunk нарізає документ ієрархічно.
// TODO(студент): реалізуйте Parent-Child chunking:
//  1) розбийте за заголовками (#, ##) на батьківські чанки;
//  2) усередині — на дочірні (абзаци, списки);
//  3) таблиці (| ... |) — АТОМАРНІ чанки Kind="table", їх різати не можна;
//  4) заповніть ParentID у дочірніх чанків.
func chunk(_ agent.Context, content string) ([]Chunk, error) {
	if content == "" {
		return nil, fmt.Errorf("порожній документ")
	}
	return []Chunk{{ID: "c1", Level: 0, Kind: "text", Text: content}}, nil // заглушка: один гігантський чанк
}

// report рахує статистику нарізки.
// TODO(студент): кількість чанків за рівнями, середній/максимальний розмір,
// скільки таблиць збережено цілими.
func report(_ agent.Context, chunks []Chunk) (string, error) {
	return fmt.Sprintf("Отримано %d чанків. TODO: статистика за рівнями і таблицями.", len(chunks)), nil
}

func main() {
	ctx := context.Background()

	nodeConfig := workflow.NodeConfig{
		RetryConfig: workflow.DefaultRetryConfig(),
	}
	loadNode := workflow.NewFunctionNode("load", load, nodeConfig)
	chunkNode := workflow.NewFunctionNode("chunk", chunk, nodeConfig)
	reportNode := workflow.NewFunctionNode("report", report, nodeConfig)

	edges := workflow.Chain(workflow.Start, loadNode, chunkNode, reportNode)

	wa, err := workflowagent.New(workflowagent.Config{
		Name:        "hierarchical_chunker",
		Description: "Ingestion-конвеєр: документ → ієрархічні чанки → статистика.",
		Edges:       edges,
	})
	if err != nil {
		log.Fatalf("failed to create workflow: %v", err)
	}

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(wa),
	}
	l := full.NewLauncher()
	if err = l.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
