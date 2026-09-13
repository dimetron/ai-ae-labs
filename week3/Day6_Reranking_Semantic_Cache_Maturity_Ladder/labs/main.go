// Стартовий шаблон ДЗ 6 — ADK Go v2.4.0, Go 1.27 (пін модуля лаб —
// courses/AI_Agents_Engineering/lectures/go.mod, звірено 27.08.2026). Перевірте
// актуальність API перед стартом: лінія релізів рухається щомісяця.
//
// Граф без LLM — запускається БЕЗ API-ключа (re-rank через LLM додасте самі):
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

// Candidate — кандидат retrieval з оцінкою.
type Candidate struct {
	ChunkID string  `json:"chunk_id"`
	Text    string  `json:"text"`
	Score   float64 `json:"score"`
}

// SearchResult — запит + кандидати, тече між вузлами графа.
type SearchResult struct {
	Query      string      `json:"query"`
	Candidates []Candidate `json:"candidates"`
	CacheHit   bool        `json:"cache_hit"`
}

// TODO(студент): семантичний кеш. Почніть з map[нормалізований запит]відповідь,
// потім перейдіть на схожість (ембединги або власна евристика) —
// перефразований запит має влучати в кеш.
var cache = map[string]string{}

// search шукає топ-K кандидатів по чанках із ДЗ 5.
// TODO(студент): завантажте чанки (JSON із ДЗ 5), реалізуйте пошук —
// векторний (ембединги) або BM25/keyword; заповніть Score.
func search(_ agent.Context, query string) (SearchResult, error) {
	if query == "" {
		return SearchResult{}, fmt.Errorf("порожній запит")
	}
	if answer, ok := cache[query]; ok {
		return SearchResult{Query: query, CacheHit: true, Candidates: []Candidate{{ChunkID: "cache", Text: answer, Score: 1}}}, nil
	}
	return SearchResult{Query: query, Candidates: []Candidate{
		{ChunkID: "todo-1", Text: "TODO: реальні кандидати з вашого корпусу", Score: 0.5},
	}}, nil
}

// rerank переранжовує кандидатів до топ-N.
// TODO(студент): LLM-судія (окремий виклик моделі: «оціни релевантність 0–10»)
// або cross-encoder-евристика; збережіть порядок «до/після» для README.
func rerank(_ agent.Context, in SearchResult) (SearchResult, error) {
	return in, nil // заглушка: порядок не змінюється
}

// answer формує фінальну відповідь із топ-кандидатів і пише в кеш.
// TODO(студент): складіть відповідь із цитатами chunk_id; запишіть у кеш.
func answer(_ agent.Context, in SearchResult) (string, error) {
	if len(in.Candidates) == 0 {
		return "Нічого не знайдено.", nil
	}
	result := fmt.Sprintf("[cache_hit=%v] Топ-результат (%s): %s", in.CacheHit, in.Candidates[0].ChunkID, in.Candidates[0].Text)
	cache[in.Query] = result
	return result, nil
}

func main() {
	ctx := context.Background()

	nodeConfig := workflow.NodeConfig{
		RetryConfig: workflow.DefaultRetryConfig(),
	}
	searchNode := workflow.NewFunctionNode("search", search, nodeConfig)
	rerankNode := workflow.NewFunctionNode("rerank", rerank, nodeConfig)
	answerNode := workflow.NewFunctionNode("answer", answer, nodeConfig)

	edges := workflow.Chain(workflow.Start, searchNode, rerankNode, answerNode)

	wa, err := workflowagent.New(workflowagent.Config{
		Name:        "high_precision_rag",
		Description: "Retrieval-конвеєр: пошук → re-rank → відповідь + семантичний кеш.",
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
