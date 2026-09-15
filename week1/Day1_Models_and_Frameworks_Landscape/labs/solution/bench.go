// Week 1 Part 1 — Cross-Model Benchmark Harness.
//
// Цей файл — ADK-free ядро харнесу: модель вартості, оцінка токенів, агрегація
// вимірів і побудова порівняльної таблиці. Тут немає жодного імпорту агентного
// фреймворка — і це навмисно: арифметика «скільки коштує задача» не залежить
// від того, хто саме викликав модель, тож вона має тестуватись без ключа,
// без мережі й без ADK.
//
// Головна теза лекції, яку цей файл робить виконуваною: **ціна за токен ≠
// ефективна ціна за задачу**. Прайс-лист — це один множник. Токенайзер,
// довжина відповіді й бюджет «роздумів» — це три інші, і саме вони вирішують,
// хто дешевший на *вашому* сценарії.
//
// Чого тут свідомо немає: жодного «переможця». Harness друкує виміри, а
// рішення ухвалює інженер.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"
)

// Sentinel-помилки харнесу.
//
// Кожна з них — це окрема пастка, у яку легко впасти й отримати правдоподібне,
// але хибне число. Правдоподібно-хибне число гірше за помилку: його вставлять
// у слайд для ради директорів.
var (
	// ErrIncompletePricing — у прайсі не заповнена одна зі ставок. Найчастіше
	// забувають вихідні токени, і вартість задачі виходить у 3–5 разів меншою.
	ErrIncompletePricing = errors.New("incomplete pricing")
	// ErrContextOverflow — сценарій не влазить у контекстне вікно моделі.
	ErrContextOverflow = errors.New("scenario does not fit the context window")
	// ErrIncomparable — спроба порівняти виміри, зняті на різних сценаріях.
	ErrIncomparable = errors.New("results are not comparable")
	// ErrNoSamples — немає жодного виміру; один запуск теж не бенчмарк, але
	// нуль запусків — це вже точно не число.
	ErrNoSamples = errors.New("no samples")
	// ErrCatalogInvalid — каталог моделей не пройшов валідацію.
	ErrCatalogInvalid = errors.New("invalid model catalog")
)

// defaultCharsPerToken — груба емпірика для латиниці. Свідомо консервативна:
// для української тексту на символ припадає більше токенів, тому кожен
// ModelSpec задає власне значення в каталозі.
const defaultCharsPerToken = 4.0

// Tokenizer — те, як провайдер ріже текст на токени.
//
// Ми не реалізуємо справжній BPE (це окрема бібліотека й окрема залежність).
// Нам достатньо одного числа — скільки символів у середньому дає один токен, —
// бо саме воно пояснює кейс «Sonnet 5»: новий токенайзер видає на 30–40 %
// більше токенів на тому самому тексті, і нижчий прайс за токен перетворюється
// на вищий рахунок за задачу.
type Tokenizer struct {
	Name string `json:"name"`
	// CharsPerToken — середня кількість символів на один токен.
	// Менше значення = «дрібніше» ріже = більше токенів = дорожче.
	CharsPerToken float64 `json:"chars_per_token"`
}

// Estimate повертає оцінку кількості токенів у тексті.
//
// Рахуємо руни, а не байти. Для української це принципово: "привіт" — це
// 6 рун, але 12 байтів, і оцінка по байтах завищила б токени вдвічі.
func (t Tokenizer) Estimate(text string) int {
	cpt := t.CharsPerToken
	if cpt <= 0 {
		cpt = defaultCharsPerToken
	}
	runes := len([]rune(text))
	if runes == 0 {
		return 0
	}
	return int(math.Ceil(float64(runes) / cpt))
}

// Pricing — ставки провайдера за мільйон токенів.
type Pricing struct {
	InputPerMTok  float64 `json:"input_per_mtok"`
	OutputPerMTok float64 `json:"output_per_mtok"`
	// SelfHosted позначає модель, за яку немає ставки за токен (локальна
	// Ollama, власний vLLM). Її вартість не нульова — це GPU-години, — але
	// вона не рахується за токен, тому harness чесно ставить 0 і позначає
	// це в таблиці, замість вдавати, що self-hosted безкоштовний.
	SelfHosted bool `json:"self_hosted"`
}

