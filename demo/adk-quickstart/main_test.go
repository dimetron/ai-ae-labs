package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/dimetron/ai-eng-course/demo/adk-quickstart/internal/utils"
	"github.com/dimetron/pi-go/pimodels"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

func TestNewAgentMissingKeyReport(t *testing.T) {
	ctx := context.Background()
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	_, err := NewAgent(ctx, "claude-sonnet-5")
	if err == nil {
		t.Fatal("очікувалась помилка через відсутність API ключа для claude, але отримано nil")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("помилка повинна підказувати змінну ANTHROPIC_API_KEY: %v", err)
	}
}

func TestNewAgentSuccessMultiProvider(t *testing.T) {
	ctx := context.Background()
	t.Setenv("GEMINI_API_KEY", "fake-gemini-key")
	t.Setenv("ANTHROPIC_API_KEY", "fake-anthropic-key")
	t.Setenv("OPENAI_API_KEY", "fake-openai-key")

	// Тільки моделі з .env.example
	modelsToTest := []struct {
		model string
	}{
		{"gemini-3.7-flash"},
		{"claude-haiku-4.5"},
		{"claude-sonnet-5"},
		{"gpt-5.6-luna"},
		{"gpt-4o"},
		{"ollama/deepseek-v4-flash:0731"},
		{"ollama/llama3"},
	}

	for _, tt := range modelsToTest {
		t.Run(tt.model, func(t *testing.T) {
			a, err := NewAgent(ctx, tt.model)
			if err != nil {
				t.Fatalf("не вдалося створити агента для %s: %v", tt.model, err)
			}
			if a == nil {
				t.Fatal("отримано nil замість екземпляра агента")
			}
			if a.Name() != agentName {
				t.Errorf("ім'я агента = %q, очікувалось %q", a.Name(), agentName)
			}
		})
	}
}

func newLocalTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping test due to local listener restriction: %v", err)
		return nil
	}
	ts := httptest.NewUnstartedServer(handler)
	ts.Listener = l
	ts.Start()
	return ts
}

func TestProvidersOfflineExecution(t *testing.T) {
	// Тільки моделі з .env.example
	tests := []struct {
		name     string
		model    string
		envKey   string
		handler  func(w http.ResponseWriter, r *http.Request)
		wantText string
	}{
		{
			name:   "gemini",
			model:  "gemini-3.7-flash",
			envKey: "GEMINI_API_KEY",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[
					{"text":"DeepSeek V4 Flash має контекстне вікно 1M токенів та низьку вартість."}]},"finishReason":"STOP"}]}`)
			},
			wantText: "DeepSeek",
		},
		{
			name:   "openai",
			model:  "gpt-4o",
			envKey: "OPENAI_API_KEY",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"id":"chatcmpl-123","object":"chat.completion","created":1677652288,"choices":[
					{"index":0,"message":{"role":"assistant","content":"Claude Opus 4.7 від Anthropic є однією з найпотужніших reasoning моделей."},"finish_reason":"stop"}]}`)
			},
			wantText: "Anthropic",
		},
		{
			name:   "anthropic",
			model:  "claude-haiku-4.5",
			envKey: "ANTHROPIC_API_KEY",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"id":"msg_123","type":"message","role":"assistant","model":"claude-haiku-4.5","content":[
					{"type":"text","text":"Для кодингу рекомендую GPT-5.5 або Kimi K2.7 Code."}],"stop_reason":"end_turn"}`)
			},
			wantText: "Kimi",
		},
		{
			name:   "ollama",
			model:  "ollama/deepseek-v4-flash:0731",
			envKey: "OLLAMA_API_KEY",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"model":"deepseek-v4-flash:0731","message":{"role":"assistant","content":"Каталог містить понад 6000 моделей від 190+ провайдерів."},"done":true}`)
			},
			wantText: "моделей",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newLocalTestServer(t, http.HandlerFunc(tt.handler))
			defer srv.Close()

			t.Setenv(tt.envKey, "fake-test-key")
			if tt.name == "gemini" {
				t.Setenv("GOOGLE_GEMINI_BASE_URL", srv.URL)
			}

			ctx := context.Background()
			a, err := NewAgent(ctx, tt.model, pimodels.WithBaseURL(srv.URL))
			if err != nil {
				t.Fatalf("NewAgent: %v", err)
			}

			r, err := runner.New(runner.Config{
				AppName:           "test-" + tt.name,
				Agent:             a,
				SessionService:    session.InMemoryService(),
				AutoCreateSession: true,
			})
			if err != nil {
				t.Fatalf("runner.New: %v", err)
			}

			msg := genai.NewContentFromText("Порадь модель", genai.RoleUser)
			var gotText string
			for event, runErr := range r.Run(ctx, "test_user", "test_session", msg, agent.RunConfig{}) {
				if runErr != nil {
					t.Fatalf("помилка виклику агента: %v", runErr)
				}
				if event != nil && event.LLMResponse.Content != nil {
					for _, p := range event.LLMResponse.Content.Parts {
						gotText += p.Text
					}
				}
			}

			if !strings.Contains(gotText, tt.wantText) {
				t.Errorf("відповідь (%s) = %q, очікувалось входження %q", tt.name, gotText, tt.wantText)
			}
		})
	}
}

func TestLoadDotEnv(t *testing.T) {
	t.Setenv("TEST_DOTENV_KEY", "")
	_ = os.Unsetenv("TEST_DOTENV_KEY")

	utils.LoadDotEnv()
}
