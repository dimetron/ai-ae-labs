// Метод 2 — golden-журнал (offline trajectory grading).
//
// Крок A: записуємо прогін у golden.jsonl. Це роблять ОДИН раз — коли
// поведінка підтверджена людиною; файл комітиться в репо.
// Крок B: кожен наступний CI-прогін генерує новий журнал і порівнюється
// з золотим: траєкторія exact-match, фінальна відповідь — ROUGE-поріг.
//
// Це саме та межа, яку курс іменує «trajectory ≠ final answer»: золотий
// журнал ловить ШЛЯХ (виклики, порядок, аргументи), ROUGE — останнє слово.
package adkeval_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dimetron/ai-eng-course/demo/7_adk-go-evals/adkeval"
	"github.com/dimetron/ai-eng-course/demo/7_adk-go-evals/internal/refundagent"

	"google.golang.org/genai"
)

const updateGoldenEnv = "EVAL_UPDATE_GOLDEN"

func TestGoldenTrajectory(t *testing.T) {
	ctx := context.Background()

	script := []adkeval.QScriptResponse{
		{FunctionCall: &genai.FunctionCall{Name: "open_refund_case", Args: map[string]any{
			"transaction_id": "txn-2026-07-118845", "merchant_id": "A-114"}}},
		{Text: `Case rc-txn-2026-07-118845-A-114 opened with status pending.`},
	}
	m := &adkeval.ScriptedModel{Script: script}
	a, err := refundagent.NewAgent("refund_eval", m)
	if err != nil {
		t.Fatal(err)
	}

	events, err := adkeval.RunOnce(ctx, a, "refund_eval",
		"Open a refund for txn-2026-07-118845, merchant A-114")
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	goldenPath := filepath.Join("evaldata", "golden_happy_path.json")

	// Режим запису: EVAL_UPDATE_GOLDEN=1 go test ./... — перезаписує golden
	// ПІСЛЯ того, як людина перевірила поведінку очима. У CI цей режим
	// заборонений (флаг не ставиться), тому тест не може тихо «погодитись».
	if os.Getenv(updateGoldenEnv) == "1" {
		if err := adkeval.SaveGolden(goldenPath, events); err != nil {
			t.Fatalf("save golden: %v", err)
		}
		t.Logf("golden оновлено: %s", goldenPath)
		return
	}

	golden, err := adkeval.LoadGolden(goldenPath)
	if err != nil {
		t.Skipf("нема golden-файлу (%v); запусти EVAL_UPDATE_GOLDEN=1 go test ./... один раз", err)
	}

	// Trajectory exact-match: послідовність інструментів з_golden.
	gotCalls, wantCalls := []adkeval.ToolCallRec{}, []adkeval.ToolCallRec{}
	for _, e := range events {
		gotCalls = append(gotCalls, e.ToolCalls...)
	}
	for _, e := range golden {
		wantCalls = append(wantCalls, e.ToolCalls...)
	}
	if len(gotCalls) != len(wantCalls) {
		t.Fatalf("траєкторія змінилась: %d викликів проти golden %d (%v vs %v)",
			len(gotCalls), len(wantCalls), gotCalls, wantCalls)
	}
	for i := range gotCalls {
		if gotCalls[i].Name != wantCalls[i].Name {
			t.Errorf("крок %d: %s проти golden %s", i, gotCalls[i].Name, wantCalls[i].Name)
		}
	}

	// Final response: не exact-match (крихко), а ROUGE F1 ≥ порогу.
	fr := adkeval.FinalResponse(events)
	gr := adkeval.FinalResponse(golden)
	if s := adkeval.Rouge1F1(adkeval.Normalize(fr), adkeval.Normalize(gr)); s < 0.85 {
		t.Errorf("final-response drift: ROUGE-F1 = %.2f < 0.85\n  got:  %q\n  gold: %q", s, fr, gr)
	}
}

func ExampleTrajScore_partialMatch() {
	expected := []adkeval.TrajStep{{Tool: "lookup"}, {Tool: "open_refund_case"}}
	actual := []*genai.FunctionCall{
		{Name: "open_refund_case"}, // lookup пропущено → крок 0 не збігся
		{Name: "open_refund_case"},
	}
	fmt.Printf("%.2f\n", adkeval.TrajScore(expected, actual))
	// Output: 0.50
}
