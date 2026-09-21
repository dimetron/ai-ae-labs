# Налаштування MacBook

## 1. Homebrew — менеджер пакетів

Сторінка: <https://brew.sh/>

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

> Команда ставить Homebrew і попросить пароль адміністратора та підтвердження
> клавішею Enter. Вставляйте її **цілком**, одним рядком: розбитий на два рядки
> `curl` завантажить не той скрипт.

## 2. Інструменти розробника

### Go

```bash
brew install go
brew install git
brew install gh
```

`gh` — офіційний CLI GitHub; він знадобиться, щоб здавати домашні завдання
(`gh auth login` один раз).

### Пошук та інструменти для агентів

```bash
brew install ripgrep    # rg — швидкий рекурсивний пошук
brew install rtk        # проксі-CLI, що оптимізує токени для LLM-агентів
brew install go-task    # task — запуск команд репозиторію (див. §4)
```

`rtk` обгортає звичні команди (`git`, `ls`, `read`, `test`, …) і стискає їхній
вивід, перш ніж той потрапить у контекст моделі, — та сама робота коштує менше
токенів.

### Docker — виберіть **один** варіант

```bash
brew install --cask docker-desktop

# альтернативи
brew install --cask orbstack
brew install --cask rancher
```

Ставте лише один із трьох: усі вони займають той самий сокет Docker, і кілька
одночасно — це конфлікти, а не більше можливостей.

| Варіант | Коли варто брати |
|---|---|
| `docker-desktop` | Звичний UI і найбільше документації. Важчий за інші: окрема віртуальна машина й помітне споживання пам'яті. |
| `orbstack` | Швидший і ощадливіший на Apple Silicon. Найкращий вибір для щоденної роботи з Go. |
| `rancher` | Якщо потрібен саме Kubernetes у комплекті, а не лише Docker. |

### Клієнти для інфраструктури

```bash
brew install kubernetes-cli    # kubectl
brew install helm
brew install libpq             # psql (клієнт, без сервера БД)
```

Формула `kubectl` — це alias: у Homebrew пакет зветься `kubernetes-cli`, і
`brew install kubectl` ставить саме його (`1.37.x`). Обидва варіанти коректні.

> **`libpq` — keg-only, і це найпоширеніша пастка.** Homebrew **не** додає його
> бінарники в `/opt/homebrew/bin`, бо вони конфліктують із повноцінним
> PostgreSQL. Тому після `brew install libpq` команда `psql` **не** з'явиться в
> `PATH`, і це виглядає як невдала установка. Додайте шлях у `~/.zshrc`:
>
> ```bash
> echo 'export PATH="/opt/homebrew/opt/libpq/bin:$PATH"' >> ~/.zshrc
> source ~/.zshrc
> psql --version   # тепер працює
> ```
>
> Це саме клієнт `psql` — без сервера. Для лабораторій цього достатньо: база
> (PostgreSQL для agentgateway, див. нижче) працює **в контейнері**, і з хоста до
> неї ходять як до будь-якої віддаленої БД. Локальний сервер Postgres на macOS
> не потрібен і не потрібно його ставити.

### Де це насправді знадобиться: база agentgateway

agentgateway уміє зберігати **логи запитів** і **стан конфігурації UI** в базу.
Логіка вибору — за URL (з конфігурації, поле `database.url`):

| `database.url` | Що буде використано |
|---|---|
| `postgres://…` або `postgresql://…` | PostgreSQL |
| будь-яке інше значення | SQLite (файл) |

Тобто для лабораторій ми піднімаємо **PostgreSQL у Docker поруч із
agentgateway** (в одному `docker-compose.yml`), а з хоста підключаємось
`psql` — саме тому потрібен `libpq`, і саме тому сервер Postgres на macOS
не потрібен.

Зверніть увагу на версію: у `demo/1_ai-gateway/docker-compose.yml` зараз
запінений `agentgateway:v1.4.1`. Поля `database` і `storage` я звірив на
встановленому образі `v1.5.0` — **перед заняттям перевірте, що ваш пінений тег
їх підтримує** (`strings` або документація релізу), бо схема конфігу в цьому
проєкті змінюється між мінорними версіями.

## 3. rtk: підключення до вашого AI-інструмента

`brew install rtk` лише ставить бінарник — він нічого не змінює. Щоб команди
почали переписуватися автоматично, треба виконати `rtk init` **для того
інструмента, яким ви користуєтесь**.

