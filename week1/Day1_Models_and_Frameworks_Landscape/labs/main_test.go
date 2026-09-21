package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dimetron/pi-go/pimodels"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// TestPromptExecution executes a live prompt test if a model name and its
// corresponding environment API credentials are found, or skips cleanly when
// offline.
//
// This is the only test in the package that may touch the network, and it skips
// rather than fails: a missing key, a quota error and an unreachable gateway are
// all "not my environment", not "broken code". The offline path stays green so
// `go test ./...` never needs a credential.
//
// Provider resolution — the part that decides WHICH backend this exercises — is
// tested offline in provider_test.go.
func TestPromptExecution(t *testing.T) {
	loadEnv()

	choice, err := chooseModel(os.Getenv("MODEL"), os.Getenv("DEFAULT_MODEL_PROVIDER"))
	if err != nil {
		t.Skipf("skipping prompt test: %v", err)
	}
	modelName := choice.Model

	// Verify the required credential exists before attempting a live call.
	// Resolve is the authority on which provider a name belongs to, so ask it
	// rather than re-deriving the answer from the name here.
	info, err := pimodels.Resolve(modelName)
	if err != nil {
		info, err = pimodels.Resolve("ollama/" + modelName)
	}
	if err == nil && !info.Ollama && !isAgentGateway(info.Provider) {
		envVar := pimodels.APIKeyEnvVar(info.Provider)
		hasKey := os.Getenv(envVar) != ""
		if info.Provider == "gemini" && (hasKey || os.Getenv("GOOGLE_API_KEY") != "") {
			hasKey = true
		}
		if !hasKey {
			t.Skipf("skipping prompt test: model %q requires %s in environment", modelName, envVar)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m, err := createModel(ctx, choice.Provider, modelName)
	if err != nil {
		t.Skipf("createModel(%q) failed: %v", modelName, err)
	}

	prompt := "Назви 3 популярні місця у Львові. Відповідай дуже коротко, одним реченням."
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			genai.NewContentFromText(prompt, "user"),
		},
	}

	var sb strings.Builder
	for resp, err := range m.GenerateContent(ctx, req, false) {
		if err != nil {
			t.Skipf("live model call returned error (%v) — skipping (quota or network)", err)
		}
		if resp.Content != nil {
			for _, part := range resp.Content.Parts {
				if part.Text != "" {
					sb.WriteString(part.Text)
				}
			}
		}
	}

	answer := strings.TrimSpace(sb.String())
	if answer == "" {
		t.Fatalf("model %s returned empty response for prompt %q", m.Name(), prompt)
	}
	t.Logf("Live prompt test SUCCESS with model %q:\n%s", m.Name(), answer)
}