// Validate ловить найдорожчу помилку цієї лабораторної.
//
// Прайс із заповненим тільки input — це не «частково заповнений прайс», це
// зламаний калькулятор: вихідні токени зазвичай коштують у 3–5 разів дорожче
// за вхідні, тож їх мовчазне ігнорування дає число, яке виглядає нормально й
// бреше в найважливішу сторону. Тому — гучна помилка, а не 0.
func (p Pricing) Validate() error {
	if p.SelfHosted {
		if p.InputPerMTok != 0 || p.OutputPerMTok != 0 {
			return fmt.Errorf("%w: self-hosted pricing must have zero rates, got in=%v out=%v",
				ErrIncompletePricing, p.InputPerMTok, p.OutputPerMTok)
		}
		return nil
	}
	if p.InputPerMTok <= 0 {
		return fmt.Errorf("%w: input_per_mtok is %v; a hosted model always bills input tokens",
			ErrIncompletePricing, p.InputPerMTok)
	}
	if p.OutputPerMTok <= 0 {
		return fmt.Errorf("%w: output_per_mtok is %v; output tokens cost 3–5× input and "+
			"ignoring them understates cost per task several-fold",
			ErrIncompletePricing, p.OutputPerMTok)
	}
	return nil
}

// Usage — токени одного виклику моделі.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	// ThoughtTokens — токени внутрішніх «роздумів» (reasoning effort).
	// Користувач їх не бачить, але платить за них як за вихідні. Це і є
	// та «шоста вісь» із лекції, виражена в грошах.
	ThoughtTokens int `json:"thought_tokens"`
}

// BilledOutput — те, за що виставляють рахунок як за вихідні токени.
func (u Usage) BilledOutput() int { return u.OutputTokens + u.ThoughtTokens }

// Total — усі токени виклику.
func (u Usage) Total() int { return u.InputTokens + u.BilledOutput() }

// Cost рахує вартість одного виклику в USD.
func Cost(u Usage, p Pricing) (float64, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	if p.SelfHosted {
		return 0, nil
	}
	const perMillion = 1_000_000.0
	return float64(u.InputTokens)/perMillion*p.InputPerMTok +
		float64(u.BilledOutput())/perMillion*p.OutputPerMTok, nil
}

// ModelSpec — один рядок датованого модельного знімка.
//
// Ключове рішення дизайну: список моделей НЕ захардкоджений у коді, він
// приходить із каталогу (див. LoadCatalog). Модельний ряд змінюється тижнями;
// код, у якому назва моделі — це рядковий літерал, застаріває мовчки.
type ModelSpec struct {
	ID       string `json:"id"`       // ідентифікатор моделі в API провайдера
	Provider string `json:"provider"` // gemini | openai | ollama
	Label    string `json:"label"`    // людська назва для таблиці
	// ContextTokens — контекстне вікно як обмежена робоча пам'ять.
	// Велике вікно не скасовує ані вартості, ані «lost in the middle».
	ContextTokens int `json:"context_tokens"`
	// ReasoningEffort — minimal | low | medium | high | "" (не задано).
	ReasoningEffort string    `json:"reasoning_effort"`
	Tokenizer       Tokenizer `json:"tokenizer"`
	Pricing         Pricing   `json:"pricing"`
	// AsOf заповнюється з каталогу, щоб дата дійшла до кожного рядка
	// таблиці. Слайд без дати — це слайд, який колись стане неправдою.
	AsOf string `json:"-"`
}

// Validate перевіряє, що специфікацію взагалі можна виміряти.
func (m ModelSpec) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("%w: model id is empty", ErrCatalogInvalid)
	}
	if strings.TrimSpace(m.Provider) == "" {
		return fmt.Errorf("%w: model %q has no provider", ErrCatalogInvalid, m.ID)
	}
	if m.ContextTokens <= 0 {
		return fmt.Errorf("%w: model %q has context_tokens=%d", ErrCatalogInvalid, m.ID, m.ContextTokens)
	}
	if err := m.Pricing.Validate(); err != nil {
		return fmt.Errorf("model %q: %w", m.ID, err)
	}
	return nil
}

// Scenario — однакове завдання, яке отримують усі моделі.
//
// «Однакове» тут — це вимога, а не побажання. Порівнювати моделі на різних
// промптах — це те саме, що порівнювати автомобілі, вимірявши один на трасі,
// а інший у місті.
type Scenario struct {
	Name   string
	Prompt string
	// ExpectedOutputTokens — очікуваний розмір відповіді. Потрібен, щоб
	// перевірити вміщення в контекст ДО того, як витратити гроші на запит.
	ExpectedOutputTokens int
}

// FitsContext перевіряє, чи влізе сценарій у контекстне вікно моделі.
//
// Перевірка робиться до виклику: провайдер відхилить завеликий запит уже
// після того, як ви за нього заплатили за вхідні токени.
func (m ModelSpec) FitsContext(s Scenario) error {
	in := m.Tokenizer.Estimate(s.Prompt)
	need := in + s.ExpectedOutputTokens
	if need > m.ContextTokens {
		return fmt.Errorf("%w: %q needs ~%d tokens (%d in + %d out) on %s, window is %d",
			ErrContextOverflow, s.Name, need, in, s.ExpectedOutputTokens, m.ID, m.ContextTokens)
	}
	return nil
}