Повний перелік інструментів і команда для кожного —
[rtk-ai/rtk § Quick Start](https://github.com/rtk-ai/rtk#quick-start).
Для цього курсу достатньо двох:

```bash
rtk init -g              # Claude Code (типовий варіант)
rtk init -g --gemini     # Gemini CLI
```

> **Це не два кроки поспіль, а вибір.** Перша команда налаштовує Claude Code,
> друга — Gemini CLI. Виконайте ту, що відповідає вашому інструментові. Якщо
> користуєтесь обома — виконайте обидві: вони пишуть у різні теки й не
> конфліктують.

Прапорець `-g` означає «глобально»: налаштування йдуть у домашню теку
(`~/.claude/`, `~/.gemini/`), а не в поточний проєкт. Тому вони діють у всіх
репозиторіях одразу, і повторювати команду для кожної лабораторної не потрібно.

### Що робить `rtk init`

1. Ставить хук, який переписує команди Bash **до** їх виконання:
   `git status` виконується як `rtk git status`. Агент отримує вже стиснутий
   вивід і навіть не знає, що викликав `rtk`.
2. Додає `RTK.md` з інструкціями для агента.

Після `rtk init` **перезапустіть свій AI-інструмент** — інакше хук не
підхопиться.

### Важливе обмеження

Хук спрацьовує лише на викликах **Bash**. Вбудовані інструменти Claude Code
(`Read`, `Grep`, `Glob`) повз нього не проходять, тож автоматичного переписування
там немає. Щоб отримати стиснутий вивід і для цих операцій, користуйтесь
shell-командами (`cat`/`head`/`tail`, `rg`/`grep`, `find`) або викликайте
`rtk read`, `rtk grep`, `rtk find` напряму.

### Перевірка

```bash
rtk --version    # має показати rtk 0.49.x або новіше
rtk init --show  # показує, що саме налаштовано
rtk gain         # панель зекономлених токенів
```

Якщо `rtk gain` падає — у вас інший пакет із такою ж назвою (на crates.io є
проєкт «Rust Type Kit»). Ставте саме через Homebrew, як вище.

## 4. `task` — команди репозиторію

У репозиторії є `Taskfile.yml`: він обгортає сирі `go`-команди, щоб усе, що
перевіряє CI, виконувалось однією командою, а не збиралося щоразу з пам'яті.

```bash
brew install go-task
```

> **Формула зветься `go-task`, а не `task`.** `brew install task` — інший пакет
> (планувальник завдань), і він не має нічого спільного з Taskfile. Перевірте:
> `task --version` має показати `3.53.x` або новіше.

### Основні команди

```bash
task                             # список усіх задач (запуск без аргументів)
task check                       # fmt:check + vet + build + test + покриття
task build                       # go build ./... (головний модуль)
task build:all                   # + вкладені модулі demo/* та examples
task test                        # go test -race ./...
task test:short                  # без -race (швидше)
task cover                       # покриття по пакетах проти порогу 85 %
task run PKG=./week1/Day1_Models_and_Frameworks_Landscape/labs
```

`task check` — це та команда, яку варто запускати **перед кожним комітом**: вона
робить рівно те саме, що й CI.

### Дві речі, які варто знати

**`go build ./...` не бачить вкладені модулі.** `demo/1_ai-gateway/mock-llm`,
`demo/3_ai-knowledge-graph-go`, `demo/adk-quickstart`, `demo/7_adk-go-evals` та
`examples/` — це окремі Go-модулі, і `./...` їх мовчки пропускає. Для них є
`task build:all`.

**Деякі задачі потребують змінної.** `task run`, `task test:pkg`,
`task cover:changed` та `task cover:html` без `PKG=…` не запустяться й скажуть,
чого бракує:

```bash
task test:pkg PKG=./week1/...        # тести одного пакета
task cover:changed PKG=week1/Day1    # поріг покриття лише для цієї теки
```

### Перевірка

```bash
task --version   # 3.53.x або новіше
task --list      # перелік задач
```

`task` шукає `Taskfile.yml` у поточній теці й **угору по батьківських**, тож
запускати його можна і з середини репозиторію. Якщо він каже
`No Taskfile found` — ви поза репозиторієм (наприклад, у `$HOME`).

## 5. Перевірка оточення

```bash
go version              # очікується go1.27 або новіше
git --version
gh --version
rg --version
rtk --version
task --version          # 3.53.x або новіше
docker version          # має показати і Client, і Server
kubectl version --client
helm version --short
psql --version          # якщо порожньо — див. пастку з keg-only вище
```

Якщо `docker version` друкує Client, але падає на Server — застосунок Docker
встановлено, але не запущено. Відкрийте його один раз із Applications.

Одна команда, щоб побачити, чого не бракує (`— НЕ ЗНАЙДЕНО` = немає):

```bash
for t in go git gh rg rtk task docker kubectl helm psql gitleaks; do
  printf '%-10s %s\n' "$t" "$(command -v $t || echo '— НЕ ЗНАЙДЕНО')"
done
```

Далі — перевірка самого репозиторію:

```bash
task build   # усе збирається без ключів і без мережі
task test    # тести проходять
```
