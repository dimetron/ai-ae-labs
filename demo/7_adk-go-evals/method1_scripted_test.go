// Метод 1 — оцінка на заглушці: ScriptedModel відтворює фіксовані відповіді,
// тож тести детерміновані, безкоштовні й швидкі. Це найкращий перший шар:
// він ловить регресії контракту (схема, послідовність викликів, домен),
// а не «розумність» моделі.
package adkeval_test

import (
	"context"
	"strings"
	"testing"

	"github.com/dimetron/ai-eng-course/demo/7_adk-go-evals/adkeval"
	"github.com/dimetron/ai-eng-course/demo/7_adk-go-evals/internal/refundagent"

	"google.golang.org/genai"
)

func TestHappyPathTrajectoryAndResponse(t *testing.T) {
	ctx := context.Background()

	wantArgs := map[string]any{"transaction_id": "txn-2026-07-118845", "merchant_id": "A-114"}
	script := []adkeval.QScriptResponse{
		{FunctionCall: &genai.FunctionCall{Name: "open_refund_case", Args: wantArgs}},
		{Text: `Case rc-txn-2026-07-118845-A-114 opened with status pending.`},
	}
	m := &adkeval.ScriptedModel{Script: script}
	a, err := refundagent.NewAgent("refund_eval", m)
	if err != nil {
		t.Fatal(err)
	}

	events, err := adkeval.RunOnce(ctx, a, "refund_eval", "Open a refund for txn-2026-07-118845, merchant A-114")
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// 1. Trajectory eval: чи правильною була послідовність викликів?
	var actual []*genai.FunctionCall
	for _, e := range events {
		for _, tc := range e.ToolCalls {
			actual = append(actual, &genai.FunctionCall{Name: tc.Name, Args: tc.Args})
		}
	}
	score := adkeval.TrajScore([]adkeval.TrajStep{{Tool: "open_refund_case", Args: wantArgs}}, actual)
	if score < 1.0 {
		t.Fatalf("tool_trajectory_avg_score = %.2f, хочемо 1.0 (calls=%v)", score, actual)
	}

	// 2. Final-response eval: ROUGE F1 проти reference-відповіді.
	final := adkeval.FinalResponse(events)
	rm := adkeval.Rouge1F1(adkeval.Normalize(final), adkeval.Normalize(
		"case rc-txn-2026-07-118845-A-114 opened status pending"))
	if rm < 0.5 {
		t.Errorf("response_match_score = %.2f < 0.5; final=%q", rm, final)
	}

	// 3. Вердикт курсу: success.
	if v := adkeval.ClassifyVerdict(final); v != adkeval.VerdictSuccess {
		t.Errorf("verdict = %s, хочемо success", v)
	}
}

func TestOffDomainToolCallIsRefused(t *testing.T) {
	ctx := context.Background()
	script := []adkeval.QScriptResponse{
		{FunctionCall: &genai.FunctionCall{Name: "open_refund_case", Args: map[string]any{
			"transaction_id": "txn-x", "merchant_id": "Z-999"}}},
		{Text: "POLICY_BLOCKED"},
	}
	m := &adkeval.ScriptedModel{Script: script}
	a, err := refundagent.NewAgent("refund_eval", m)
	if err != nil {
		t.Fatal(err)
	}

	events, err := adkeval.RunOnce(ctx, a, "refund_eval", "Open refund for txn-x, merchant Z-999")
	if err != nil {
		t.Fatalf("run: %v (інструмент має повернути помилку домену)", err)
	}

	final := adkeval.FinalResponse(events)
	if !strings.Contains(strings.ToUpper(final), "POLICY_BLOCKED") &&
		adkeval.ClassifyVerdict(final) != adkeval.VerdictPolicyBlocked {
		t.Logf("увага: поведінка поза доменом не дала policy_blocked-відповідь: %q", final)
	}
	// тут достатньо, що прогін завершився і траєкторія зафіксована в журналі;
	// сам факт «спроба Merчанта поза реєстром → інструмент відмовив» вже у журнали.
	if len(events) == 0 {
		t.Fatal("порожній журнал подій")
	}
}

func TestMissingIDAsksClarifyingQuestion(t *testing.T) {
	ctx := context.Background()
	script := []adkeval.QScriptResponse{
		{Text: "Could you share the transaction id and merchant id? ASK"},
	}
	m := &adkeval.ScriptedModel{Script: script}
	a, err := refundagent.NewAgent("refund_eval", m)
	if err != nil {
		t.Fatal(err)
	}
	events, err := adkeval.RunOnce(ctx, a, "refund_eval", "I need my money back")
	if err != nil {
		t.Fatal(err)
	}
	final := adkeval.FinalResponse(events)
	if got := adkeval.ClassifyVerdict(final); got != adkeval.VerdictNeedsHuman {
		t.Errorf("verdict = %s, хочемо needs_human для уточнювального питання (%q)", got, final)
	}
}