// Catalog — датований знімок модельного ряду.
type Catalog struct {
	// AsOf — «станом на». Обов'язкове поле: каталог без дати не можна
	// показувати на камеру.
	AsOf   string      `json:"as_of"`
	Source string      `json:"source"`
	Models []ModelSpec `json:"models"`
}

// LoadCatalog читає й валідує каталог моделей.
//
// Валідація тут навмисно сувора: краще впасти на старті з повідомленням
// «у каталозі немає as_of», ніж півгодини бенчмаркати модель, яку провайдер
// перейменував три місяці тому.
func LoadCatalog(r io.Reader) (Catalog, error) {
	var c Catalog
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Catalog{}, fmt.Errorf("%w: %v", ErrCatalogInvalid, err)
	}
	if strings.TrimSpace(c.AsOf) == "" {
		return Catalog{}, fmt.Errorf("%w: as_of is empty; a model snapshot without a date is not verifiable",
			ErrCatalogInvalid)
	}
	if len(c.Models) == 0 {
		return Catalog{}, fmt.Errorf("%w: catalog has no models", ErrCatalogInvalid)
	}
	seen := make(map[string]bool, len(c.Models))
	for i, m := range c.Models {
		if err := m.Validate(); err != nil {
			return Catalog{}, err
		}
		if seen[m.ID] {
			return Catalog{}, fmt.Errorf("%w: duplicate model id %q", ErrCatalogInvalid, m.ID)
		}
		seen[m.ID] = true
		c.Models[i].AsOf = c.AsOf
	}
	return c, nil
}

// Find шукає специфікацію за ID.
func (c Catalog) Find(id string) (ModelSpec, bool) {
	for _, m := range c.Models {
		if m.ID == id {
			return m, true
		}
	}
	return ModelSpec{}, false
}

// Sample — один вимір: один прогін сценарію на одній моделі.
type Sample struct {
	Latency time.Duration
	Usage   Usage
	// Estimated=true означає, що токени порахували ми, бо провайдер не
	// повернув usage-метадані (типово для стрімінгу й деяких проксі).
	// Оцінка й реальний рахунок — це різні числа, і таблиця має це показувати.
	Estimated bool
}

// Result — усі виміри однієї моделі на одному сценарії.
type Result struct {
	Spec     ModelSpec
	Scenario string
	Samples  []Sample
}

// N — кількість вимірів. Одна проба — це не бенчмарк, і таблиця друкує n,
// щоб глядач сам бачив, наскільки числу можна вірити.
func (r Result) N() int { return len(r.Samples) }

