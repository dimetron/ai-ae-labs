# Домашнє завдання 6 — Agentic GraphRAG: hybrid retrieval, multi-hop planning, re-ranking, semantic cache та eval harness

> **Станом на 08/2026 (перед використанням потрібно перевірити):**
> - ADK Go v2.2.0 (`google.golang.org/adk/v2`) — сигнатури `workflow.NewEmittingFunctionNode`, `workflow.NewJoinNode`, `workflow.NewEdgeBuilder` (`AddFanOut`/`AddFanIn`) відповідають прикладам у `sources/github/adk-go/examples/workflow/`.
> - **Go 1.27** — `go`-директива модуля лаб (`courses/AI_Agents_Engineering/lectures/go.mod`, звірено 27.08.2026). Не плутати з `go 1.26.5` у `go.mod` самого ADK v2.2.0: то мінімум для залежності, а не версія, яку має мати студент.
> - Reranker `BAAI/bge-reranker-v2-m3` — приклад-кандидат, не вимога.
> - Поріг semantic cache `0.92` — стартова гіпотеза, не константа; калібрується на власному корпусі.

## Легенда

Минулого разу (ДЗ 5) ми зробили пошуковий **індекс**: lossless PDF-парсер, ontology-driven chunking, entity resolution і синхронізовані vector + graph представлення для архіву LEDGERWORKS — 5 000 PDF: звіти PCI DSS, vendor SOC2 підрядників і договори з еквайрами. Але індекс без **розумного retrieval-шару** — це мертвий вантаж. Оксана ставить запит, яким ми закінчили минулий вебінар: *«які мерчанти на тарифі T-2 підпадають під вимогу НБУ 2026 і хто підписував їхні договори?»*, і наївний vector-RAG повертає одну з двох половин відповіді, бо не вміє поєднувати структурний фільтр (тариф + вимога НБУ) із семантичним пошуком і graph-traversal `Merchant → Contract → Signer`.

Цього разу ми додаємо **мозок**: агент-планувальник, який сам вирішує, коли використати vector retriever, коли — graph filter, коли зробити ще один hop; re-ranker як спільний шар оцінки доказів; семантичний кеш для повторюваних запитів юристів; і **eval harness**, який вимірює, чи стало справді краще за vector-RAG baseline. Це — пошукова половина **Макро-артефакту №2** курсу — `Agentic GraphRAG Engine`. Наступний місток — event-sourced operational state і LLM-Wiki / OKF у Вебінарі 9; фінальний capstone успадкує цей engine як knowledge substrate з provenance, budget та eval, а не будуватиме retrieval наново.

## Ранній win: перший видимий результат за ≤15 хвилин

Стартер працює **без API-ключа і без завершеного ДЗ 5**:

1. `go version` — потрібен **Go 1.27+** (`go`-директива в `courses/AI_Agents_Engineering/lectures/go.mod`) → `go mod tidy` — без помилок.
2. `go run . console` і подайте запит *«Що таке тариф T-2?»*.
3. **Checkpoint:** у консолі — відповідь вузла `answer` із provenance: `[cache_hit=false] Топ-результат (todo-1): TODO: реальні кандидати з вашого корпусу`. Повний шлях `search → rerank → answer` видно в event log і тестах, а не в консольному виводі. Замініть `TODO`-кандидат у `search` на текст із прикладу — відповідь отримає реальний зміст.

4. `go test ./...` — теж **зелено з коробки**. У стартері є `main_test.go`: фейк-контекст на `agent.NewStrictContextMock`, табличні тести на `search`, наскрізний `TestPipeline_EarlyWin` (перевіряє, що у відповіді є `chunk_id`, тобто provenance) і `TestCache_ExactHit`.

**Checkpoint:** `ok … 5 passed, 2 skipped`. Два пропущені — це **критерії оцінювання**, а не заготовки: `TestRerank_ChangesOrder` (15 балів) і `TestCache_ParaphraseHits` (10 балів). Знімайте `t.Skip` тоді, коли берете відповідний пункт, і дозвольте тесту падати — це ваша специфікація. Пара «до/після» з першого з них іде просто в README як один із п'яти обов'язкових прикладів.

Повний індекс із ДЗ 5 знадобиться лише для multi-hop частини (пункти 2+ нижче).

## Основне завдання

**Ваше завдання —** завершити `Agentic GraphRAG Engine` (Макро-артефакт №2) на корпусі з ДЗ 5, стартуючи з [courses/AI_Agents_Engineering/lectures/week3/Day6_Reranking_Semantic_Cache_Maturity_Ladder/labs/main.go](labs/main.go):

