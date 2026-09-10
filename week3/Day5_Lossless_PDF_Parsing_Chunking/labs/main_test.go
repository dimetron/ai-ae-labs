// Скелет тестів для ДЗ 5.
//
// Homework.md, пункт 7, вимагає табличні тести на кожен вузол графа через
// agent.StrictContextMock — і `go test ./...` зеленим. Цей файл дає готову
// форму, щоб ви не витрачали перші 40 хвилин на «як узагалі підсунути
// agent.Context у функцію вузла».
//
// ЯК ЦИМ КОРИСТУВАТИСЬ
//
//	go test ./...            — зараз зелено: тести перевіряють ЗАГЛУШКИ
//	                           (один гігантський чанк — це поточна поведінка).
//	                           Це навмисно: зелений старт означає, що
//	                           середовище живе, і далі ви ламаєте тести
//	                           свідомо, а не воюєте з інструментами.
//
// Ваш шлях: реалізуєте TODO у main.go → тести нижче ПАДАЮТЬ, бо вони описують
// стару поведінку → переписуєте їх під нову. Місця, які треба переписати,
// позначені TODO(студент). Тест «таблиця не ріжеться» (TestChunk_TableStaysWhole)
// уже описує ЦІЛЬОВУ поведінку і має падати, доки chunk — заглушка: це
// контракт ДЗ, а не заготовка. Він єдиний тут із t.Skip — зніміть skip, коли
// візьметесь за chunking.
//
// Чому StrictContextMock, а не nil: вузли приймають agent.Context. Передати
// туди nil можна лише доти, доки ви його не використовуєте; щойно у вашому
// коді з'явиться ctx.InvocationID() або емісія події, nil дасть panic у
// найгіршому місці — у проді. StrictContextMock панікує ГУЧНО і одразу на
// будь-якому методі, про який ви не подумали, тож помилка знаходиться в тесті,
// а не на демо.
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/adk/v2/agent"
)

// nodeCtx — суворий фейк agent.Context. Вбудований StrictContextMock означає:
// будь-який метод, який ваш вузол викличе, а ми його тут не передбачили,
// впаде з panic, а не поверне тихо нульове значення.
type nodeCtx struct {
	agent.StrictContextMock
}

func newNodeCtx() *nodeCtx {
	return &nodeCtx{StrictContextMock: agent.NewStrictContextMock(context.Background())}
}

// --- Вузол load --------------------------------------------------------------

func TestLoad(t *testing.T) {
	dir := t.TempDir()

	okPath := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(okPath, []byte("# Розділ\n\nтекст\n"), 0o600); err != nil {
		t.Fatalf("підготовка фікстури: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		wantErr  bool
		contains string
	}{
		{name: "читає наявний файл", path: okPath, contains: "# Розділ"},
		{name: "файла немає", path: filepath.Join(dir, "ghost.md"), wantErr: true},
		{name: "порожній шлях", path: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := load(newNodeCtx(), tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("load(%q) = %q, err = nil; очікували помилку", tt.path, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("load(%q) error = %v", tt.path, err)
			}
			if !strings.Contains(got, tt.contains) {
				t.Errorf("load(%q) = %q; очікували підрядок %q", tt.path, got, tt.contains)
			}
		})
	}
}

// --- Вузол chunk -------------------------------------------------------------

// TestChunk_Stub фіксує ПОТОЧНУ поведінку заглушки: один чанк на весь документ.
//
// TODO(студент): щойно ви реалізуєте Parent-Child chunking, цей тест має
// впасти — і це правильно. Замініть його на перевірку вашої ієрархії:
// скільки parents, скільки children, чи в кожного child заповнений ParentID,
// чи Level зростає всередині розділу.
func TestChunk_Stub(t *testing.T) {
	got, err := chunk(newNodeCtx(), "# Розділ\n\nабзац\n")
	if err != nil {
		t.Fatalf("chunk() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("заглушка має повертати 1 чанк, отримали %d — схоже, ви вже реалізували chunking; перепишіть цей тест", len(got))
	}
	if got[0].ParentID != "" {
		t.Errorf("ParentID кореневого чанка = %q, очікували порожній", got[0].ParentID)
	}
}

func TestChunk_EmptyDocumentIsError(t *testing.T) {
	if _, err := chunk(newNodeCtx(), ""); err == nil {
		t.Fatal("chunk(\"\") = nil error; порожній документ має бути помилкою")
	}
}

// TestChunk_TableStaysWhole — КОНТРАКТ ДЗ, а не заготовка.
//
// Головна вимога тижня: жодна таблиця не може бути розрізана між двома
// чанками. Тест перевіряє це напряму — усі рядки однієї Markdown-таблиці
// мають опинитися в ОДНОМУ чанку з Kind == "table".
//
// TODO(студент): приберіть t.Skip, коли візьметесь за вузол chunk.
func TestChunk_TableStaysWhole(t *testing.T) {
	t.Skip("зніміть skip, коли реалізуєте Parent-Child chunking у main.go")

	const doc = `# Ставки

| Мерчант | Тариф | Ставка комісії, % |
|---------|-------|-------------------|
| A-114   | T-2   | 2.9               |
| A-207   | T-2   | 2.9               |
| A-331   | T-1   | 1.8               |

Далі йде звичайний абзац.
`

	chunks, err := chunk(newNodeCtx(), doc)
	if err != nil {
		t.Fatalf("chunk() error = %v", err)
	}

	var tableChunks []Chunk
	for _, c := range chunks {
		if c.Kind == "table" {
			tableChunks = append(tableChunks, c)
		}
	}
	if len(tableChunks) != 1 {
		t.Fatalf("очікували рівно 1 чанк типу table, отримали %d — таблицю розрізано", len(tableChunks))
	}
	for _, row := range []string{"A-114", "A-207", "A-331", "Ставка комісії"} {
		if !strings.Contains(tableChunks[0].Text, row) {
			t.Errorf("у табличному чанку немає %q — рядок або заголовок втрачено:\n%s", row, tableChunks[0].Text)
		}
	}
}

// --- Вузол report ------------------------------------------------------------

// TODO(студент): за Homework п.5 report має друкувати статистику за рівнями,
// розміри (min/avg/max) і `Tables: K/K preserved`. Розширте таблицю кейсів,
// коли реалізуєте це.
func TestReport(t *testing.T) {
	tests := []struct {
		name     string
		chunks   []Chunk
		contains string
	}{
		{name: "порожній вхід", chunks: nil, contains: "0"},
		{name: "один чанк", chunks: []Chunk{{ID: "c1"}}, contains: "1"},
		{name: "три чанки", chunks: []Chunk{{ID: "c1"}, {ID: "c2"}, {ID: "c3"}}, contains: "3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := report(newNodeCtx(), tt.chunks)
			if err != nil {
				t.Fatalf("report() error = %v", err)
			}
			if !strings.Contains(got, tt.contains) {
				t.Errorf("report() = %q; очікували підрядок %q", got, tt.contains)
			}
		})
	}
}
