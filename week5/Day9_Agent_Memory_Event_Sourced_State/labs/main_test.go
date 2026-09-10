// Каркас тестів ДЗ 9. Він компілюється й проходить одразу — саме тому
// перший `go test` у цій лабі зелений, а не «[no test files]».
//
// «Немає тестових файлів» — найгірший з можливих результатів: він
// виглядає як успіх і рівно тому нікого не змушує подивитись уважніше.
// Це та сама причина, з якої ми в лекції вимагаємо, щоб відмова затвора
// несла названу причину, а не тишу.
//
// Нижче — один робочий тест і два скелети, позначені t.Skip. Скелети НЕ
// падають: вони друкують, який саме пункт ДЗ закривають. Зніміть Skip,
// коли писатимете тіло.
//
//	go test ./...            # усе зелене, скіпи видно
//	go test -race ./...      # так це перевірятимуть при оцінюванні
//	go test -run Concurrent -v ./...
package main

import (
	"testing"
)

// TestRememberFactRejectsEmptyKey — робочий тест, який проходить одразу.
//
// Він тут не для повноти покриття, а щоб показати ідіому: guard-clause
// перевіряється БЕЗ рушія, сесії та мережі. rememberFact відсіює порожній
// ключ до того, як торкнеться ctx, тому agent.Context може бути nil —
// це інтерфейс. Жодного GOOGLE_API_KEY для тестів не потрібно, і це
// свідома властивість дизайну: тест, який вимагає ключ, не запуститься
// в CI.
func TestRememberFactRejectsEmptyKey(t *testing.T) {
	tests := []struct {
		name    string
		in      FactInput
		wantErr bool
	}{
		{name: "порожній ключ відхиляється", in: FactInput{Key: "", Value: "legacy"}, wantErr: true},
		{name: "порожній ключ і порожнє значення", in: FactInput{}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rememberFact(nil, tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("rememberFact(%+v) err = %v, wantErr = %v", tt.in, err, tt.wantErr)
			}
		})
	}
}

// TestTwoSessionsMemorySurvives — ДЗ 9, пункт 1 основного завдання.
//
// Що довести: факт, записаний у сесії A, доступний у сесії B того самого
// userID/appName ПІСЛЯ AddSessionToMemory. Без другої сесії демо
// «з амнезією і без» не існує, а саме воно оцінюється в 30 балів.
//
// Каркас:
//
//	sessionService := session.InMemoryService()
//	memoryService := memory.InMemoryService()
//	prev, _ := sessionService.Create(ctx, &session.CreateRequest{AppName: …, UserID: …})
//	// … AppendEvent зі щонайменше 3 повідомленнями …
//	memoryService.AddSessionToMemory(ctx, prev.Session)
//	// … друга сесія того самого userID; перевірити, що факт видно …
//
// Еталон каркаса — sources/github/adk-go/examples/tools/loadmemory/main.go
// (helper createPreviousSessionWithHistory). Мережа не потрібна: обидва
// сервіси — in-memory.
func TestTwoSessionsMemorySurvives(t *testing.T) {
	t.Skip("ДЗ 9, п.1: напишіть сценарій двох сесій і зніміть цей Skip")
}

// TestStateDeltaConcurrentConsolidation — ДЗ 9, ++ Advanced (до 20 балів).
//
// Що довести: ≥10 goroutine-ів пишуть у РІЗНІ ключі user:worker:<i>, і
// жодне значення не загублено. Запускати обов'язково з -race.
//
// Межа, яку тест НЕ доводить і яку треба назвати в README: відсутність
// data race ≠ вирішений конфлікт двох записів в ОДИН ключ. Там перемагає
// останній, і для лічильника чи балансу потрібен окремий дизайн (atomic
// або транзакція). Тест на різні ключі — про перше, не про друге.
func TestStateDeltaConcurrentConsolidation(t *testing.T) {
	t.Skip("ДЗ 9, ++ Advanced: напишіть тест на ≥10 паралельних записів і зніміть цей Skip")
}
