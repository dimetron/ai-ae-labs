# AGENTS.md — ADK Go v2 Model Expert Agent

Цей документ визначає архітектуру, правила, конвенції та інструменти для агентів і розробників, які працюють із модулем `demo/adk-quickstart`.

---

## 1. Загальний огляд (Overview)

`demo/adk-quickstart` — це еталонна реалізація інтелектуального агента (**Model Expert Agent**) на базі **Google Agent Development Kit (ADK) Go v2**.

### Ключові можливості:
- **Multi-Provider LLM**: Автоматичне визначення та перемикання провайдерів (Google Gemini, Anthropic Claude, OpenAI, Ollama) через бібліотеку `github.com/dimetron/pi-go/pimodels`.
- **Вбудований каталог (Embedded Catalog)**: Повна база даних `models.dev` (6800+ моделей, 190+ провайдерів, 300+ labs) зашита у бінарник через `//go:embed`.
- **Автономність (Zero External DB)**: In-memory індексація, швидка фільтрація, ранжування та технічні паспорти без зовнішніх СКБД чи мережевих запитів.
- **Готовий Web UI & CLI**: Підтримка запуску консольного інтерфейсу та Web UI з візуалізацією графу виконання й трасування сесій через `launcher/full`.

---

## 2. Команди розробки (SDLC Gates)

Всі перевірки та запуски виконуються через `Makefile` або `Taskfile.yml` ([taskfile.dev](https://taskfile.dev/)):

```bash
# Запуск інтерактивного Web UI на http://localhost:8080/ui/
make run-web    # або: task run-web

# Запуск у CLI-режимі
make run        # або: task run

# Повний цикл верифікації (форматування gofmt, перевірка go vet, тести, компіляція)
make verify     # або: task verify

# Запуск усіх unit-тестів
make test       # або: task test

# Збірка бінарників у папку bin/
make build      # або: task build
```

---

## 3. Структура проєкту (Project Layout)

```
demo/adk-quickstart/
├── AGENTS.md                  # Цей файл: правила, архітектура та карта для агентів
├── README.md                  # Документація для користувачів та студентів курсу
├── Makefile                   # Стандартизовані команди make
├── Taskfile.yml               # Сучасний таск-ранер Taskfile (taskfile.dev)
├── main.go                    # Точка входу: <100 LOC, multi-provider запуск та launcher
├── main_test.go               # Тести ініціалізації та багатопровайдерного перемикання
├── .env.example               # Зразок конфігурації API-ключів та вибору моделей
├── resources/
│   ├── system_prompt.md       # Зовнішній системний промпт (embedded у main.go)
│   ├── models.json            # Повний зліпок API каталогу models.dev (~9.3 MB)
│   └── recommended_models.json# Офіційні рекомендації моделей для курсу Prometheus
└── internal/
    ├── tools/                 # Повністю інкапсульований пакет інструментів агента
    │   ├── types.go           # Типізовані DTO та JSON Schema для ADK Function Calling
    │   ├── store.go           # In-memory індекс, багатопараметрична фільтрація й ранжування
    │   ├── tools.go           # Конструктори ADK інструментів та tools.All()
    │   └── tools_test.go      # Тести валідації інструментів та пошуку
    ├── auth/                  # OAuth 2.0 PKCE / Device Code flow для провайдерів
    └── utils/                 # Утиліти (автоматичне завантаження .env файлів)
```

---

## 4. Набір інструментів агента (ADK Function Tools)

Пакет `internal/tools` експортує єдиний метод `tools.All() []tool.Tool`, що реєструє 6 інструментів:

| Інструмент | Опис | Призначення |
|---|---|---|
| `list_course_models` | Офіційний перелік рекомендованих моделей курсу Prometheus | Витягує моделі з `recommended_models.json` з ролями та порадами щодо `.env` |
| `get_model_details` | Технічний паспорт конкретної моделі | Точні ціни (input/output/cache per 1M), контекстне вікно, ліміти виводу, reasoning options |
| `list_models` | Гнучкий пошук та фільтрація каталогу models.dev | Фільтри за назвою, провайдером, lab, сімейством, ціною, розміром контексту, reasoning |
| `recommend_models` | Підбір та ранжування моделей під задачу | Скоринг моделей під coding agents, classification, math reasoning, long context |
| `list_providers` | Довідник хостинг-провайдерів інференсу | 190+ провайдерів (OpenRouter, Groq, Together, DeepSeek, OpenAI) з API URLs та docs |
| `list_labs` | Довідник дослідницьких лабораторій | 300+ організацій (OpenAI, Anthropic, Google DeepMind, Meta, Mistral, Moonshot, Qwen) |

---

## 5. Конфігурація моделей (.env)

Провайдер визначається автоматично через префікс або ім'я моделі:
- **Google Gemini**: `MODEL=gemini-3.7-flash` (потрібен `GEMINI_API_KEY` або `GOOGLE_API_KEY`)
- **Anthropic Claude**: `MODEL=claude-haiku-4.5` або `claude-sonnet-5` (потрібен `ANTHROPIC_API_KEY`)
- **OpenAI**: `MODEL=gpt-5.6-luna` або `gpt-4o` (потрібен `OPENAI_API_KEY`)
- **Ollama (Локально)**: `MODEL=ollama/deepseek-v4-flash:0731` або `ollama/llama3` (працює без зовнішніх ключів)

---

## 6. Інваріанти розробки (Rules & Invariants)

1. **Лаконічність `main.go`**: Файл `main.go` є навчальним фасадом і має залишатися компактним (**<100 LOC**).
2. **Інкапсуляція інструментів**: Усі нові структури даних, парсери та інструменти додаються виключно в `internal/tools`.
3. **Автономність бінарника**: Усі ресурси (промпти, схеми, каталоги) підвантажуються через `//go:embed`, без хардкоду абсолютних або динамічних локальних шляхів.
4. **Якість коду**: Будь-які зміни повинні безпомилково проходити `make verify` (`gofmt`, `go vet`, unit tests, `go build`).
