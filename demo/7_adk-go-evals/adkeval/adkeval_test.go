package adkeval

import (
	"testing"

	"google.golang.org/genai"
)

func TestTrajScoreExact(t *testing.T) {
	want := []TrajStep{{Tool: "a"}, {Tool: "b"}}
	got := []*genai.FunctionCall{{Name: "a"}, {Name: "b"}}
	if s := TrajScore(want, got); s != 1 {
		t.Errorf("exact match: %.2f != 1", s)
	}
}

func TestTrajScoreOrderMatters(t *testing.T) {
	want := []TrajStep{{Tool: "a"}, {Tool: "b"}}
	got := []*genai.FunctionCall{{Name: "b"}, {Name: "a"}}
	if s := TrajScore(want, got); s != 0 {
		t.Errorf("перестановка кроків має ламати порядок-залежний score: %.2f", s)
	}
}

func TestTrajScoreEmpty(t *testing.T) {
	if s := TrajScore(nil, nil); s != 1 {
		t.Errorf("обидві порожні = 1 (нема чого ламати), отримано %.2f", s)
	}
	if s := TrajScore([]TrajStep{{Tool: "x"}}, nil); s != 0 {
		t.Errorf("очікували виклик, не дочекались: %.2f", s)
	}
}

func TestRouge1F1IdenticalAndEmpty(t *testing.T) {
	if s := Rouge1F1("case opened pending", "case opened pending"); s < 0.99 {
		t.Errorf("ідентичні тексти: %.2f", s)
	}
	if s := Rouge1F1("", "reference"); s != 0 {
		t.Errorf("порожній кандидат має давати 0, отримано %.2f", s)
	}
}

func TestRouge1F1WordOrderTolerant(t *testing.T) {
	// ROUGE-1 — unigram метрика: перестановка слів не змінює множину.
	s := Rouge1F1("pending status opened case", "case opened pending status")
	if s < 0.99 {
		t.Errorf("unigram F1 не залежить від порядку слів: %.2f", s)
	}
}

func TestClassifyVerdict(t *testing.T) {
	cases := map[string]string{
		"POLICY_BLOCKED":              VerdictPolicyBlocked,
		"Could you share the id? ASK": VerdictNeedsHuman,
		"":                            VerdictInsufficientEvidence,
		"Case rc-123 opened.":         VerdictSuccess,
	}
	for in, want := range cases {
		if got := ClassifyVerdict(in); got != want {
			t.Errorf("ClassifyVerdict(%q)=%s, хочемо %s", in, got, want)
		}
	}
}

func TestGoldenRoundTrip(t *testing.T) {
	tDir := t.TempDir()
	path := tDir + "/g.json"
	in := []*EventRecord{{Author: "model", Text: "hi", ToolCalls: []ToolCallRec{{Name: "t"}}}}
	if err := SaveGolden(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadGolden(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Text != "hi" || out[0].ToolCalls[0].Name != "t" {
		t.Errorf("round trip втратив дані: %+v", out)
	}
}
