// Package adkeval — бібліотека оцінювання ADK Go агентів у чотирьох методах.
//
// Це не порт adk-python eval, а навчальна реалізація тих самих ідей мовою Go:
// trajScore/responseMatch/LLM-as-judge-контракт + прогін по evalset через
// runner.NewInMemory. Мета — показати механіку, яку Python-харнес приховує.
package adkeval

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"math"
	"os"
	"strings"
	"unicode"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// ---- Модель-заглушка: детермінований «підробний LLM» -----------------

// ScriptedModel — model.LLM, який відтворює заздалегідь написані відповіді.
// Це те саме, що fake-сервіс для HTTP-тестів: оцінка стає повторюваною й
// безкоштовною. Сценарій — черга QScriptResponse; кожен виклик GenerateContent
// знімає наступний запис.
type ScriptedModel struct {
	NameLabel string
	Script    []QScriptResponse
	call      int
}

// QScriptResponse — один запис сценарію: або function_call, або текст.
type QScriptResponse struct {
	// FunctionCall, якщо not nil — модель «викликає інструмент».
	FunctionCall *genai.FunctionCall
	// Text — фінальна текстова відповідь моделі.
	Text string
}

// Name повертає назву моделі (частина model.LLM).
func (m *ScriptedModel) Name() string {
	if m.NameLabel == "" {
		return "scripted-model"
	}
	return m.NameLabel
}

// GenerateContent віддає наступний запис сценарію (model.LLM).
func (m *ScriptedModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if m.call >= len(m.Script) {
			yield(nil, fmt.Errorf("scripted model: сценарій вичерпано на виклику %d (перевірте кількість кроків у тесті)", m.call))
			return
		}
		s := m.Script[m.call]
		m.call++
		var content *genai.Content
		switch {
		case s.FunctionCall != nil:
			content = genai.NewContentFromFunctionCall(s.FunctionCall.Name, s.FunctionCall.Args, "model")
		default:
			content = genai.NewContentFromText(s.Text, "model")
		}
		yield(&model.LLMResponse{Content: content, TurnComplete: true}, nil)
	}
}

// ---- Метрики ----------------------------------------------------------

// FunctionCalls витягує function-call частини з події (копія логіки
// internal/utils — та сама визначена поведінка, без приватного імпорту).
func FunctionCalls(c *genai.Content) []*genai.FunctionCall {
	if c == nil {
		return nil
	}
	var out []*genai.FunctionCall
	for _, p := range c.Parts {
		if p.FunctionCall != nil {
			out = append(out, p.FunctionCall)
		}
	}
	return out
}

// TrajStep — один крок очікуваної траєкторії: ім'я інструмента (+опційно args,
// які порівнюються як JSON дрібно: лише ключі верхнього рівня, бо порядок
// полів і вкладеність не входять у ROUGE-подібну семантику ADK).
type TrajStep struct {
	Tool string
	Args map[string]any `json:"args,omitempty"`
}

// TrajScore — порт tool_trajectory_avg_score: середнє 1/0 збігів фактичних
// викликів проти очікуваних, по порядку. Це дає градацію («майже той шлях»),
// яку exact-match golden-тест не дає — і ту саму крихкість до зайвих викликів.
func TrajScore(expected []TrajStep, actual []*genai.FunctionCall) float64 {
	n := len(expected)
	if n == 0 && len(actual) == 0 {
		return 1
	}
	if n == 0 || len(actual) == 0 {
		return 0
	}
	matched := 0
	for i, e := range expected {
		if i >= len(actual) {
			break
		}
		if actual[i].Name != e.Tool {
			continue
		}
		if e.Args != nil && !looseArgsMatch(e.Args, actual[i].Args) {
			continue
		}
		matched++
	}
	return float64(matched) / float64(n)
}

// looseArgsMatch — збіг аргументів за ключами верхнього рівня.
func looseArgsMatch(want, got map[string]any) bool {
	if got == nil {
		return false
	}
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			return false
		}
		ws, gs := fmt.Sprintf("%v", wv), fmt.Sprintf("%v", gv)
		// числа: 118845 vs 118845.0 не повинні битись
		if ws != gs && !numEq(ws, gs) {
			return false
		}
	}
	return true
}

func numEq(a, b string) bool {
	var fa, fb float64
	if _, err := fmt.Sscanf(a, "%g", &fa); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(b, "%g", &fb); err != nil {
		return false
	}
	return math.Abs(fa-fb) < 1e-9
}

// Rouge1F1 — основа response_match_score ADK: unigram F1 candidate-vs-reference.
// ADK-Python використовує ROUGE-recall з warn про низькі пороги; F1 — ближча
// метрика для коротких відповідей, різницю задокументовано в README.
func Rouge1F1(candidate, reference string) float64 {
	c := shingles(candidate)
	r := shingles(reference)
	if len(r) == 0 {
		return 0
	}
	if len(c) == 0 {
		return 0
	}
	inter := 0
	for tok, n := range r {
		if m := c[tok]; m > 0 {
			if n < m {
				inter += n
			} else {
				inter += m
			}
		}
	}
	p := float64(inter) / float64(lenOf(c))
	rec := float64(inter) / float64(lenOf(r))
	if p+rec == 0 {
		return 0
	}
	return 2 * p * rec / (p + rec)
}

func shingles(s string) map[string]int {
	out := map[string]int{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		out[w]++
	}
	return out
}

func lenOf(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

// Normalize — нормалізація тексту для response-match метрик: нижній
// регістр, без пунктуації, поля через один пробіл. ROUGE на сирому тексті
// шумить через пунктуацію; це найпростіший чесний компроміс.
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// FinalResponse — остання текстова подія прогону.
func FinalResponse(events []*EventRecord) string {
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		if e.Text != "" && e.Author != "user" {
			return e.Text
		}
	}
	return ""
}

// ---- Запис траєкторії (для golden-файлів і offline-grading) ------------

// EventRecord — редукована подія прогону: все, що потрібно грейдеру.
type EventRecord struct {
	Author    string         `json:"author"`
	Text      string         `json:"text,omitempty"`
	ToolCalls []ToolCallRec  `json:"tool_calls,omitempty"`
	NodePath  string         `json:"node_path,omitempty"`
	Meta      map[string]any `json:"meta,omitempty"`
}

// ToolCallRec — нормалізований виклик інструмента в журналі.
type ToolCallRec struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// SaveGolden / LoadGolden — збереження журналу як golden-файлу тесту.
func SaveGolden(path string, events []*EventRecord) error {
	b, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// LoadGolden зчитує golden-журнал.
func LoadGolden(path string) ([]*EventRecord, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []*EventRecord
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("golden %s: %w", path, err)
	}
	return out, nil
}

// Verdict —outcome enum курсу, прив'язаний до оцінки прогону.
const (
	VerdictSuccess              = "success"
	VerdictInsufficientEvidence = "insufficient_evidence"
	VerdictNeedsHuman           = "needs_human"
	VerdictPolicyBlocked        = "policy_blocked"
)

// ClassifyVerdict мапить текстову відповідь агента на outcome enum.
// Це патерн курсу В.3: «я не знаю» — типізоване значення, а не вільний текст.
func ClassifyVerdict(finalText string) string {
	t := strings.ToUpper(strings.TrimSpace(finalText))
	switch {
	case strings.Contains(t, "POLICY_BLOCKED"):
		return VerdictPolicyBlocked
	case strings.Contains(t, "NEEDS_HUMAN"), strings.Contains(t, "ASK"):
		return VerdictNeedsHuman
	case t == "":
		return VerdictInsufficientEvidence
	default:
		return VerdictSuccess
	}
}
