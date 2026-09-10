// mock-llm is a tiny OpenAI-compatible chat-completions server used by the
// agentgateway demo. It returns a canned response and echoes the requested
// model name so the smoke test can verify which backend served a request.
//
// It deliberately implements only the two endpoints the demo needs:
//   - POST /v1/chat/completions  (OpenAI-compatible, non-streaming)
//   - GET  /healthz              (used by the smoke test readiness check)
//
// Model "mock-fail" always returns HTTP 500 so the demo can prove that
// agentgateway's failover routing falls through to a healthy backend.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type chatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type chatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []choice `json:"choices"`
	Usage   usage    `json:"usage"`
}

type choice struct {
	Index        int     `json:"index"`
	Message      message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Simulate a failing backend for the failover demo.
	if req.Model == "mock-fail" {
		writeError(w, http.StatusInternalServerError, "simulated backend failure for model mock-fail")
		return
	}

	// Rough prompt size so the gateway has real token numbers to price.
	promptTokens := 0
	for _, m := range req.Messages {
		promptTokens += len(strings.Fields(m.Content))
	}
	if promptTokens == 0 {
		promptTokens = 7
	}
	completionTokens := 12

	content := fmt.Sprintf("Hello from mock-llm! You asked for model %q.", req.Model)

	resp := chatResponse{
		ID:      "chatcmpl-mock-" + fmt.Sprint(time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []choice{{
			Index:        0,
			Message:      message{Role: "assistant", Content: content},
			FinishReason: "stop",
		}},
		Usage: usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": msg,
			"type":    "invalid_request_error",
		},
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", handleChat)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	log.Printf("mock-llm listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
