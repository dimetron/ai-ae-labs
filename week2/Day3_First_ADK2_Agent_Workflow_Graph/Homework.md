# Домашнє завдання 3 — Агент стає графом: вузли, ребра, тести

> **Станом на 09/2026 (перед використанням потрібно перевірити):** пін `google.golang.org/adk/v2` = v2.4.0, Go 1.27 — брати з `go.mod` лабораторії, не `@latest`. Сигнатури `workflow.NewFunctionNode`, `workflow.NewToolNodeTyped`, `workflow.Chain`, `agent.NewStrictContextMock` **звірено дослівно з модулем v2.4.0 13.09.2026** — точні рядки й файли в [`references.md`](references.md).

> **Одна пастка компіляції, яку варто знати заздалегідь.** `workflow.NewFunctionNode` повертає **одне** значення, а `workflow.NewToolNodeTyped` — **два** (`*ToolNode, error`). Це друга за частотою причина «не збирається» в цьому ДЗ.

## Легенда

Оксана з комплаєнсу LEDGERWORKS не може довести, чи агент справді відкрив кейс повернення мерчанту A-114, чи лише ввічливо про це написав. Досі ваш агент був «чорною скринькою»: модель сама вирішувала, коли викликати інструмент. Спершу запустіть reference `solution/` із лекції: він показує `Runner + LlmAgent + один tool` і той самий tool у графі без ключа. Далі ви розкладаєте цей шлях на **workflow-граф**: функції та інструменти стають вузлами, потік — явними ребрами, і кожен крок можна протестувати окремо.

**Наскрізний інструмент курсу вже у стартері.** ДЗ 2 було навмисно в іншому домені (курси валют, `get_exchange_rate`), тому переносити звідти нічого не треба: стартер постачає `open_refund_case` із контрактом `RefundCaseInput{TransactionID, MerchantID}` → `RefundCaseOutput{CaseID, TransactionID, MerchantID, Status}`. Ваша робота — граф навколо інструмента й тести, а не тіло інструмента. Фікстури: `txn-2026-07-118845`, `A-114`, кейс `rc-txn-2026-07-118845-A-114`.

## Ранній win: перший видимий результат за ≤15 хвилин

Перш ніж писати власний код, переконайтеся, що середовище живе:

```bash
cd courses/AI_Agents_Engineering/lectures
go test -v -run TestEventLogIsAuditable ./week2/Day3_First_ADK2_Agent_Workflow_Graph/solution
```

**Очікуваний результат** (звірено 22.08.2026):

```
=== RUN   TestEventLogIsAuditable
=== RUN   TestEventLogIsAuditable/LlmAgent
=== RUN   TestEventLogIsAuditable/workflow-граф
--- PASS: TestEventLogIsAuditable (0.00s)
    --- PASS: TestEventLogIsAuditable/LlmAgent (0.00s)
    --- PASS: TestEventLogIsAuditable/workflow-граф (0.00s)
PASS
ok  	github.com/dimetron/ai-eng-course/labs/week2/Day3_First_ADK2_Agent_Workflow_Graph/labs/solution
```

Без API-ключа й без мережі. Два підтести — це і є теза дня: **той самий аудит-слід** у LLM-шляху й у графі. Якщо тест зелений, усе далі — питання вашого коду, а не оточення.

Хочете побачити самі логи, а не тільки `PASS`:

```bash
go run ./week2/Day3_First_ADK2_Agent_Workflow_Graph/labs/solution
```

## Основне завдання

**Ваше завдання —** розкласти агента LEDGERWORKS на явний workflow-граф ADK 2.0, де кожен виклик `open_refund_case` є обов'язковим вузлом і лишає подію в event log, а не рішенням, яке ухвалює сама модель.

Зберіть `ADK 2.0 First Agent (Go)` як явний граф зі стартового шаблону [courses/AI_Agents_Engineering/lectures/week2/Day3_First_ADK2_Agent_Workflow_Graph/labs/main.go](labs/main.go):

1. Побудуйте статичний потік `Start → prepare → open_refund_case → format`: `workflow.NewFunctionNode` для підготовки та форматування, `workflow.NewToolNodeTyped` для типізованого інструмента зі стартера, ребра — через `workflow.Chain`.
2. Переконайтеся, що весь граф працює **без API-ключа**: жодного LLM-вузла, тільки функції та інструмент (`go run . console`).
3. Покрийте вузли табличними тестами на `agent.NewStrictContextMock` (див. `agent/context_mock.go`): щонайменше по 2 кейси на `prepare` і `format`, включно з помилковим входом. Для tool-handler перевірте успішний `StateDelta` і порожній `StateDelta` при помилці.
4. Додайте в README нормалізований фрагмент event log одного рану та розділ «Що дає граф проти imperative-скрипта» (5–7 речень). Не вигадуйте формат `[ev:*]`: зафіксуйте `session.Event` / `StateDelta` через власний стабільний formatter.

## Альтернативні теми (на вибір)

Та сама механіка (граф `FunctionNode → ToolNode → FunctionNode` + табличні тести), інший домен:

