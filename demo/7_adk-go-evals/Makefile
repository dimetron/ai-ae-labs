.PHONY: test vet fmt live clean

test: ## швидкі тести: ScriptedModel + golden + evalset (безкоштовно, ~2s)
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

live: ## живий прогін evalset на gemini-3.7-flash (потрібен GEMINI_API_KEY)
	go run ./cmd/liveeval

clean:
	rm -rf artifacts