1. **Агент-планувальник як workflow-граф.** Зберіть конвеєр `Start → classify → (decompose) → [vector_search | graph_filter | graph_traversal] → JoinNode → rerank → format` через `workflow.NewEdgeBuilder` із `AddFanOut` / `AddFanIn`. Використайте `workflow.NewEmittingFunctionNode` для класифікатора (`single-hop` / `multi-hop` / `global` / `metadata-filter`) і `workflow.NewJoinNode` для збору результатів із кількох гілок (еталонний патерн — `sources/github/adk-go/examples/workflow/complex/`). Обов'язковий **budget retrieval-кроків (3–5)** захищає від нескінченного планування: terminal `FunctionNode` повідомляє «недостатньо доказів», якщо немає набору доказів із provenance за локально відкаліброваним критерієм.

2. **Multi-hop запити з provenance.** Запит Оксани (`тариф T-2, НБУ 2026, хто підписав`) має повернути **обидва** речення з минулого тижня (список мерчантів на T-2 + підписант по кожному) із посиланнями на конкретні `chunk_id` / `document_id` / `span`. Підзапити йдуть до різних retriever-ів (`vector` для семантики, `graph_filter` для тарифу й вимоги НБУ, `graph_traversal` для зв'язку Contract → Signer) і об'єднуються через `JoinNode` у `map[nodeName]RetrievalResult` перед `rerank`.

3. **Re-ranking як спільний шар.** Інтегруйте мультимовний cross-encoder reranker (наприклад, `BAAI/bge-reranker-v2-m3`; перевірте model card, ліцензію та input schema) у конвеєр: перший етап — vector/graph retriever повертає top-K (наприклад, K=20); другий — reranker переранжує до top-N (N=3). Задокументуйте **5 запитів, де reranking змінив топ-результат** (таблиця «до/після» з groundedness та top-1 relevance) і як ви відкалібрували quality gate на своєму gold set.

4. **Семантичний кеш.** Додайте вузол `cache_check` на початку графа: cosine-similarity між embedding запиту й embedding кешованих запитів. Поріг оберіть і обґрунтуйте на своєму corpus/gold set; `0.92` може бути стартовою гіпотезою, але не є вимогою. Ключ має ізолювати tenant, permission/policy boundary, версію корпусу та TTL. **Hit** → відповідь із provenance; **miss** → повний конвеєр. Продемонструйте в README: один запит → miss; перефразований (*«скільки коштує X?»* ≈ *«яка ціна X?»*) → hit; виміряйте exact-hit, semantic-hit і miss. Поясніть, чи кешуєте фінальну відповідь або проміжні `RetrievalResult`, і як інвалідуєте записи після оновлення корпусу.

5. **Retrieval eval harness.** Зберіть набір із **20 запитів** (10 single-hop, 10 multi-hop) і проженіть кожен через:
   - **Vector-RAG baseline** — один embeddings-пошук top-5 → LLM-відповідь.
   - **Agentic GraphRAG** — ваш сьогоднішній агент.

   Для кожного запиту заміряйте: `top1_relevance` (1-5, людська оцінка), `groundedness` (чи відповідь підтверджується чанками), `latency_p95`, `tokens_used`. Підсумкова таблиця — у README, з **обґрунтуванням**, на яких типах запитів Agentic виграє, а де vector-RAG достатній.

6. **Decision sheet.** У README заповніть односторінковий decision sheet «vector vs GraphRAG vs agentic» для вашого конкретного корпусу: на якому щаблі **Retrieval Maturity Ladder** стоїть задача, чому, і який наступний щабель розглядатимете, якщо query patterns зміняться.

## Альтернативні теми (на вибір)

Та сама механіка (planner → fan-out vector/graph → JoinNode → rerank → cache → eval), інший корпус:

- **Security:** пошук по базі CVE-описів вашого стеку — «які вразливості стосуються нашої версії Postgres?» з re-ranking за релевантністю версії та multi-hop через зв'язок CVE → affected_versions → fixed_in. Окремий інтерес: graph traversal від CVE до vendor advisories.
- **Research:** пошук по конспектах статей із ДЗ 5 — багатослівні дослідницькі питання, де перший кандидат рідно найкращий, і де multi-hop через citations/cocitations працює краще за чистий vector.
- **Fun:** пошук по книзі рецептів — *«щось швидке без духовки з куркою»* (запит, який ламає naive keyword search, але з графом «інгредієнт → страва → час приготування» — працює). Тут особливо добре видно роль multi-hop.

## ++ Advanced (до 20 балів, для сеньйорів)

- **Паралельний гібрид через fan-out:** keyword-гілка (BM25) + vector-гілка через `AddFanOut`, злиття результатів через `JoinNode` і RRF (Reciprocal Rank Fusion) перед re-rank. Еталонний патерн fan-out/JoinNode — `sources/github/adk-go/examples/workflow/complex/main.go` (дослідники → gather → format).
- **Eval harness із grounding-автоматикою:** замість ручної оцінки groundedness — LLM-суддя, який перевіряє, чи кожне твердження відповіді має span-посилання в чанках. Запишіть precision/recall grounding-у для 20 запитів.

## 🔥 Бонус-трек (не оцінюється, не потрібен для сертифіката)

**Adaptive planner із budget/latency SLO.** Реалізуйте `AdaptiveConfig{MaxTokens, MaxLatency, MinScore}`: зупиняйтеся, коли локально відкалібрований quality gate підтверджує достатні докази з provenance, або коли `token_counter > MaxTokens`. Запустіть eval harness і порівняйте сумарну вартість та groundedness із/без адаптивного планувальника. Балів не дає; робиться для себе.

## Якщо щось не працює

- **Multi-hop запит не запускає обидві гілки.** Перевірте `EdgeBuilder`: маршрут `multi_hop` має вести через `AddRoute(classify, plan, StringRoute("multi_hop"))` до `AddFanOut(plan, vectorNode, graphNode)` + `AddFanIn(gather, …)`. В event log для одного `invocation_id` мають бути і `node_enter: vector_search`, і `node_enter: graph_query`. Якщо є лише vector — класифікатор емітить `vector_only` (перевірте, що розпізнаються і рік, і вендор).
- **Re-ranker не змінює top-3.** Переконайтеся, що в пул перед re-rank зливаються кандидати з **усіх** гілок, а не лише з vector. Якщо пул правильний, а порядок не міняється — перевірте модель (мультимовність, input schema) і калібрування порога на власному gold set.
- **Кеш завжди miss (або навпаки — хибні hit).** Поріг `0.92` — стартова гіпотеза: занизький дає false-positive на різних за змістом запитах, зависокий не ловить перефразування. Виміряйте false-positive rate на 5–10 парах перефразувань зі свого корпусу і скоригуйте.
- **Немає API-ключа для LLM-декомпозитора.** Це не блокер: у ДЗ 6 декомпозицію дозволено реалізувати rule-based парсером (regex на «тариф T-2, НБУ 2026, хто підписав») — LLM-вузол лишіть як production-опцію.
- **Немає завершеного ДЗ 5 (індексу).** Ранній win і single-hop гілка працюють на вбудованих кандидатах стартера; для multi-hop частини доробіть мінімальний індекс із ДЗ 5 на 2–3 документах — повний корпус не обов'язковий.

## Відомі прогалини курсу (follow-up)

- **LLM-Wiki / OKF** як третя нога KB-модуля (поряд із taxonomy/ontology і hybrid retrieval) — закрита у Вебінарі 9 (context engineering). На цьому тижні ми не зобов'язані її реалізовувати, але decision sheet має згадати, що вона існує і куди дивитися.
- **FalkorDB GraphRAG-SDK** — репозиторій є, але в нашій вікі позначений як `gated pending raw`. У ДЗ не вимагається підключати FalkorDB як залежність. Якщо вирішите використати — окремо зафіксуйте це в README як «експериментальне».

## Критерії оцінювання

| Критерій | Бали |
|---|---|
| Робочий agentic retrieval-граф: planner → fan-out (vector + graph) → JoinNode → rerank → format, з обов'язковим budget limit | 25 |
| Multi-hop запит Оксани повертає обидва речення з provenance-посиланнями | 10 |
| Re-ranking з документованим ефектом (5 запитів «до/після» + groundedness) | 15 |
| Семантичний кеш із продемонстрованим hit на перефразованому запиті, обґрунтуванням «що кешуємо» | 10 |
| Eval harness на 20 запитах із таблицею vector-RAG vs Agentic за 4 метриками | 15 |
| Decision sheet: щабель Retrieval Maturity Ladder з обґрунтуванням + README | 5 |
| ++ Advanced (бонус) | 20 |
| **Разом** | **100** |

Базове завдання дає до 80 балів; ++ Advanced — це **+20 додаткових балів** (разом 100). 🔥 Бонус-трек не оцінюється взагалі. Альтернативна тема оцінюється за тими самими критеріями. **Базового завдання достатньо для сертифіката; «🔥 Бонус» ніколи не є його умовою.** Бали за всі 12 завдань підсумовуються: від 60 % відкривається генерація сертифіката, від 85 % — з відзнакою.

## Формат здачі

GitHub-репозиторій з кодом, eval-таблицями та README; `go build ./...` має проходити; `go test ./...` — зелений (тести є у стартері з коробки, тож «якщо є» більше не застосовується: два `t.Skip` мають бути зняті й зелені, бо це критерії на 25 балів разом). README має містити: (а) decision sheet з обґрунтуванням щабля, (б) таблицю 5 запитів «до/після» rerank, (в) демонстрацію cache hit, (г) eval-таблицю 20 запитів із чотирма метриками. Посилання на репозиторій — у форму здачі. Якщо репозиторій закритий — додайте акаунт ментора в collaborators (акаунт указано в інструкції до курсу на платформі).

Стартовий шаблон: [courses/AI_Agents_Engineering/lectures/week3/Day6_Reranking_Semantic_Cache_Maturity_Ladder/labs/main.go](labs/main.go) · Еталонні приклади: [`sources/github/adk-go/examples/workflow/basic/`](https://github.com/google/adk-go/blob/v2.2.0/examples/workflow/basic/main.go), [`sources/github/adk-go/examples/workflow/routing/string/`](https://github.com/google/adk-go/blob/v2.2.0/examples/workflow/routing/string/main.go) (для класифікатора), [`sources/github/adk-go/examples/workflow/complex/`](https://github.com/google/adk-go/blob/v2.2.0/examples/workflow/complex/main.go) (для fan-out + JoinNode у ++ Advanced) · Wiki-опори: [hybrid retrieval](https://localaimaster.com/blog/reranking-cross-encoders-guide), [CacheRAG / semantic caching](https://futureagi.com/blog/what-is-semantic-caching-llms-2026). FalkorDB GraphRAG-SDK — case study для слайда, **не залежність** для коду.

## Відео, якщо застрягли (не обов'язкове, посилання звірені 27.08.2026)

Повний список із поясненнями — у `Lecture.md`, розділ «Відео-опори». Чотири найкорисніші саме під це ДЗ:

- **Пункт 6, decision sheet — не знаєте, як обґрунтувати щабель** — https://www.youtube.com/watch?v=w9u11ioHGA0. Автор розкладає ті самі техніки за осями складність × вплив і дає рамку «baseline → loss analysis → вибір за complexity-adjusted impact». Це буквально форма вашого decision sheet.
- **++ Advanced, RRF — не розумієте, чому не можна просто скласти скори** — https://www.youtube.com/watch?v=4Xe_iMYxBQc. Звідти видно, чому BM25 узагалі не має верхньої межі (TF, IDF, field-length norm), тобто чому сума з косинусом мовчки віддає перемогу одному retriever-у.
- **Пункт 5, eval — чотирьох метрик мало** — https://www.youtube.com/watch?v=wRJD0inpmjU. Три reference-free метрики, які варто додати: answer completeness, document relevance, hallucination detection. І виміряний trade-off: вища повнота відповіді збільшує поверхню для галюцинацій.
- **Пункт 4, кеш — плутаєте його з кешем провайдера** — https://www.youtube.com/watch?v=SkM4k4SKvCM. Це запис про **інший** кеш (prompt/KV: вхідні токени, хвилини життя, вмирає від таймстемпа в system prompt). Подивіться саме щоб побачити, чим він не є вашим семантичним кешем.

**Українською**, якщо хочеться спершу почути поняття рідною мовою: https://www.youtube.com/watch?v=e24YTos57y8 (fwdays, 13.08.2026) — naive RAG → hybrid → corrective → GraphRAG.

> Цифри, названі спікерами (приріст якості, hit-rate, латентність), — заявлені ними, не відтворені нами. У README вони не замінюють ваш власний eval: сенс пункту 5 саме в тому, щоб виміряти на **своєму** корпусі.

## Дедлайн

Два тижні після дати відкриття завдання.

Шановні слухачі!

Дедлайн надсилання розв'язку на перевірку — 23 год. 59 хв. 16.10.2026 (це крайній термін,
рекомендуємо виконати і надіслати розв'язок протягом тижня після відкриття завдання).

**Інструкція до виконання завдання:**

До 23 год. 59 хв. 16.10.2026 року додайте отримані результати виконання завдання в наступному форматі.

Посилання на репозиторій з виконаним завданням з доданим акаунтом автора до collaborators
https://github.com/dimetron — активне посилання.

Натисніть кнопку «Надіслати відповідь та перейти до наступного етапу». Переконайтесь, що з'явився
напис «Чекаємо оцінки викладача».

Очікуйте на оцінку — згодом вона з'явиться під відповіддю у розділі «Оцінка викладача».

**Зверніть увагу!** Надіслати завдання вдруге неможливо — у вас є лише одна спроба.

Слухачам, які служать у ЗСУ, дедлайн продовжуємо за окремим запитом.

