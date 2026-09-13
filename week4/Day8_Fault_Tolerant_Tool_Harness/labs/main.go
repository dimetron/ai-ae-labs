// Стартовий шаблон ДЗ 8 — станом на ADK Go v2.4.0 (вересень 2026), перевірте актуальність API.
//
// Запуск:
//
//	export GOOGLE_API_KEY=...
//	go run . console
//
// Патерн functiontool — із sources/github/adk-go/examples/multiagent/collaboration/main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

// Класи помилок харнеса.
var (
	ErrTransient = errors.New("транзієнтний збій")  // таймаут, 5xx — лікується ретраєм
	ErrSemantic  = errors.New("семантична помилка") // невалідні аргументи — лікується моделлю
)

// RefundCaseInput / RefundCaseOutput — контракт «ненадійного» інструмента.
//
// Це наскрізний інструмент курсу (`courses/legend.md`): рівно два поля, якими
// кейс повернення однозначно ідентифікується.
type RefundCaseInput struct {
	TransactionID string `json:"transaction_id" jsonschema:"Ідентифікатор транзакції, напр. txn-2026-07-118845"`
	MerchantID    string `json:"merchant_id" jsonschema:"Ідентифікатор мерчанта, напр. A-114"`
}

type RefundCaseOutput struct {
	Status string `json:"status"`
	CaseID string `json:"case_id,omitempty"`
	// Observation — ТЕКСТ помилки для моделі. Семантична помилка не валить
	// ран: вона повертається сюди, і модель виправляє свій наступний хід.
	Observation string `json:"observation,omitempty"`
}

// flakyCheck імітує ненадійну залежність (еквайринг, який блимає).
// TODO(студент): додайте ще ≥2 режими збою (таймаут через context,
// недоступна залежність) і зробіть імовірність керованою через env/прапорець.
//
// Рядок "undefined" перевіряється явно: саме він о 03:00 доїхав у прод-БД —
// він валідний рядок і проходить будь-яку перевірку «поле непорожнє».
func flakyCheck(_ agent.Context, in RefundCaseInput) (RefundCaseOutput, error) {
	if in.MerchantID == "" || in.MerchantID == "undefined" {
		return RefundCaseOutput{}, fmt.Errorf("%w: merchant_id не витягнуто із запиту", ErrSemantic)
	}
	if in.TransactionID == "" || in.TransactionID == "undefined" {
		return RefundCaseOutput{}, fmt.Errorf("%w: transaction_id не витягнуто із запиту", ErrSemantic)
	}
	if rand.Float64() < 0.4 { //nolint:gosec // навчальна імітація збою
		return RefundCaseOutput{}, fmt.Errorf("%w: імітований таймаут еквайрингу", ErrTransient)
	}
	return RefundCaseOutput{Status: "pending", CaseID: "rc-" + in.TransactionID + "-" + in.MerchantID}, nil
}

// withHarness — серце ДЗ: обгортка, що класифікує помилки.
// TODO(студент):
//  1. errors.Is(err, ErrTransient) → повторити виклик (до N спроб, backoff);
//  2. errors.Is(err, ErrSemantic) → НЕ повертати error, а покласти текст
//     у RefundCaseOutput.Observation — модель побачить і самовиправиться;
//  3. невідомі помилки → нагору як є; ліміт самокорекцій — з явним фейлом.
func withHarness(fn func(agent.Context, RefundCaseInput) (RefundCaseOutput, error)) func(agent.Context, RefundCaseInput) (RefundCaseOutput, error) {
	return func(ctx agent.Context, in RefundCaseInput) (RefundCaseOutput, error) {
		out, err := fn(ctx, in)
		if err != nil {
			return RefundCaseOutput{}, err // заглушка: поки що прокидаємо все
		}
		return out, nil
	}
}

func main() {
	ctx := context.Background()

	model, err := gemini.NewModel(ctx, "gemini-flash-latest", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	refundTool, err := functiontool.New(functiontool.Config{
		Name:        "open_refund_case",
		Description: "Відкриває кейс повернення по транзакції мерчанта. Може повертати observation про помилку — прочитай його і виправ виклик.",
	}, withHarness(flakyCheck))
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "resilient_agent",
		Model:       model,
		Description: "Агент зі стійкими до збоїв інструментами.",
		// TODO(студент): інструкція — якщо інструмент повернув observation
		// про помилку, проаналізуй її, виправ аргументи і спробуй ще раз
		// (не більше N разів).
		Instruction: "ЗАМІНИ: інструкція самовідновлення після помилок інструментів.",
		Tools: []tool.Tool{
			refundTool,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, &launcher.Config{
		AgentLoader: agent.NewSingleLoader(a),
	}, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
