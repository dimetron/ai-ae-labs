// Метод 3 — LLM-as-judge на живій моделі.
//
// Рубрика-суддя читає діалог і повертає валідний JSON-вердикт; ми валідуємо
// схему вердикта (критик не проходить далі без passed/explanation) — той самий
// патерн schema-validated verdict, що й у multi-critic циклі В.11.
//
// Потрібен GEMINI_API_KEY (або інший ключ провайдера з models.dev каталогу).
// Без ключа тести скіпаються — `go test ./...` лишається зеленим у CI.
package adkeval_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dimetron/ai-eng-course/demo/7_adk-go-evals/adkeval"
	"github.com/dimetron/ai-eng-course/demo/7_adk-go-evals/internal/refundagent"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// judgeVerdict — схема відповіді судді.
type judgeVerdict struct {
	Passed      bool    `json:"passed"`
	Score       float64 `json:"score"`
	Explanation string  `json:"explanation"`
}

const judgePrompt = `You are an evaluation judge. Rate the assistant's final response.

<question>
%s
</question>

<assistant_response>
%s
</assistant_response>

Grade helpfulness and correctness for a refund-desk agent on a 0..1 scale.
Respond with ONLY this JSON object:
{"passed": <true|false>, "score": <0..1>, "explanation": "<one sentence>"}`

func TestLiveLLMJudge(t *testing.T) {
	ctx := context.Background()

	judgeModelName := "gemini-3.7-flash"
	judge, err := adkeval.LiveModel(ctx, judgeModelName)
	if err != nil {
		t.Skipf("нема живої моделі (%v) — skip, не fail", err)
	}

	// Агент під оцінку: та сама жива модель (у проді це була б окрема
	// конфігурація); тут важливий сам патерн «згенерував →ждавив».
	agentModel, err := adkeval.LiveModel(ctx, judgeModelName)
	if err != nil {
		t.Fatal(err)
	}
	a, err := refundagent.NewAgent("refund_eval", agentModel)
	if err != nil {
		t.Fatal(err)
	}

	question := "Open a refund for transaction txn-2026-07-118845, merchant A-114"
	events, err := adkeval.RunOnce(ctx, a, "refund_eval", question)
	if err != nil {
		t.Skipf("live run недоступний (%v) — skip, не fail (ключ/квота провайдера)", err)
	}
	final := adkeval.FinalResponse(events)
	if strings.TrimSpace(final) == "" {
		t.Fatal("порожня фінальна відповідь агента")
	}
	t.Logf("agent final: %s", final)

	verdict := askJudge(t, ctx, judge, judgePrompt, question, final)
	t.Logf("judge: passed=%v score=%.2f explanation=%s", verdict.Passed, verdict.Score, verdict.Explanation)

	if verdict.Score < 0.6 {
		t.Errorf("суддя поставив %.2f < 0.6: %s", verdict.Score, verdict.Explanation)
	}
}

// askJudge — один judge-виклик зі схемною валідацією.
func askJudge(t *testing.T, ctx context.Context, judge model.LLM, promptTpl, question, answer string) judgeVerdict {
	t.Helper()
	prompt := fmt.Sprintf(promptTpl, question, answer)
	for resp, err := range judge.GenerateContent(ctx, &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText(prompt, "user")},
	}, false) {
		if err != nil {
			t.Fatalf("judge call: %v", err)
		}
		raw := firstText(resp.Content)
		v, verdictErr := parseVerdict(raw)
		if verdictErr != nil {
			t.Fatalf("суддя повернув невалідний JSON (%v): %q", verdictErr, raw)
		}
		return v
	}
	t.Fatal("порожня відповідь судді")
	return judgeVerdict{}
}

func parseVerdict(raw string) (judgeVerdict, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return judgeVerdict{}, fmt.Errorf("не знайдено JSON-об'єкт")
	}
	var v judgeVerdict
	if err := json.Unmarshal([]byte(raw[start:end+1]), &v); err != nil {
		return judgeVerdict{}, err
	}
	return v, nil
}

func firstText(c *genai.Content) string {
	if c == nil {
		return ""
	}
	var out strings.Builder
	for _, p := range c.Parts {
		out.WriteString(p.Text)
	}
	return out.String()
}
