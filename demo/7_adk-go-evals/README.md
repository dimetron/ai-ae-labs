# 7_adk-go-evals — Agent Evals у ADK Go: чотири методи

Демо-проєкт до курсу **AI Agents Engineering** (Тиждень 4/6): як оцінювати
ADK Go агента, коли в SDK **нема** нативного eval API. Станом на `adk-go`
v2.2.0 (08/2026) єдиний eval-код у репозиторії — заглушка REST-роутера;
повний стек (`eval sets`, `tool_trajectory_avg_score`, LLM-as-judge,
user simulation) живе на Python/хмарній стороні. Первинне джерело — стаб
REST-роутера в самому SDK:
[`server/adkrest/internal/routers/eval.go`](https://github.com/google/adk-go/blob/main/server/adkrest/internal/routers/eval.go).

Цей проєкт закриває прогалину чотирма практичними методами, від дешевого до
дорожчого, і показує механіку, яку Python-харнес приховує.

## Агент під оцінкою

`internal/refundagent` — мінімальний refund-desk агент курсу: один типізований
інструмент `open_refund_case` (доменна межа `allowedMerchants`), інструкція з
правилами, outcome enum (`success | insufficient_evidence | needs_human |
policy_blocked`) як контракт вердиктів.

## Бібліотека `adkeval`

| Імпорт | Що дає |
|---|---|
| `adkeval.ScriptedModel` | детермінована модель-заглушка (сценарій Q→A) |
| `adkeval.RunOnce` | прогін через `runner.NewInMemory` → `[]EventRecord` |
| `adkeval.TrajScore` | порт `tool_trajectory_avg_score` (порядок + args, середнє 1/0) |
| `adkeval.Rouge1F1` | основа `response_match_score` (unigram F1; ADK-Python рахує recall — різниця задокументована) |
| `adkeval.ClassifyVerdict` | текстова відповідь → outcome enum |
| `adkeval.SaveGolden/LoadGolden` | golden-журнали траєкторії для offline-грейдингу |
| `adkeval.LoadEvalSet / RunEvalSet / Report` | evalset-раннер із порогами, причинами відмов і results_*.json |

## Чотири методи

### Метод 1 — Scripted model (unit-grade, безкоштовно, `<1s`)

Файл: [`method1_scripted_test.go`](method1_scripted_test.go)

Підміняємо модель сценарієм: перший виклик → function call, другий → фінальна
 відповідь. Прогін через реальний `Runner`, але нуль токенів і нуль мережі.
Оцінюємо одночасно всі три виміри:

```go
score := adkeval.TrajScore([]adkeval.TrajStep{{Tool: "open_refund_case", Args: wantArgs}}, actual)
rm    := adkeval.Rouge1F1(adkeval.Normalize(final), adkeval.Normalize(reference))
v     := adkeval.ClassifyVerdict(final) // success
```

**Що ловить:** регресії контракту — неправильний інструмент, зіпсовані
аргументи, зникнення вердикту. **Чого не ловить:** «розумність» справжньої
моделі — сценарій завжди той самий.

### Метод 2 — Golden trajectory log (offline regression, exact path)

Файл: [`method2_golden_test.go`](method2_golden_test.go)

Один раз записуємо підтверджений людиною прогін у `evaldata/golden_happy_path.json`
(режим `EVAL_UPDATE_GOLDEN=1`), комітимо. Кожен наступний CI-прогін
порівнюється з золотим: послідовність викликів — exact match, фінальна
відповідь — ROUGE ≥ 0.85 (exact-match на тексті свідомо не робимо — крихко).

```bash
EVAL_UPDATE_GOLDEN=1 go test -run TestGoldenTrajectory .   # записати golden
go test -run TestGoldenTrajectory .                        # перевіряти назавжди
```

**Це і є trajectory eval у найчистішому вигляді:** оцінюємо ШЛЯХ, не лише
останнє слово. Золотий журнал = `evalset`-файл Python-світу, тільки як
Go-артефакт.

### Метод 3 — Live agent + LLM-as-judge (дорого, "розумність")

Файл: [`method3_judge_test.go`](method3_judge_test.go)

Справжній запуск агента на живій моделі (`pimodels`, каталог models.dev,
дефолт `gemini-3.7-flash`), потім суддя з рубрикою оцінює діалог і повертає
JSON `{"passed", "score", "explanation"}`. Вердикт судді валідовується схемою:
невалідний JSON — гучний fail тесту, а не тихе прохання. Без API-ключа або при
вичерпаній квоті тест **скіпається** (CI лишається зеленим).

**Що ловить:** погіршення якості після зміни промпта. **Не ловить:**
детермінованість — тому це третій шар, а не перший.

### Метод 4 — EvalSet runner (integration-grade, повний звіт)

Файли: [`evalset/refund.evalset.json`](evalset/refund.evalset.json),
[`method4_evalset_test.go`](method4_evalset_test.go), [`cmd/liveeval`](cmd/liveeval/main.go)

Набір кейсів у JSON-форматі за духом adk-python: `prompt`,
`expected_tool_calls` (+args), `expected_response`, `expected_verdict`,
пороги. Один прогін оцінює все і пише `artifacts/results_<ts>.json`:
 машины машино-читаний артефакт для CI + людино-читаний summary:

```
Eval Run Summary refund_desk_eval
model= tests passed=4 failed=0
  [PASSED] refund_happy_path traj=1.00 resp=0.96 verdict=success
  [PASSED] off_domain_merchant_refused traj=1.00 resp=1.00 verdict=policy_blocked
  [PASSED] missing_ids_asks_human traj=0.00 resp=0.57 verdict=needs_human
  [PASSED] second_merchant_in_registry traj=1.00 resp=1.00 verdict=success
```

Правило чесності: кожен FAILED **несе названу причину** у полі
`failure_cause` («trajectory 0.50 < 0.80», «verdict success проти очікуваного
policy_blocked») — безпричинний нуль гірший за помилку.

Живий варіант того самого раннера:

```bash
export GEMINI_API_KEY=...   # ключ моделі (models.dev каталог)
go run ./cmd/liveeval -set evalset/refund.evalset.json
```

## Запуск

```bash
cd demo/7_adk-go-evals
go test ./...        # усе, крім live-тестів (вони skip без GEMINI_API_KEY)
go vet ./...
go test -run TestLiveLLMJudge -v .   # необов'язково: живий judge (платні токени)
go run ./cmd/liveeval                # живий прогін evalset + results_*.json
```

## Дизайнерські рішення, які варто знати

1. **`TrajScore` — порядокозалежний**, як `tool_trajectory_avg_score`.
   Це означає ту саму крихкість: один «зайвий» коректний виклик ламає решту
   вирівнювання. У проді пороги опускають до 0.6–0.8 саме заради цього.
   Для строгих доменів використовуйте exact-match golden (Метод 2).
2. **ROUGE-F1 проти ADK-Python ROUGE-recall**: F1 штрафує вигадування слів,
   recall — ні. Для коротких відповідей агента F1 чесніший; якщо потрібна
   точна бітова сумісність з Python-числами — порахуйте recall замість F1
   (одна формула в `Rouge1F1`).
3. **Ключі економляться наперед**: тести на ScriptedModel їдуть у CI кожен
   коміт; live-judge і live-eval — ручні ворота перед мерджем промптових змін.
   Це та сама трирівнева піраміда, що й у звичайних тестах: unit → integration
   → manual/E2E.
4. **Normalize перед метриками**: пунктуація й регістр — шум для ROUGE.
   Без нормалізації той самий сенс дає розкид score ±0.15 і нестабильні пороги.
5. **Golden ≠ fixture**: golden-журнал — це *записана поведінка*, а не дані для
   запуску. Його змінюють тільки людським рішенням після ревʼю нового значення.

## Зв'язок з курсом

- Вебінар 8 (Fault-Tolerant Tool Harness): golden trajectory vs final answer —
  тут обидва у коді.
- Вебінар 11 (Multi-Critic): schema-validated verdict судді — той самий патерн.
- Вебінар 12 (Self-Improving Agents): `experiments.log` autoresearch — це
  результати серії прогонів `RunEvalSet`; однина метрики і бюджету природно
  лягає на `Report.Passed`/traj score як fitness.
- Оціночні межі курсу: `Course 2` бере на себе endpoint-grading через
  agents-cli/Vertex і failure-cluster analysis.
