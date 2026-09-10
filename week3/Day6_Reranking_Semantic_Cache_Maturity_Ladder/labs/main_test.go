// Скелет тестів для ДЗ 6.
//
// Homework.md вимагає зелений `go test ./...`; цей файл дає готову форму, щоб
// перші хвилини пішли на retrieval, а не на «як підсунути agent.Context».
//
// ЯК ЦИМ КОРИСТУВАТИСЬ
//
//	go test ./...            — зараз зелено: тести описують ЗАГЛУШКИ стартера.
//	                           Зелений старт означає, що середовище живе.
//
// Далі ви реалізуєте TODO у main.go, тести падають (бо описують стару
// поведінку) — і ви переписуєте їх під свою. Місця позначені TODO(студент).
//
// Два тести нижче — НЕ заготовки, а контракти ДЗ, і вони під t.Skip доти,
// доки ви не візьметесь за відповідний пункт:
//
//	TestRerank_ChangesOrder      — критерій «5 запитів до/після» (15 балів)
//	TestCache_ParaphraseHits     — критерій «hit на перефразованому» (10 балів)
//
// Чому StrictContextMock, а не nil: вузли приймають agent.Context. nil працює
// рівно доти, доки ви його не торкаєтесь; щойно у вузлі з'явиться
// ctx.InvocationID() — nil дасть panic там, де його ніхто не чекає.
// StrictContextMock панікує гучно і одразу на будь-якому непередбаченому
// методі, тож помилка ловиться в тесті, а не на демо.
package main

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/adk/v2/agent"
)

type nodeCtx struct {
	agent.StrictContextMock
}

func newNodeCtx() *nodeCtx {
	return &nodeCtx{StrictContextMock: agent.NewStrictContextMock(context.Background())}
}

// resetCache — кеш у стартері глобальний, тому тести мусять його чистити.
//
// Це не косметика: два тести, що ділять глобальний стан, дають «іноді
// червоно» — найгірший вид падіння, бо його списують на випадковість.
func resetCache(t *testing.T) {
	t.Helper()
	cache = map[string]string{}
	t.Cleanup(func() { cache = map[string]string{} })
}

// --- Вузол search ------------------------------------------------------------

func TestSearch(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		wantErr     bool
		wantCandGTE int
	}{
		{name: "звичайний запит", query: "Що таке тариф T-2?", wantCandGTE: 1},
		{name: "порожній запит — помилка", query: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCache(t)
			got, err := search(newNodeCtx(), tt.query)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("search(%q) error = nil; очікували помилку", tt.query)
				}
				return
			}
			if err != nil {
				t.Fatalf("search(%q) error = %v", tt.query, err)
			}
			if got.Query != tt.query {
				t.Errorf("Query = %q, очікували %q", got.Query, tt.query)
			}
			if len(got.Candidates) < tt.wantCandGTE {
				t.Errorf("кандидатів = %d, очікували щонайменше %d", len(got.Candidates), tt.wantCandGTE)
			}
			if got.CacheHit {
				t.Errorf("CacheHit = true на порожньому кеші")
			}
		})
	}
}

// --- Ранній win із лекції ----------------------------------------------------

// TestPipeline_EarlyWin проганяє повний шлях search → rerank → answer у пам'яті,
// без launcher-а. Це той самий «перший видимий результат», що на слайді, тільки
// відтворюваний у CI.
//
// Перевіряємо не текст відповіді (він у стартері — заглушка), а те, що
// у відповіді є provenance: ідентифікатор чанка, з якого вона зібрана.
// Відповідь без посилання на джерело в цьому курсі не зараховується.
func TestPipeline_EarlyWin(t *testing.T) {
	resetCache(t)
	ctx := newNodeCtx()

	res, err := search(ctx, "Що таке тариф T-2?")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	ranked, err := rerank(ctx, res)
	if err != nil {
		t.Fatalf("rerank: %v", err)
	}
	out, err := answer(ctx, ranked)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}

	if len(ranked.Candidates) == 0 {
		t.Fatal("після rerank не лишилось кандидатів")
	}
	topID := ranked.Candidates[0].ChunkID
	if !strings.Contains(out, topID) {
		t.Errorf("у відповіді немає provenance (%q):\n%s", topID, out)
	}
}

func TestAnswer_NoCandidates(t *testing.T) {
	resetCache(t)
	out, err := answer(newNodeCtx(), SearchResult{Query: "щось"})
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if !strings.Contains(out, "Нічого не знайдено") {
		t.Errorf("порожній набір кандидатів має давати явну відмову, отримали %q", out)
	}
}

// --- Кеш ---------------------------------------------------------------------

