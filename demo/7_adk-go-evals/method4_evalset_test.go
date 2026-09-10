// Method 4 — evalset runner: оцінка всього набору кейсів одним прогоном,
// з порогами, verdict-перевіркою і машино-читаним звітом (results_*.json).
//
// Тестовий виклик використовує ScriptedModel, тому CI не платить токенів.
// Живий режим — `go run ./cmd/liveeval` у корені демо.
package adkeval_test

import (
	"context"
	"testing"

	"github.com/dimetron/ai-eng-course/demo/7_adk-go-evals/adkeval"
	"github.com/dimetron/ai-eng-course/demo/7_adk-go-evals/internal/refundagent"

	"google.golang.org/genai"
)

func TestEvalSetGraded(t *testing.T) {
	ctx := context.Background()

	es, err := adkeval.LoadEvalSet("evalset/refund.evalset.json")
	if err != nil {
		t.Fatal(err)
	}

	// Сценарій заглушки: порядок відповідей на кваліфіковані запити.
	m := &adkeval.ScriptedModel{Script: []adkeval.QScriptResponse{
		// refund_happy_path
		{FunctionCall: &genai.FunctionCall{Name: "open_refund_case", Args: map[string]any{
			"transaction_id": "txn-2026-07-118845", "merchant_id": "A-114"}},
		},
		{Text: `Case rc-txn-2026-07-118845-A-114 opened with status pending.`},
		// off_domain_merchant_refused
		{FunctionCall: &genai.FunctionCall{Name: "open_refund_case", Args: map[string]any{
			"transaction_id": "txn-bad", "merchant_id": "Z-999"}},
		},
		{Text: `POLICY_BLOCKED`},
		// missing_ids_asks_human
		{Text: `Could you share the transaction id and merchant id? ASK`},
		// second_merchant_in_registry
		{FunctionCall: &genai.FunctionCall{Name: "open_refund_case", Args: map[string]any{
			"transaction_id": "txn-2026-08-000001", "merchant_id": "A-207"}},
		},
		{Text: `Case rc-txn-2026-08-000001-A-207 opened with status pending.`},
	}}

	a, err := refundagent.NewAgent("refund_eval", m)
	if err != nil {
		t.Fatal(err)
	}

	rep, err := adkeval.RunEvalSet(ctx, a, "refund_eval", es)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("\n" + rep.SummaryText())

	if rep.Failed > 0 {
		for _, c := range rep.Cases {
			if !c.Passed {
				t.Errorf("кейс %s провалився: %s", c.EvalID, c.FailureCause)
			}
		}
	}

	if t.Failed() {
		return
	}
	// Зберегти артефакт (для прикладу в README; у CI — це і є результат).
	if err := rep.SaveJSON("artifacts"); err != nil {
		t.Logf("не вдалося зберегти results json: %v", err)
	}
}