// MedianLatency повертає медіану, а не середнє.
//
// Одна відповідь провайдера, що застрягла на 9 секундах, зсуває середнє й
// не зсуває медіану. Латентність майже завжди має довгий правий хвіст, тож
// середнє тут systematically бреше на користь «повільної» моделі.
func (r Result) MedianLatency() time.Duration {
	if len(r.Samples) == 0 {
		return 0
	}
	xs := make([]time.Duration, len(r.Samples))
	for i, s := range r.Samples {
		xs[i] = s.Latency
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
	mid := len(xs) / 2
	if len(xs)%2 == 1 {
		return xs[mid]
	}
	return (xs[mid-1] + xs[mid]) / 2
}

// LatencyRange повертає мінімум і максимум — розкид, який ховає медіана.
func (r Result) LatencyRange() (min, max time.Duration) {
	if len(r.Samples) == 0 {
		return 0, 0
	}
	min, max = r.Samples[0].Latency, r.Samples[0].Latency
	for _, s := range r.Samples[1:] {
		if s.Latency < min {
			min = s.Latency
		}
		if s.Latency > max {
			max = s.Latency
		}
	}
	return min, max
}

// MeanUsage — середні токени по вимірах (округлення вниз).
func (r Result) MeanUsage() Usage {
	if len(r.Samples) == 0 {
		return Usage{}
	}
	var in, out, thought int
	for _, s := range r.Samples {
		in += s.Usage.InputTokens
		out += s.Usage.OutputTokens
		thought += s.Usage.ThoughtTokens
	}
	n := len(r.Samples)
	return Usage{InputTokens: in / n, OutputTokens: out / n, ThoughtTokens: thought / n}
}

// CostPerTask — середня вартість одного прогону сценарію.
//
// Саме це число, а не «ціна за мільйон токенів», є одиницею порівняння:
// бізнес платить за закритий тікет, а не за токен.
func (r Result) CostPerTask() (float64, error) {
	if len(r.Samples) == 0 {
		return 0, fmt.Errorf("%w: %s", ErrNoSamples, r.Spec.ID)
	}
	var total float64
	for _, s := range r.Samples {
		c, err := Cost(s.Usage, r.Spec.Pricing)
		if err != nil {
			return 0, fmt.Errorf("model %q: %w", r.Spec.ID, err)
		}
		total += c
	}
	return total / float64(len(r.Samples)), nil
}

// HasEstimates повідомляє, чи є серед вимірів оцінені (не від провайдера).
func (r Result) HasEstimates() bool {
	for _, s := range r.Samples {
		if s.Estimated {
			return true
		}
	}
	return false
}

// Row — один рядок порівняльної таблиці.
type Row struct {
	Model      string
	Provider   string
	Effort     string
	N          int
	Median     time.Duration
	Min        time.Duration
	Max        time.Duration
	Usage      Usage
	CostUSD    float64
	SelfHosted bool
	Estimated  bool
	AsOf       string
}

// Report — порівняння кількох моделей на ОДНОМУ сценарії.
type Report struct {
	Scenario string
	Rows     []Row
}

// Compare зводить виміри в таблицю.
//
// Дві речі, які він робить навмисно:
//
//  1. Відмовляється зводити результати з різних сценаріїв (ErrIncomparable).
//     Це найпоширеніший спосіб отримати «бенчмарк», який нічого не міряє:
//     одну модель питали про погоду, іншу — про PCI DSS, і таблиця виглядає
//     переконливо.
//  2. Сортує рядки за ідентифікатором моделі, а НЕ за вартістю чи латентністю.
//     Сортування за метрикою — це вже рейтинг, а рейтинг ховає компроміс:
//     найдешевша модель може не проходити за data residency, найшвидша — за
//     якістю. Рішення ухвалює інженер, harness лише подає цифри.
func Compare(results []Result) (Report, error) {
	if len(results) == 0 {
		return Report{}, fmt.Errorf("%w: nothing to compare", ErrNoSamples)
	}

	scenario := results[0].Scenario
	rows := make([]Row, 0, len(results))
	for _, res := range results {
		if res.Scenario != scenario {
			return Report{}, fmt.Errorf(
				"%w: %q was measured on scenario %q, %q on %q; same prompt or no comparison",
				ErrIncomparable, results[0].Spec.ID, scenario, res.Spec.ID, res.Scenario)
		}
		if res.N() == 0 {
			return Report{}, fmt.Errorf("%w: model %q has no samples", ErrNoSamples, res.Spec.ID)
		}
		cost, err := res.CostPerTask()
		if err != nil {
			return Report{}, err
		}
		min, max := res.LatencyRange()
		rows = append(rows, Row{
			Model:      res.Spec.Label,
			Provider:   res.Spec.Provider,
			Effort:     effortLabel(res.Spec.ReasoningEffort),
			N:          res.N(),
			Median:     res.MedianLatency(),
			Min:        min,
			Max:        max,
			Usage:      res.MeanUsage(),
			CostUSD:    cost,
			SelfHosted: res.Spec.Pricing.SelfHosted,
			Estimated:  res.HasEstimates(),
			AsOf:       res.Spec.AsOf,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Model < rows[j].Model })
	return Report{Scenario: scenario, Rows: rows}, nil
}

// effortLabel нормалізує підпис бюджету «роздумів» для таблиці.
func effortLabel(effort string) string {
	if strings.TrimSpace(effort) == "" {
		return "default"
	}
	return strings.ToLower(strings.TrimSpace(effort))
}

// Markdown друкує таблицю, яку можна вставити в README лабораторної.
//
// Колонки підібрані так, щоб на них можна було відповісти на питання Ірини
// «скільки це коштує» і питання Оксани «звідки ці числа».
func (r Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Сценарій: %s\n\n", r.Scenario)
	b.WriteString("| модель | provider | effort | n | median | min–max | in tok | out tok | think tok | $/задача | токени | станом на |\n")
	b.WriteString("|---|---|---|---:|---:|---|---:|---:|---:|---:|---|---|\n")
	for _, row := range r.Rows {
		source := "provider"
		if row.Estimated {
			source = "оцінка"
		}
		cost := fmt.Sprintf("%.6f", row.CostUSD)
		if row.SelfHosted {
			cost = "self-hosted"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %s | %s–%s | %d | %d | %d | %s | %s | %s |\n",
			row.Model, row.Provider, row.Effort, row.N,
			ms(row.Median), ms(row.Min), ms(row.Max),
			row.Usage.InputTokens, row.Usage.OutputTokens, row.Usage.ThoughtTokens,
			cost, source, row.AsOf)
	}
	b.WriteString("\nТаблиця відсортована за назвою моделі, а не за метрикою: " +
		"це виміри, а не рейтинг.\n")
	return b.String()
}

// ms форматує тривалість у мілісекундах.
func ms(d time.Duration) string {
	return fmt.Sprintf("%.1f ms", float64(d)/float64(time.Millisecond))
}
