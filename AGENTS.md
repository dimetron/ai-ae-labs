# AGENTS.md — репозиторій курсу «AI Agents Engineering» (тиждень 1)

Цей документ описує структуру, команди збірки та конвенції репозиторію для агентів
і розробників. **Область дії — весь репозиторій** (це гілка лише з матеріалами тижня 1).

---

## 1. Загальний огляд

Репозиторій з кодом курсу **AI Agents Engineering** (Prometheus): стартові шаблони
лабораторних робіт, умови домашніх завдань і демо-проєкти тижня 1.

**Стек:** Go 1.27 · `google.golang.org/adk/v2 v2.4.0` (ADK вимагає Go ≥ 1.26.6) · станом на 09/2026.

Репозиторій містить **три незалежні Go-модулі** — це важливо, бо `go build ./...`
з кореня не покриває демо:

| Модуль | Шлях | Призначення |
|---|---|---|
| кореневий | `./` (`github.com/dimetron/ai-eng-course/labs`) | лаби тижня 1 + `internal/` |
| quickstart | `demo/adk-quickstart/` | Week 1 starter: ADK Go v2 агент |
| mock-llm | `demo/1_ai-gateway/mock-llm/` | keyless mock для agentgateway |

---

## 2. Команди (SDLC Gates)

Усі команди — **з кореня репозиторію**, якщо не вказано інше.

```bash
# Збірка й тести кореневого модуля (лаби тижня 1 + internal/) — без ключів і без мережі
go build ./... && go test ./...

# go vet + перевірка форматування
go vet ./...
gofmt -l .          # форматуйте файли, які правите: gofmt -w <файл>

# Офлайн-шлях без жодного ключа (ранній win) — рахує повну таблицю
cd week1/Day1_Models_and_Frameworks_Landscape/labs/solution && go run .

# Лаби з реальною моделлю потребують ключа (apps/.env або env-змінна)
go run ./week1/Day1_Models_and_Frameworks_Landscape/labs

# Day2: -offline бере ЛИШЕ фікстурні курси НБУ; модель усе одно потрібна
cd week1/Day2_Structured_Output_Function_Calling/labs && go run . -offline console
```

```bash
# Демо adk-quickstart — окремий модуль, команди з його теки
cd demo/adk-quickstart
make verify     # gofmt → go vet → go test -v → build у bin/ (або: task verify)
make run        # інтерактивний консольний режим
make run-web    # Web UI на http://localhost:8080/ui/

# Демо agentgateway — Docker-стек
cd demo/1_ai-gateway && docker compose up -d
```

Перед комітом мають проходити: `go build ./...`, `go test ./...` і `make verify`
у `demo/adk-quickstart`. `make build`/`make verify` пише бінарники в
`demo/adk-quickstart/bin/` — теку додано в `.gitignore`, не комітьте її.

> **Відоме відхилення:** `gofmt -l .` у корені **не порожній** — 7 файлів у
> `week1/Day2_Structured_Output_Function_Calling/labs/` (`compare*.go`,
> `ratelimit*.go`, `rates.go`, `*_e2e_test.go`) не відформатовані. Це стан,
> успадкований з `main`, а не результат вашої роботи. Форматуйте лише файли,
> які правите; масовий `gofmt -w` по цих файлах — окремим комітом.

---

## 3. Структура

```
week1/<День>/labs/          стартовий шаблон лаби + README з покроковою інструкцією
week1/<День>/Homework.md    умова ДЗ, критерії оцінювання, формат здачі
week1/<День>/labs/solution/ еталонний розв'язок (Day 1)
internal/                   спільні helper-пакети: adkenv, fakellm, labrun
apps/.env-example           шаблон ключів провайдерів (копія → apps/.env)
demo/adk-quickstart/        Week 1 starter — окремий Go-модуль
demo/1_ai-gateway/          agentgateway + Jaeger/Prometheus/Grafana
```

---

## 4. Інваріанти розробки (Rules & Invariants)

1. **`internal/` — спільний код лаб.** Пакети `adkenv`, `fakellm`, `labrun`
   імпортуються **лише лабами тижня 1**. Якщо додаєте туди код, переконайтеся, що
   він не залежить від `week1/` (заборона циклів) і не ламає `go build ./...`.
2. **Провайдери — з `apps/.env`, а не хардкодом.** Ключі читаються з `apps/.env`
   (шаблон — `apps/.env-example`) або зі змінних оточення; **змінна оточення має
   пріоритет**. Перший знайдений провайдер виграє. Кожна лаба друкує, якого
   провайдера обрала і чому.
3. **Офлайн-режим обов'язковий для тестів.** `go test ./...` має проходити без
   ключів і без мережі (скриптована модель `fakellm` + фікстури в `testdata/`).
   Не ламайте це, додаючи обов'язкові мережеві виклики в тести.
   *Увага:* `-offline` у Day2 вимикає **лише** живе API курсів НБУ — чат із
   моделлю все одно потребує провайдера. Єдина повністю безключова точка входу —
   `week1/Day1_.../labs/solution` (`go run .`).
4. **Секрети — ніколи в git.** Реальні ключі не комітяться; у документації та тестах —
   лише плейсхолдери й фікстури.
5. **Три модулі — три перевірки.** Зміна в `demo/adk-quickstart` перевіряється
   `make verify` усередині його теки, а не з кореня.
6. **Мова документації — українська.** README, Homework і lab-README — українською;
   коментарі в коді та ідентифікатори — англійською.
7. **Версії піняться з датою.** У документації фіксуйте версії з міткою «станом на
   ММ/РРРР»: ADK і agentgateway — молоді проєкти, схеми змінюються між мінорними релізами.

---

## 5. Куди що додавати

- Новий день тижня → `week1/Day<N>_<Назва>/` з `Homework.md` + `labs/` (шаблон і README).
- Спільний для лаб helper → `internal/<пакет>/` + тест.
- Демо курсу → `demo/<назва>/` зі **своїм** `go.mod`, якщо це окремий застосунок.
- Інструкція для студента → у README відповідної лаби, а не в кореневий README.
