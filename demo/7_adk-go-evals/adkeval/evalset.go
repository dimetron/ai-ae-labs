package adkeval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/genai"
)

// ---- EvalSet: формат, сумісний за духом з adk-python .evalset.json ----

// EvalCase — один кейс: питання + очікування (траєкторія і/або відповідь).
type EvalCase struct {
	EvalID            string     `json:"eval_id"`
	Prompt            string     `json:"prompt"`
	ExpectedToolCalls []TrajStep `json:"expected_tool_calls,omitempty"`
	ExpectedResponse  string     `json:"expected_response,omitempty"`
	TrajThreshold     float64    `json:"traj_threshold,omitempty"`     // дефолт 0.8
	ResponseThreshold float64    `json:"response_threshold,omitempty"` // дефолт 0.5
	ExpectedVerdict   string     `json:"expected_verdict,omitempty"`
}

// EvalSet — файл evalset-у.
type EvalSet struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Cases       []EvalCase `json:"eval_cases"`
}

// LoadEvalSet читає JSON-evalset з диска.
func LoadEvalSet(path string) (*EvalSet, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var es EvalSet
	if err := json.Unmarshal(b, &es); err != nil {
		return nil, fmt.Errorf("evalset %s: %w", path, err)
	}
	if len(es.Cases) == 0 {
		return nil, fmt.Errorf("evalset %s: нуль кейсів", path)
	}
	return &es, nil
}

// ---- Грейдер і звіт ---------------------------------------------------

// CaseResult — результат оцінки одного кейсу.
type CaseResult struct {
	EvalID        string  `json:"eval_id"`
	TrajScore     float64 `json:"tool_trajectory_avg_score"`
	ResponseScore float64 `json:"response_match_score"`
	Verdict       string  `json:"verdict"`
	Passed        bool    `json:"passed"`
	FailureCause  string  `json:"failure_cause,omitempty"` // правило курсу: причина виживає у звіт
}

// Report — підсумок прогону по evalset-у.
type Report struct {
	SetName   string       `json:"eval_set"`
	Model     string       `json:"model"`
	Timestamp string       `json:"timestamp"`
	Cases     []CaseResult `json:"cases"`
	Passed    int          `json:"passed"`
	Failed    int          `json:"failed"`
}

// SummaryText — людино-читаний рядок у стилі "Eval Run Summary" ADK.
func (r *Report) SummaryText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Eval Run Summary %s\n", r.SetName)
	fmt.Fprintf(&b, "model=%s tests passed=%d failed=%d\n", r.Model, r.Passed, r.Failed)
	for _, c := range r.Cases {
		status := "PASSED"
		cause := ""
		if !c.Passed {
			status = "FAILED"
			cause = " — " + c.FailureCause
		}
		fmt.Fprintf(&b, "  [%s] %s traj=%.2f resp=%.2f verdict=%s%s\n",
			status, c.EvalID, c.TrajScore, c.ResponseScore, c.Verdict, cause)
	}
	return b.String()
}

// SaveJSON пишуть звіт у results.json (артефакт CI).
func (r *Report) SaveJSON(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("results_%d.json", time.Now().Unix()))
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// RunEvalSet — full runner: агент × кожен кейс → грейдинг → Report.
//
// Це «eval run» у мініатюрі: generate (прогін агента) + grade (метрики) в
// одному виклику; each failure carries a named cause into the report.
func RunEvalSet(ctx context.Context, a agent.Agent, appName string, es *EvalSet) (*Report, error) {
	rep := &Report{SetName: es.Name, Timestamp: time.Now().UTC().Format(time.RFC3339)}
	for _, tc := range es.Cases {
		res := runCase(ctx, a, appName, tc)
		rep.Cases = append(rep.Cases, res)
		if res.Passed {
			rep.Passed++
		} else {
			rep.Failed++
		}
	}
	return rep, nil
}

func runCase(ctx context.Context, a agent.Agent, appName string, tc EvalCase) CaseResult {
	res := CaseResult{EvalID: tc.EvalID}
	events, err := RunOnce(ctx, a, appName, tc.Prompt)
	if err != nil {
		res.FailureCause = "run error: " + err.Error()
		return res
	}

	// Зібрати фактичні виклики й фінальну відповідь.
	var actual []*genai.FunctionCall
	for _, e := range events {
		for _, tcall := range e.ToolCalls {
			actual = append(actual, &genai.FunctionCall{Name: tcall.Name, Args: tcall.Args})
		}
	}
	final := FinalResponse(events)

	// Метрики.
	if len(tc.ExpectedToolCalls) > 0 {
		res.TrajScore = TrajScore(tc.ExpectedToolCalls, actual)
	}
	if tc.ExpectedResponse != "" {
		res.ResponseScore = Rouge1F1(Normalize(final), Normalize(tc.ExpectedResponse))
	} else if final != "" {
		// нема reference — оцінюємо лише наявність змістовної відповіді
		res.ResponseScore = 1.0
	}

	// Пороги (дефолти як в ADK: 0.8 траєкторія / 0.5 відповідь).
	trajT, respT := tc.TrajThreshold, tc.ResponseThreshold
	if trajT == 0 {
		trajT = 0.8
	}
	if respT == 0 {
		respT = 0.5
	}

	res.Verdict = ClassifyVerdict(final)

	// Критерії проходження + названа причина відмови.
	switch {
	case len(tc.ExpectedToolCalls) > 0 && res.TrajScore < trajT:
		res.FailureCause = fmt.Sprintf("trajectory %.2f < %.2f", res.TrajScore, trajT)
	case tc.ExpectedResponse != "" && res.ResponseScore < respT:
		res.FailureCause = fmt.Sprintf("response %.2f < %.2f", res.ResponseScore, respT)
	case tc.ExpectedVerdict != "" && res.Verdict != tc.ExpectedVerdict:
		res.FailureCause = fmt.Sprintf("verdict %s проти очікуваного %s", res.Verdict, tc.ExpectedVerdict)
	default:
		res.Passed = true
	}
	return res
}
