// Живий прогін evalset-у на справжній моделі (gemini-3.7-flash за
// замовчуванням; каталог models.dev через pimodels).
//
// Використання:
//
//	export GEMINI_API_KEY=...
//	go run ./cmd/liveeval -set evalset/refund.evalset.json [-model gemini-3.7-flash]
//	                       [-out artifacts] [-m pattern]
//
// Ціна: кожен кейс — 1..N LLM-викликів; 4 кейси ≈ копійки на Flash, але
// завжди запускайте спершу заглушковий `go test .`, а живий прогін — перед
// комітом зміни промпта/схеми.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/dimetron/ai-eng-course/demo/7_adk-go-evals/adkeval"
	"github.com/dimetron/ai-eng-course/demo/7_adk-go-evals/internal/refundagent"
)

func main() {
	setPath := flag.String("set", "evalset/refund.evalset.json", "шлях до evalset JSON")
	modelName := flag.String("model", "", "модель (дефолт з pimodels: gemini-3.7-flash)")
	outDir := flag.String("out", "artifacts", "тека для results_*.json")
	flag.Parse()

	ctx := context.Background()
	m, err := adkeval.LiveModel(ctx, *modelName)
	if err != nil {
		log.Fatal(err)
	}
	a, err := refundagent.NewAgent("refund_eval", m)
	if err != nil {
		log.Fatal(err)
	}

	es, err := adkeval.LoadEvalSet(*setPath)
	if err != nil {
		log.Fatal(err)
	}

	rep, err := adkeval.RunEvalSet(ctx, a, "refund_eval", es)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(rep.SummaryText())
	if err := rep.SaveJSON(*outDir); err != nil {
		fmt.Fprintf(os.Stderr, "не вдалося зберегти звіт: %v\n", err)
	}
	if rep.Failed > 0 {
		os.Exit(1)
	}
}
