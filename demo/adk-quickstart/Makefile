# Демо Google ADK Go v2 Quickstart
#
# Файл .env підвантажується автоматично, якщо він існує.
ifneq (,$(wildcard .env))
include .env
export
endif

.DEFAULT_GOAL := help

.PHONY: help
help: ## показати список доступних команд
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk -F':.*?## ' '{printf "  make %-15s %s\n", $$1, $$2}'

.PHONY: run
run: ## запустити агента в інтерактивному консольному режимі
	go run .

.PHONY: run-console
run-console: ## запустити в консолі (явно вказати console)
	go run . console

.PHONY: run-web
run-web: ## запустити агента з веб-інтерфейсом (Web UI на http://localhost:8080/ui/ + REST API на /api)
	go run . web webui api

.PHONY: login-codex
login-codex: ## увійти через OpenAI Codex Device Flow (без потреби ручного API-ключа)
	go run ./cmd/login codex

.PHONY: login
login: ## увійти через OAuth/Device flow (PROV=codex|opencode)
	go run ./cmd/login $(if $(PROV),$(PROV),codex)

.PHONY: build
build: ## скомпілювати бінарні файли у bin/
	@mkdir -p bin
	go build -o bin/adk-quickstart .
	go build -o bin/login ./cmd/login

.PHONY: test
test: ## запустити всі тести
	go test -v ./...

.PHONY: vet
vet: ## перевірити код за допомогою go vet
	go vet ./...

.PHONY: verify
verify: ## повна перевірка перед комітом (форматування, vet, тести, компіляція)
	@echo "==> Перевірка форматування gofmt..."
	@test -z "$$(gofmt -l .)" || (echo "Файли потребують форматування gofmt:" && gofmt -l . && exit 1)
	@echo "==> Запуск go vet..."
	go vet ./...
	@echo "==> Запуск тестів..."
	go test -v ./...
	@echo "==> Компіляція бінарників..."
	@mkdir -p bin
	go build -o bin/adk-quickstart .
	go build -o bin/login ./cmd/login
	@echo "==> Все успішно перевірено!"
