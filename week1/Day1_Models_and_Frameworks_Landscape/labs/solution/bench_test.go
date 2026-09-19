package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTokenizerEstimate(t *testing.T) {
	tests := []struct {
		name string
		tok  Tokenizer
		text string
		want int
	}{
		{"порожній текст", Tokenizer{CharsPerToken: 4}, "", 0},
		{"латиниця по 4 символи", Tokenizer{CharsPerToken: 4}, "abcdefgh", 2},
		{"округлення вгору", Tokenizer{CharsPerToken: 4}, "abcde", 2},
		// "привіт" — 6 рун, але 11 байтів. Якби ми рахували байти, оцінка
		// для української була б завищена майже вдвічі.
		{"кирилиця рахується в рунах", Tokenizer{CharsPerToken: 3}, "привіт", 2},
		{"щільніший токенайзер дає більше токенів", Tokenizer{CharsPerToken: 3}, "abcdefgh", 3},
		{"нульове значення падає на дефолт", Tokenizer{}, "abcdefgh", 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tok.Estimate(tc.text); got != tc.want {
				t.Errorf("Estimate(%q) = %d, want %d", tc.text, got, tc.want)
			}
		})
	}
}

func TestCost(t *testing.T) {
	hosted := Pricing{InputPerMTok: 1, OutputPerMTok: 10}

	tests := []struct {
		name    string
		usage   Usage
		pricing Pricing
		want    float64
	}{
		{"нульове використання", Usage{}, hosted, 0},
		{
			name:    "вхід і вихід рахуються за різними ставками",
			usage:   Usage{InputTokens: 1_000_000, OutputTokens: 1_000_000},
			pricing: hosted,
			want:    11,
		},
		{
			// Головне: токени «роздумів» оплачуються як вихідні, хоча
			// користувач їх ніколи не бачить.
			name:    "thought-токени білляться як вихідні",
			usage:   Usage{InputTokens: 0, OutputTokens: 500_000, ThoughtTokens: 500_000},
			pricing: hosted,
			want:    10,
		},
		{
			name:    "self-hosted не має ціни за токен",
			usage:   Usage{InputTokens: 9_000_000, OutputTokens: 9_000_000},
			pricing: Pricing{SelfHosted: true},
			want:    0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Cost(tc.usage, tc.pricing)
			if err != nil {
				t.Fatalf("Cost() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("Cost() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPricingValidate — негативний тест, який несе тезу лекції.
//
// Прайс без ставки за вихідні токени не має давати «майже правильне» число.
// Він має падати: вихід коштує в рази дорожче за вхід, і калькулятор, який
// його ігнорує, занижує вартість задачі в кілька разів — рівно в той бік,
// у який приємно помилятися перед фінансовим директором.
func TestPricingValidate(t *testing.T) {
	tests := []struct {
		name    string
		pricing Pricing
		wantErr bool
	}{
		{"повний прайс", Pricing{InputPerMTok: 1, OutputPerMTok: 10}, false},
		{"self-hosted без ставок", Pricing{SelfHosted: true}, false},
		{"забули вихідні токени", Pricing{InputPerMTok: 1}, true},
		{"забули вхідні токени", Pricing{OutputPerMTok: 10}, true},
		{"self-hosted зі ставкою — суперечність", Pricing{InputPerMTok: 1, SelfHosted: true}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.pricing.Validate()
			if tc.wantErr {
				if !errors.Is(err, ErrIncompletePricing) {
					t.Fatalf("Validate() error = %v, want ErrIncompletePricing", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

// TestIgnoringOutputTokensUnderstatesCost показує ЦІНУ помилки, а не лише її
// наявність: на типовому співвідношенні токенів «вартість тільки за вхід»
// занижує рахунок у рази.
func TestIgnoringOutputTokensUnderstatesCost(t *testing.T) {
	pricing := Pricing{InputPerMTok: 1.25, OutputPerMTok: 10}
	usage := Usage{InputTokens: 1200, OutputTokens: 400, ThoughtTokens: 200}

	full, err := Cost(usage, pricing)
	if err != nil {
		t.Fatalf("Cost() error = %v", err)
	}
	naive := float64(usage.InputTokens) / 1_000_000 * pricing.InputPerMTok

	if ratio := full / naive; ratio < 3 {
		t.Errorf("повна вартість лише в %.1f× більша за наївну; тест мав би "+
			"показувати кратне заниження (full=%v naive=%v)", ratio, full, naive)
	}
	if _, err := Cost(usage, Pricing{InputPerMTok: 1.25}); !errors.Is(err, ErrIncompletePricing) {
		t.Errorf("Cost() з неповним прайсом error = %v, want ErrIncompletePricing", err)
	}
}

// TestCheaperPriceSheetCanCostMore — кейс «читайте дрібний шрифт».
//
// Модель B має нижчу ставку за токен, але щільніший токенайзер: на тому
// самому тексті вона видає більше токенів і в підсумку дорожча за задачу.
// Це і є різниця між прайс-листом і ефективною вартістю.
func TestCheaperPriceSheetCanCostMore(t *testing.T) {
	text := strings.Repeat("Комісія за chargeback залежить від тарифу мерчанта. ", 20)

	specA := ModelSpec{
		ID:        "model-a",
		Tokenizer: Tokenizer{CharsPerToken: 4.0},
		Pricing:   Pricing{InputPerMTok: 1.00, OutputPerMTok: 8.00},
	}
	specB := ModelSpec{
		ID:        "model-b",
		Tokenizer: Tokenizer{CharsPerToken: 2.8}, // «дрібніше» ріже той самий текст
		Pricing:   Pricing{InputPerMTok: 0.80, OutputPerMTok: 6.40},
	}
	if specB.Pricing.OutputPerMTok >= specA.Pricing.OutputPerMTok {
		t.Fatal("передумова тесту зламана: B має бути дешевшою за прайсом")
	}

	costOf := func(s ModelSpec) float64 {
		u := Usage{InputTokens: s.Tokenizer.Estimate(text), OutputTokens: s.Tokenizer.Estimate(text)}
		c, err := Cost(u, s.Pricing)
		if err != nil {
			t.Fatalf("Cost(%s) error = %v", s.ID, err)
		}
		return c
	}

	a, b := costOf(specA), costOf(specB)
	if b <= a {
		t.Errorf("ефективна вартість B = %v, A = %v; тест мав показати, що "+
			"нижчий прайс за токен не рятує від щільнішого токенайзера", b, a)
	}
}

func TestFitsContext(t *testing.T) {
	spec := ModelSpec{
		ID:            "small",
		ContextTokens: 100,
		Tokenizer:     Tokenizer{CharsPerToken: 4},
	}
	tests := []struct {
		name    string
		sc      Scenario
		wantErr bool
	}{
		{"влазить", Scenario{Name: "s", Prompt: strings.Repeat("a", 40), ExpectedOutputTokens: 50}, false},
		{"рівно межа", Scenario{Name: "s", Prompt: strings.Repeat("a", 200), ExpectedOutputTokens: 50}, false},
		{"довгий промпт не влазить", Scenario{Name: "s", Prompt: strings.Repeat("a", 400), ExpectedOutputTokens: 10}, true},
		{"очікуваний вихід не влазить", Scenario{Name: "s", Prompt: "abcd", ExpectedOutputTokens: 500}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := spec.FitsContext(tc.sc)
			if tc.wantErr != (err != nil) {
				t.Fatalf("FitsContext() error = %v, wantErr = %v", err, tc.wantErr)
			}
			if tc.wantErr && !errors.Is(err, ErrContextOverflow) {
				t.Errorf("error = %v, want ErrContextOverflow", err)
			}
		})
	}
}

// TestLoadEmbeddedCatalog перевіряє, що каталог, з яким реально їде лаба,
// валідний — і що дата в ньому є.
func TestLoadEmbeddedCatalog(t *testing.T) {
	c, err := LoadCatalog(bytes.NewReader(catalogJSON))
	if err != nil {
		t.Fatalf("LoadCatalog(catalog.json) error = %v", err)
	}
	if c.AsOf == "" {
		t.Error("as_of порожній")
	}
	if len(c.Models) < 2 {
		t.Errorf("моделей = %d; для порівняння треба щонайменше дві", len(c.Models))
	}
	for _, m := range c.Models {
		if m.AsOf != c.AsOf {
			t.Errorf("модель %q має as_of = %q, want %q (дата має дійти до кожного рядка таблиці)",
				m.ID, m.AsOf, c.AsOf)
		}
		if _, err := envVar(m.Provider); err != nil {
			t.Errorf("модель %q: %v", m.ID, err)
		}
	}
	if _, ok := c.Find("gemini-flash-latest"); !ok {
		t.Error("Find() не знайшов моделі, яка є в каталозі")
	}
	if _, ok := c.Find("no-such-model"); ok {
		t.Error("Find() знайшов модель, якої немає")
	}
}

func TestLoadCatalogRejectsBadInput(t *testing.T) {
	const okModel = `{"id":"m","provider":"gemini","label":"M","context_tokens":1000,
		"reasoning_effort":"low","tokenizer":{"name":"t","chars_per_token":4},
		"pricing":{"input_per_mtok":1,"output_per_mtok":10,"self_hosted":false}}`

	tests := []struct {
		name string
		json string
	}{
		{"не JSON", `{`},
		{"немає as_of", `{"as_of":"","source":"x","models":[` + okModel + `]}`},
		{"немає моделей", `{"as_of":"08/2026","source":"x","models":[]}`},
		{
			// Друкарська помилка в ключі не має «просто ігноруватись»:
			// саме так у каталог потрапляє ціна, яку ніхто не задав.
			name: "невідоме поле",
			json: `{"as_of":"08/2026","source":"x","modelz":[]}`,
		},
		{
			name: "дублікат id",
			json: `{"as_of":"08/2026","source":"x","models":[` + okModel + `,` + okModel + `]}`,
		},
		{
			name: "неповний прайс",
			json: `{"as_of":"08/2026","source":"x","models":[{"id":"m","provider":"gemini","label":"M",
				"context_tokens":1000,"reasoning_effort":"","tokenizer":{"name":"t","chars_per_token":4},
				"pricing":{"input_per_mtok":1,"output_per_mtok":0,"self_hosted":false}}]}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadCatalog(strings.NewReader(tc.json)); err == nil {
				t.Fatal("LoadCatalog() error = nil, want a loud failure")
			}
		})
	}
}

// sampleAt — короткий конструктор виміру для тестів агрегації.
func sampleAt(msLatency int, in, out int) Sample {
	return Sample{
		Latency: time.Duration(msLatency) * time.Millisecond,
		Usage:   Usage{InputTokens: in, OutputTokens: out},
	}
}

func TestResultAggregation(t *testing.T) {
	spec := ModelSpec{ID: "m", Pricing: Pricing{InputPerMTok: 1, OutputPerMTok: 10}}

	tests := []struct {
		name       string
		samples    []Sample
		wantMedian time.Duration
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{
			// Медіана, а не середнє: один викид на 5 с не має «псувати» модель.
			name:       "непарна кількість, один викид",
			samples:    []Sample{sampleAt(100, 10, 10), sampleAt(5000, 10, 10), sampleAt(120, 10, 10)},
			wantMedian: 120 * time.Millisecond,
			wantMin:    100 * time.Millisecond,
			wantMax:    5000 * time.Millisecond,
		},
		{
			name:       "парна кількість — середнє двох центральних",
			samples:    []Sample{sampleAt(100, 10, 10), sampleAt(200, 10, 10)},
			wantMedian: 150 * time.Millisecond,
			wantMin:    100 * time.Millisecond,
			wantMax:    200 * time.Millisecond,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := Result{Spec: spec, Scenario: "s", Samples: tc.samples}
			if got := r.MedianLatency(); got != tc.wantMedian {
				t.Errorf("MedianLatency() = %v, want %v", got, tc.wantMedian)
			}
			min, max := r.LatencyRange()
			if min != tc.wantMin || max != tc.wantMax {
				t.Errorf("LatencyRange() = %v..%v, want %v..%v", min, max, tc.wantMin, tc.wantMax)
			}
			if got := r.N(); got != len(tc.samples) {
				t.Errorf("N() = %d, want %d", got, len(tc.samples))
			}
		})
	}
}

func TestResultCostPerTask(t *testing.T) {
	spec := ModelSpec{ID: "m", Pricing: Pricing{InputPerMTok: 1, OutputPerMTok: 10}}

	r := Result{Spec: spec, Scenario: "s", Samples: []Sample{
		{Usage: Usage{InputTokens: 1_000_000, OutputTokens: 0}},
		{Usage: Usage{InputTokens: 1_000_000, OutputTokens: 200_000}},
	}}
	got, err := r.CostPerTask()
	if err != nil {
		t.Fatalf("CostPerTask() error = %v", err)
	}
	if want := 2.0; got != want { // (1 + 3) / 2
		t.Errorf("CostPerTask() = %v, want %v", got, want)
	}

	empty := Result{Spec: spec, Scenario: "s"}
	if _, err := empty.CostPerTask(); !errors.Is(err, ErrNoSamples) {
		t.Errorf("CostPerTask() без вимірів error = %v, want ErrNoSamples", err)
	}
}

// TestCompareRejectsIncomparableScenarios — другий негативний тест лекції.
//
// Найтихіший спосіб зробити безглуздий бенчмарк — виміряти моделі на різних
// завданнях і звести це в одну таблицю. Виглядатиме переконливо, означатиме
// нуль. Harness має відмовитись.
func TestCompareRejectsIncomparableScenarios(t *testing.T) {
	pricing := Pricing{InputPerMTok: 1, OutputPerMTok: 10}
	a := Result{
		Spec:     ModelSpec{ID: "a", Label: "A", Pricing: pricing},
		Scenario: "short-qa",
		Samples:  []Sample{sampleAt(100, 100, 100)},
	}
	b := Result{
		Spec:     ModelSpec{ID: "b", Label: "B", Pricing: pricing},
		Scenario: "long-rag-context",
		Samples:  []Sample{sampleAt(100, 100, 100)},
	}

	_, err := Compare([]Result{a, b})
	if !errors.Is(err, ErrIncomparable) {
		t.Fatalf("Compare() error = %v, want ErrIncomparable", err)
	}
	for _, want := range []string{"short-qa", "long-rag-context"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("повідомлення %q не називає сценарій %q", err, want)
		}
	}
}

func TestCompareRejectsEmptyInput(t *testing.T) {
	if _, err := Compare(nil); !errors.Is(err, ErrNoSamples) {
		t.Errorf("Compare(nil) error = %v, want ErrNoSamples", err)
	}
	empty := Result{Spec: ModelSpec{ID: "a", Pricing: Pricing{InputPerMTok: 1, OutputPerMTok: 10}}, Scenario: "s"}
	if _, err := Compare([]Result{empty}); !errors.Is(err, ErrNoSamples) {
		t.Errorf("Compare() з моделлю без вимірів error = %v, want ErrNoSamples", err)
	}
}

// TestReportIsMeasurementNotRanking закріплює вимогу STEARING: артефакт лаби —
// вимірюваний harness, а не рейтинг. Рядки йдуть за назвою моделі, тож
// найдешевша модель не опиняється зверху «як переможець».
func TestReportIsMeasurementNotRanking(t *testing.T) {
	pricing := Pricing{InputPerMTok: 1, OutputPerMTok: 10}
	cheap := Result{
		Spec:     ModelSpec{ID: "z", Label: "Zeta (дешева)", Pricing: pricing, AsOf: "08/2026", ReasoningEffort: "low"},
		Scenario: "s",
		Samples:  []Sample{sampleAt(100, 10, 10)},
	}
	pricey := Result{
		Spec:     ModelSpec{ID: "a", Label: "Alpha (дорога)", Pricing: pricing, AsOf: "08/2026"},
		Scenario: "s",
		Samples:  []Sample{{Latency: time.Second, Usage: Usage{InputTokens: 10_000, OutputTokens: 10_000}, Estimated: true}},
	}

	report, err := Compare([]Result{cheap, pricey})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if report.Scenario != "s" {
		t.Errorf("Scenario = %q, want %q", report.Scenario, "s")
	}
	if report.Rows[0].Model != "Alpha (дорога)" {
		t.Errorf("перший рядок = %q; сортування має бути за назвою, а не за метрикою",
			report.Rows[0].Model)
	}
	if report.Rows[0].CostUSD <= report.Rows[1].CostUSD {
		t.Error("передумова тесту зламана: перший рядок мав бути дорожчим")
	}
	if !report.Rows[0].Estimated {
		t.Error("Estimated не дійшов до рядка таблиці")
	}
	if report.Rows[1].Effort != "low" {
		t.Errorf("Effort = %q, want %q", report.Rows[1].Effort, "low")
	}
	if report.Rows[0].Effort != "default" {
		t.Errorf("Effort без значення = %q, want %q", report.Rows[0].Effort, "default")
	}

	md := report.Markdown()
	for _, want := range []string{"08/2026", "Alpha (дорога)", "оцінка", "не рейтинг"} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown() не містить %q", want)
		}
	}
}
