// Стартовий шаблон ДЗ 4 — станом на ADK Go v2.4.0 (вересень 2026), перевірте актуальність API.
//
// Запуск:
//
//	export GOOGLE_API_KEY=...
//	go run .
//	curl http://localhost:8080/health
//
// Основано на sources/github/adk-go/examples/rest/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/server/adkrest"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/geminitool"
)

func main() {
	ctx := context.Background()

	model, err := gemini.NewModel(ctx, "gemini-flash-latest", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// TODO(студент): замініть цього агента-заглушку на ВАШОГО агента з ДЗ 3
	// (workflowagent із графом prepare → tool → format).
	a, err := llmagent.New(llmagent.Config{
		Name:        "service_agent",
		Model:       model,
		Description: "Агент, обгорнутий у REST-сервіс.",
		Instruction: "ЗАМІНИ: сюди йде інструкція вашого агента з ДЗ 3.",
		Tools: []tool.Tool{
			geminitool.GoogleSearch{},
		},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	restServer, err := adkrest.NewServer(adkrest.ServerConfig{
		AgentLoader:     agent.NewSingleLoader(a),
		SessionService:  session.InMemoryService(),
		SSEWriteTimeout: 120 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to create REST API server: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", restServer))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	log.Println("Starting server on :8080")
	log.Println("API available at http://localhost:8080/api/")
	log.Println("Health check at http://localhost:8080/health")

	// TODO(студент): graceful shutdown —
	//  1) створіть http.Server{Addr: ":8080", Handler: mux};
	//  2) signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM);
	//  3) у горутині ListenAndServe, після <-ctx.Done() — server.Shutdown
	//     з таймаутом; активний запит має завершитися коректно.
	// TODO(студент): Dockerfile — multi-stage, CGO_ENABLED=0, фінальний
	// образ FROM scratch (+ ca-certificates для HTTPS до Gemini API).
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
