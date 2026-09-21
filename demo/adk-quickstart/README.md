# ADK Go v2 Model Expert Agent (з підтримкою pimodels та models.dev)

Приклад інтелектуального агента на **Google Agent Development Kit (ADK) Go v2** з підтримкою **multi-provider LLM** через пакет [`pimodels`](https://github.com/dimetron/pi-go/tree/main/pimodels) (`github.com/dimetron/pi-go/pimodels`) та вбудованим каталогом [`models.dev`](https://models.dev) (`resources/models.json`).

Агент **Model Expert** виступає в ролі AI Model Architect: аналізує характеристики та ціни, порівнює моделі за контекстним вікном, підтримкою reasoning, tool calling, structured output, open weights, а також рекомендує оптимальні моделі під конкретні технічні завдання (кодинг, математика, аналіз документів, дешевий інференс тощо). Працює як в **інтерактивній консолі**, так і у **веб-інтерфейсі (Web UI)**.

---

## 🛠 Доступні інструменти агента (ADK Function Tools)

Вбудований каталог індексує понад **6 800+ моделей** від **190+ провайдерів** та **300+ лабораторій**:

| Інструмент | Опис |
|---|---|
| `list_models` | Пошук та фільтрація моделей за текстом, провайдером, лабораторією (`lab`), сімейством, `reasoning`, `tool_call`, `open_weights`, мінімальним контекстом (`min_context`) та максимальною ціною (`max_input_cost`). |
| `list_providers` | Список провайдерів інференсу (OpenRouter, Groq, Together, HPC-AI, OpenAI тощо) з URL API та посиланнями на документацію. |
| `list_labs` | Список лабораторій-творців моделей (OpenAI, Anthropic, Google, DeepSeek, Meta, Mistral AI, Moonshot AI, Alibaba Qwen тощо) з кількістю моделей та основними сімействами. |
| `get_model_details` | Повні специфікації моделі: ліміт контексту, вихідних токенів, точна вартість input/output/cache per 1M tokens, reasoning опції, модальності, дати релізу. |
| `recommend_models` | Автоматичне ранжування та підбір найкращих моделей під конкретну задачу з поясненням критеріїв вибору (`fit_reason`). |

---

## 🚀 Швидкий старт (2 хвилини)

### 1. Переконайтеся, що існує `apps/.env`

Демо читає **спільний** файл `apps/.env` репозиторію — так само, як лаби тижня.
Під час старту `utils.LoadDotEnv()` спершу шукає локальний `./.env`, а якщо його
немає — піднімається вгору по дереву каталогів до найближчого `apps/.env`.
Копіювати щось у теку демо **не потрібно**.

Якщо файлу ще немає, створіть його з шаблону:

```bash
cp apps/.env-example apps/.env
```

### 2. Вкажіть API-ключ вашого провайдера

```bash
# Local agentgateway (те, що використовують лаби):
AGENTGATEWAY_API_KEY=agw_sk_...
# MODEL=agentgateway/gemini/gemini-3.8-flash

# Google Gemini:
GEMINI_API_KEY=AIzaSy...
# MODEL=gemini-3.7-flash

# Anthropic Claude:
# ANTHROPIC_API_KEY=sk-ant-...
# MODEL=claude-haiku-4.5
# MODEL=claude-sonnet-5

# OpenAI:
# OPENAI_API_KEY=sk-proj-...
# MODEL=gpt-5.6-luna
# MODEL=gpt-4o

# Локальна Ollama:
# MODEL=ollama/deepseek-v4-flash:0731
# MODEL=ollama/llama3
```

---

## 💬 Запуск агента

### Варіант A: Інтерактивний консольний режим

Запустіть агента через `task`:

```bash
task run
```

Інші способи запуску того самого режиму:

```bash
task run-console   # явно вказати підкоманду console
go run .           # без task-раннера
go run . console   # без task-раннера, явно console
```

Агент читає запити зі стандартного вводу. Завершення — `Ctrl+D` (EOF).

### Варіант B: Веб-інтерфейс (Web UI)

ADK Go v2 має вбудований веб-інтерфейс. Запустіть його командою:

```bash
task run-web
```

Інші способи запуску того самого режиму:

```bash
go run . web webui api   # без task-раннера
```

Після запуску відкрийте браузер за адресою:
👉 **http://localhost:8080/ui/**

Сервер піднімає три підсистеми одночасно:

| Прапорець | Що піднімає | Адреса |
|---|---|---|
| `web` | HTTP-сервер ADK | `http://localhost:8080` |
| `webui` | веб-інтерфейс для чату з агентом | `http://localhost:8080/ui/` |
| `api` | REST API для UI (CORS) | `http://localhost:8080/api` |

У веб-інтерфейсі доступний інтерактивний чат з агентом, перемикання сесій, перегляд викликів інструментів (Function Calls), стан агентів (State & Artifacts) та повне трасування подій (Event Traces):

![Google ADK Go v2 Model Expert Agent Web UI](img/screenshoot.png)

### Приклади діалогу

Обидва режими працюють з тим самим агентом і тим самим каталогом. Приклади
запитів і реальні відповіді агента:

```text
User  -> Скільки моделей у каталозі? Одним рядком.
Agent -> У каталозі налічується 4 823 моделі.

User  -> Які топ-3 лабораторії за кількістю моделей?
Agent -> 1. OpenAI — 659 моделей (сімейства: gpt, gpt-mini, gpt-oss)
         2. Qwen (Alibaba) — 451 модель (сімейства: qwen, qwen3.6, qwen3.5)
         3. Google — 319 моделей (сімейства: gemini-flash, gemma, gemini-pro)
         (На 4-му місці Anthropic із 207 моделями.)

User  -> Порадь 3 дешеві моделі для кодинг-агента з tool calling і контекстом від 128k.
Agent -> | Модель | Контекст | Input / Output (за 1M) | Tool Calling |
         | Gemini 3.7 Flash   | 1048k | $0.15 / $0.60 | Так |
         | DeepSeek V4 Flash  |  128k | $0.00         | Так |
         | Claude Haiku 4.5   |  200k | $1.00 / $5.00 | Так |

User  -> Які параметри та вартість у anthropic/claude-opus-4.7?
Agent -> Claude Opus 4.7 (Anthropic):
         - Контекстне вікно: 1 000 000 токенів (1M)
         - Макс. вихід: 128 000 токенів
         - Вартість: $5.00 / 1M input, $25.00 / 1M output, $0.50 / 1M cache read
         - Можливості: Reasoning (effort: low/medium/high), Tool Call, Multimodal (Text + Image)
```

### Приклад: передати запит без інтерактивної сесії

Агент читає зі stdin, тож запит можна подати одним рядком — зручно для
швидкої перевірки, що все налаштовано правильно:

```bash
echo "Скільки провайдерів і лабораторій у каталозі?" | go run . console
```

### Приклад: змінити порт Web UI

Прапорець `-port` належить підкоманді `web`, тому передається після неї:

```bash
go run . web -port 9090 webui api
# UI: http://localhost:9090/ui/
```

---

## 🏗 Як влаштований агент

1. **Вбудований каталог (`//go:embed resources/models.json`)**:
   Файл з повною базою моделей компілюється безпосередньо в бінарний файл через механізм `go:embed`. При старті `catalog.Store` створює швидкі індекси в пам'яті.

2. **Декларативні інструменти ADK v2 (`functiontool.New`)**:
   Усі схеми інструментів автоматично виводяться з типізованих Go-структур (`ListModelsInput`, `GetModelDetailsInput`, `RecommendModelsInput` тощо).

3. **Multi-provider LLM через `pimodels`**:
   Будь-яка обрана модель (Gemini, Claude, GPT, Ollama) повертається як `pimodels.Model`, що реалізує інтерфейс ADK `model.LLM`.

---

## 🛠 Корисні команди ([Taskfile](https://taskfile.dev/))

Усі команди запускаються через `task` з теки `demo/adk-quickstart`:

| Команда | Опис |
|---|---|
| `task` | Показати перелік усіх доступних команд (те саме, що `task --list`) |
| `task run` | Запустити агента в інтерактивній консолі (`go run .`) |
| `task run-console` | Те саме, але з явною підкомандою `console` |
| `task run-web` | Запустити Web UI на порту 8080 (`go run . web webui api`) |
| `task build` | Скомпілювати бінарні файли у `bin/` (`bin/adk-quickstart`, `bin/login`) |
| `task test` | Запустити модульні та офлайн-тести |
| `task vet` | Запустити аналізатор коду `go vet` |
| `task fmt` | Відформатувати всі Go-файли через `gofmt -w .` |
| `task verify` | Повна перевірка: форматування, vet, тести, компіляція |
| `task login-codex` | Увійти через OpenAI Codex Device Flow (запускає `cmd/login codex`) |
| `task login PROV=...` | Увійти через OAuth/Device (наприклад: `task login PROV=opencode`) |

> **Примітка:** Додаток автоматично підвантажує змінні оточення під час запуску,
> навіть якщо ви запускаєте `go run .` напряму. Спершу читається локальний `./.env`,
> а якщо його немає — найближчий `apps/.env` вище по дереву каталогів.
> Явний `export` у терміналі має пріоритет над файлом.