- **Security:** конвеєр-ревʼюер конфігів — `parse_yaml → check_rules → report`: знаходить у Kubernetes-манифесті privileged-контейнери та відсутні resource limits.
- **Research:** конспектор статей — `fetch_url → extract_text → summary_stub`: витягує текст сторінки й рахує статистику (слова, заголовки, посилання).
- **Fun:** граф-бариста — `parse_order → price_calculator → receipt`: перетворює «два лате і круасан» на типізований чек.

## ++ Advanced (до 20 балів, для сеньйорів)

- Умовна маршрутизація: додайте вузол-класифікатор із `workflow.StringRoute` (як у `examples/workflow/routing/string`), що обирає між двома інструментами залежно від запиту.
- Валідація схем вузла через `workflow.NewFunctionNodeWithSchema` + тест, що невалідний вхід відсікається до виконання функції.

## 🔥 Бонус-трек (не оцінюється, не потрібен для сертифіката)

- **Друга гілка маршрутизації.** Розширте `workflow.StringRoute` до трьох гілок (refund / статус транзакції / поза доменом) і додайте по одному табличному тест-кейсу на кожну. Балів не дає; робиться для себе.

## Якщо щось не працює

1. **`go build` падає на імпортах** — перевірте, що імпортуєте `google.golang.org/adk/v2/tool/functiontool`, а не пакет іншої версії ADK; потім `go mod tidy`.
2. **`NewToolNodeTyped` повертає помилку при збірці графа** — схема не виводиться з типів: звірте теги `json:"..."` і `jsonschema:"..."` на полях Input/Output.
3. **`go run . console` не стартує** — порівняйте `go version` із директивою `go` у `go.mod`; якщо launcher падає, `l.CommandLineSyntax()` покаже доступні підкоманди.
4. **Golden-тест «мигає» між ранами** — ви порівнюєте сирий stdout з нестабільними ID/timestamp; нормалізуйте формат власним formatter-ом і перевіряйте порядок business-подій та `StateDelta`.
5. **Нічого не працює і незрозуміло чому** — поверніться до раннього win: якщо `solution/`-тест зелений, проблема у вашому коді; якщо ні — у середовищі (версія Go, `go mod tidy`).

## Критерії оцінювання

| Критерій | Бали |
|---|---|
| Робочий граф `Start → prepare → tool → format` через `workflow.Chain` | 30 |
| `open_refund_case` підключено як типізований `ToolNode[RefundCaseInput, RefundCaseOutput]` | 15 |
| Табличні тести на `StrictContextMock`, усі зелені (≥4 кейси) | 25 |
| README: нормалізований event log фрагмент + «граф vs скрипт», зафіксовані версії `adk/v2` і Go | 10 |
| ++ Advanced (бонус) | 20 |
| **Разом** | **100** |

Базове завдання дає до 80 балів; ++ Advanced — це **+20 додаткових балів** (разом 100), він не є умовою сертифіката. Альтернативна тема оцінюється за тими самими критеріями. Бали за всі 12 завдань підсумовуються: від 60 % відкривається генерація сертифіката, від 85 % — з відзнакою.

## Формат здачі

GitHub-репозиторій з кодом, тестами та README; `go build ./...` і `go test ./...` мають проходити; посилання на репозиторій — у форму здачі. Якщо репозиторій закритий — додайте акаунт ментора в collaborators (акаунт указано в інструкції до курсу на платформі).

Стартовий шаблон: [courses/AI_Agents_Engineering/lectures/week2/Day3_First_ADK2_Agent_Workflow_Graph/labs/main.go](labs/main.go) · детермінований reference: [solution/](labs/solution/main.go) · еталонні API-приклади: [`sources/github/adk-go/examples/workflow/basic/`](https://github.com/google/adk-go/blob/v2.4.0/examples/workflow/basic/main.go), [`sources/github/adk-go/examples/workflow/routing/string/`](https://github.com/google/adk-go/blob/v2.4.0/examples/workflow/routing/string/main.go)

## Дедлайн

Два тижні після дати відкриття завдання.

Шановні слухачі!

Дедлайн надсилання розв'язку на перевірку — 23 год. 59 хв. 09.10.2026 (це крайній термін,
рекомендуємо виконати і надіслати розв'язок протягом тижня після відкриття завдання). Наступна
лекція спирається на цей артефакт, тому раннє надсилання допомагає вам самим.

**Інструкція до виконання завдання:**

До 23 год. 59 хв. 09.10.2026 року додайте отримані результати виконання завдання в наступному форматі.

Посилання на репозиторій з виконаним завданням з доданим акаунтом автора до collaborators
https://github.com/dimetron — активне посилання.

Натисніть кнопку «Надіслати відповідь та перейти до наступного етапу». Переконайтесь, що з'явився
напис «Чекаємо оцінки викладача».

Очікуйте на оцінку — згодом вона з'явиться під відповіддю у розділі «Оцінка викладача».

**Зверніть увагу!** Надіслати завдання вдруге неможливо — у вас є лише одна спроба.

Слухачам, які служать у ЗСУ, дедлайн продовжуємо за окремим запитом.

