// Стартовий шаблон ДЗ 3 — станом на ADK Go v2.2.0 (серпень 2026), перевірте актуальність API.
//
// Граф без LLM — запускається БЕЗ API-ключа:
//
//	go run . console
//	# далі введіть: Мерчант A-114 просить повернення по транзакції txn-2026-07-118845
//
// Чому інструмент уже написаний за вас. У ДЗ 2 ви робили типізований контракт
// на курсах валют (`get_exchange_rate`) — навмисно в іншому домені, щоб урок про
// схему не змішувався з доменом. Тому переносити звідти нічого не треба:
// наскрізний інструмент курсу `open_refund_case` постачається тут готовим, а
// ваша робота сьогодні — граф навколо нього, а не тіло інструмента.
//
// Основано на sources/github/adk-go/examples/workflow/basic/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/adk/v2/workflow"
)

// RefundCaseInput / RefundCaseOutput — наскрізний контракт курсу.
// Рівно два поля на вході: кейс має бути однозначно ідентифікований, і не більше.
type RefundCaseInput struct {
	TransactionID string `json:"transaction_id" jsonschema:"transaction identifier, e.g. txn-2026-07-118845"`
	MerchantID    string `json:"merchant_id" jsonschema:"merchant identifier, e.g. A-114"`
}

type RefundCaseOutput struct {
	CaseID        string `json:"case_id"`
	TransactionID string `json:"transaction_id"`
	MerchantID    string `json:"merchant_id"`
	Status        string `json:"status" jsonschema:"pending or already_open"`
}

// knownMerchants — мерчанти LEDGERWORKS. Невідомий мерчант має давати помилку,
// яка НАЗИВАЄ доступні варіанти: саме з цього тексту модель здогадається, як
// виправити власний виклик.
var knownMerchants = map[string]bool{"A-114": true, "B-207": true}

var (
	mu    sync.Mutex
	cases = map[string]RefundCaseOutput{}
)

// openRefundCase — тіло наскрізного інструмента. Постачається готовим.
//
// Ідемпотентність тут не прикраса: цикл агента не гарантує рівно один виклик,
// а подвійне повернення коштів — це реальні гроші.
func openRefundCase(_ agent.Context, in RefundCaseInput) (RefundCaseOutput, error) {
	txn := strings.ToLower(strings.TrimSpace(in.TransactionID))
	merchant := strings.ToUpper(strings.TrimSpace(in.MerchantID))
	if !transactionID.MatchString(txn) {
		return RefundCaseOutput{}, fmt.Errorf("invalid transaction id: %q", in.TransactionID)
	}
	if !knownMerchants[merchant] {
		return RefundCaseOutput{}, fmt.Errorf("unknown merchant id: %q; available: A-114, B-207", in.MerchantID)
	}

	mu.Lock()
	defer mu.Unlock()

	id := "rc-" + txn + "-" + merchant
	if existing, ok := cases[id]; ok {
		existing.Status = "already_open"
		return existing, nil
	}
	out := RefundCaseOutput{CaseID: id, TransactionID: txn, MerchantID: merchant, Status: "pending"}
	cases[id] = out
	return out, nil
}

var (
	transactionID = regexp.MustCompile(`^txn(-[a-z0-9]+)+$`)
	merchantID    = regexp.MustCompile(`^[a-z]+-[0-9]+$`)
)

// prepare перетворює повідомлення користувача на типізований вхід інструмента.
//
// Наївна версія нижче працює на канонічному формулюванні. TODO(студент):
// зробіть її надійною й покрийте ТАБЛИЧНИМИ тестами на
// agent.NewStrictContextMock — порожній рядок, текст без транзакції, текст без
// мерчанта, інший порядок токенів, інший регістр.
func prepare(_ agent.Context, msg string) (RefundCaseInput, error) {
	var in RefundCaseInput
	for _, token := range strings.FieldsFunc(strings.ToLower(msg), func(r rune) bool {
		return r != '-' && r != '_' && !('a' <= r && r <= 'z') && !('0' <= r && r <= '9')
	}) {
		if in.TransactionID == "" && transactionID.MatchString(token) {
			in.TransactionID = token
			continue
		}
		if in.MerchantID == "" && merchantID.MatchString(token) {
			in.MerchantID = strings.ToUpper(token)
		}
	}
	if in.TransactionID == "" || in.MerchantID == "" {
		return RefundCaseInput{}, fmt.Errorf("очікую ID транзакції та мерчанта, отримано: %q", msg)
	}
	return in, nil
}

// format робить із виходу інструмента людську відповідь.
// Вузол format не має права «додумувати» текст: він форматує рівно те, що
// повернув інструмент. TODO(студент): формат на ваш смак + тести.
func format(_ agent.Context, out RefundCaseOutput) (string, error) {
	return fmt.Sprintf("Кейс %s: транзакція %s, мерчант %s, статус %s",
		out.CaseID, out.TransactionID, out.MerchantID, out.Status), nil
}

func main() {
	ctx := context.Background()

	refundTool, err := functiontool.New(functiontool.Config{
		Name:        "open_refund_case",
		Description: "Opens a refund case for a LEDGERWORKS merchant transaction.",
	}, openRefundCase)
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	// TODO(студент): retry — властивість ОКРЕМОГО вузла. DefaultRetryConfig
	// повторює будь-яку помилку 5 разів із backoff 1s→60s, тож «невідомий
	// мерчант» коштуватиме користувачу пів хвилини очікування на ту саму
	// помилку. Чисті вузли (prepare, format) ретраїв не потребують взагалі.
	nodeConfig := workflow.NodeConfig{
		RetryConfig: workflow.DefaultRetryConfig(),
	}
	prepareNode := workflow.NewFunctionNode("prepare", prepare, nodeConfig)
	refundNode, err := workflow.NewToolNodeTyped[RefundCaseInput, RefundCaseOutput](refundTool, nodeConfig)
	if err != nil {
		log.Fatalf("Failed to create tool node: %v", err)
	}
	formatNode := workflow.NewFunctionNode("format", format, nodeConfig)

	// Статичний потік: Start → prepare → tool → format.
	// TODO(студент, ++): умовна маршрутизація через workflow.StringRoute —
	// див. examples/workflow/routing/string/main.go
	edges := workflow.Chain(workflow.Start, prepareNode, refundNode, formatNode)

	wa, err := workflowagent.New(workflowagent.Config{
		Name:        "first_graph_agent",
		Description: "Статичний граф: підготовка → інструмент → форматування.",
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