// TestCache_ExactHit фіксує поточну поведінку заглушки: кеш точний,
// ключ — сам рядок запиту.
func TestCache_ExactHit(t *testing.T) {
	resetCache(t)
	ctx := newNodeCtx()
	const q = "Що таке тариф T-2?"

	first, err := search(ctx, q)
	if err != nil {
		t.Fatalf("search #1: %v", err)
	}
	if first.CacheHit {
		t.Fatal("перший запит не може бути hit")
	}
	if _, err := answer(ctx, first); err != nil {
		t.Fatalf("answer: %v", err)
	}

	second, err := search(ctx, q)
	if err != nil {
		t.Fatalf("search #2: %v", err)
	}
	if !second.CacheHit {
		t.Errorf("повтор того самого запиту мав дати cache hit")
	}
}

// TestCache_ParaphraseHits — КОНТРАКТ ДЗ (10 балів), не заготовка.
//
// Точний кеш ловить лише буквальний повтор. Вимога ДЗ — семантичний кеш:
// перефразований запит теж має влучати.
//
// TODO(студент): приберіть t.Skip, коли реалізуєте схожість замість map-lookup.
// І одразу додайте ДРУГИЙ тест — на false positive: два РІЗНІ за змістом
// запити, які не мають ділити відповідь. Високий hit-rate без виміряного
// false-positive rate — це не перемога, а регресія якості (див. Lecture,
// Блок 4, «І ще одне: кеш — це межа безпеки»).
func TestCache_ParaphraseHits(t *testing.T) {
	t.Skip("зніміть skip, коли реалізуєте семантичний кеш замість точного map-lookup")

	resetCache(t)
	ctx := newNodeCtx()

	first, err := search(ctx, "Що таке тариф T-2?")
	if err != nil {
		t.Fatalf("search #1: %v", err)
	}
	if _, err := answer(ctx, first); err != nil {
		t.Fatalf("answer: %v", err)
	}

	para, err := search(ctx, "Розкажи про тарифний план T-2")
	if err != nil {
		t.Fatalf("search перефразованого: %v", err)
	}
	if !para.CacheHit {
		t.Errorf("перефразований запит не влучив у семантичний кеш")
	}
}

// --- Re-ranking --------------------------------------------------------------

// TestRerank_Stub фіксує поточну поведінку: заглушка не змінює порядок.
//
// TODO(студент): цей тест має впасти, щойно ви реалізуєте re-ranker.
func TestRerank_Stub(t *testing.T) {
	in := SearchResult{Query: "q", Candidates: []Candidate{
		{ChunkID: "a", Score: 0.1},
		{ChunkID: "b", Score: 0.9},
	}}
	got, err := rerank(newNodeCtx(), in)
	if err != nil {
		t.Fatalf("rerank: %v", err)
	}
	if got.Candidates[0].ChunkID != "a" {
		t.Fatalf("заглушка не мала змінювати порядок, отримали топ-1 = %q — схоже, re-ranker уже працює; перепишіть цей тест", got.Candidates[0].ChunkID)
	}
}

// TestRerank_ChangesOrder — КОНТРАКТ ДЗ (15 балів), не заготовка.
//
// Сенс re-ranker-а в тому, що він піднімає справді релевантний чанк над
// семантично близьким, але нерелевантним. Класичний приклад із лекції:
// на запит про T-2 vector повертає нагору чанк про T-1, бо тексти майже
// однакові; cross-encoder бачить пару «запит ↔ чанк» цілком і опускає його.
//
// TODO(студент): приберіть t.Skip і підставте свої кандидати. Пару «до/після»
// із цього тесту можна одразу класти в README — це один із п'яти потрібних
// прикладів.
func TestRerank_ChangesOrder(t *testing.T) {
	t.Skip("зніміть skip, коли реалізуєте re-ranker у main.go")

	in := SearchResult{Query: "яка ставка комісії на тарифі T-2?", Candidates: []Candidate{
		{ChunkID: "c-A331-rate", Text: "Мерчант A-331 обслуговується за тарифом T-1, ставка комісії 1.8 %.", Score: 0.88},
		{ChunkID: "c-A114-rate", Text: "Мерчант A-114 обслуговується за тарифом T-2, ставка комісії 2.9 %.", Score: 0.86},
	}}

	got, err := rerank(newNodeCtx(), in)
	if err != nil {
		t.Fatalf("rerank: %v", err)
	}
	if len(got.Candidates) == 0 {
		t.Fatal("re-ranker не лишив жодного кандидата")
	}
	if got.Candidates[0].ChunkID != "c-A114-rate" {
		t.Errorf("топ-1 = %q, очікували c-A114-rate: re-ranker не підняв чанк про T-2 над чанком про T-1",
			got.Candidates[0].ChunkID)
	}
}
