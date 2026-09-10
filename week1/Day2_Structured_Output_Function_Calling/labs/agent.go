package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/genai"

	"github.com/dimetron/ai-eng-course/labs/internal/adkenv"
)

// ErrNoProvider reports that no usable provider credentials were found.
var ErrNoProvider = errors.New("no model provider configured")

// ProviderChoice describes which backend was selected and why. Returned
// alongside the model so the CLI can print it: a learner who cannot tell which
// provider answered cannot debug a wrong answer.
type ProviderChoice struct {
	Provider string
	Model    string
	Reason   string
}

// BuildModel selects a model backend from the environment.
//
// ADK Go v2.2.0 ships exactly three provider packages — gemini, openaimodel and
// apigee. There is NO Anthropic backend, so an Anthropic key alone is not
// enough to run this lab; that is a real constraint of the framework, not an
// oversight in the exercise.
//
// Ollama is reached through openaimodel's BaseURL field, which the package
// documents as being "for OpenAI-compatible endpoints". This is how a
// self-hosted model gets first-class treatment without an upstream Ollama
// provider existing.
//
// Precedence is explicit and documented rather than implicit, because "which
// key wins" is otherwise the single most confusing thing about a multi-provider
// example. Order: agentgateway -> ollama -> gemini -> openai.
func BuildModel(ctx context.Context) (model.LLM, ProviderChoice, error) {
	// agentgateway wins over every direct provider on purpose: when the local
	// gateway is up, ALL traffic should go through it, otherwise the traces and
	// the cost ledger have holes in them and stop being evidence. Same
	// openaimodel backend as Ollama — a gateway is just another
	// OpenAI-compatible URL. See the Week 1 guide
	// (courses/.../Local_Monitoring_Agentgateway.md).
	if base, ok := adkenv.Key("AGENTGATEWAY_BASE_URL"); ok {
		name := envOrDefault("AGENTGATEWAY_MODEL", "mock-gpt")
		key, _ := adkenv.Key("AGENTGATEWAY_API_KEY")
		m, err := openaimodel.NewModel(ctx, name, &openaimodel.ClientConfig{
			APIKey:  orPlaceholder(key),
			BaseURL: base,
		})
		if err != nil {
			return nil, ProviderChoice{}, fmt.Errorf("agentgateway via openai-compatible endpoint: %w", err)
		}
		return m, ProviderChoice{
			Provider: "agentgateway",
			Model:    name,
			Reason:   "AGENTGATEWAY_BASE_URL is set; all calls are routed through the local gateway",
		}, nil
	}

	if base, ok := adkenv.Key("OLLAMA_BASE_URL"); ok {
		name := envOrDefault("OLLAMA_MODEL", "qwen3")
		key, _ := adkenv.Key("OLLAMA_API_KEY") // Ollama ignores it; some proxies require it.
		m, err := openaimodel.NewModel(ctx, name, &openaimodel.ClientConfig{
			APIKey:  orPlaceholder(key),
			BaseURL: base,
		})
		if err != nil {
			return nil, ProviderChoice{}, fmt.Errorf("ollama via openai-compatible endpoint: %w", err)
		}
		return m, ProviderChoice{
			Provider: "ollama",
			Model:    name,
			Reason:   "OLLAMA_BASE_URL is set; using openaimodel with a custom BaseURL",
		}, nil
	}

	if key, ok := adkenv.Key("GOOGLE_API_KEY"); ok {
		name := envOrDefault("GEMINI_MODEL", "gemini-flash-latest")
		m, err := gemini.NewModel(ctx, name, &genai.ClientConfig{APIKey: key})
		if err != nil {
			return nil, ProviderChoice{}, fmt.Errorf("gemini: %w", err)
		}
		return m, ProviderChoice{
			Provider: "gemini",
			Model:    name,
			Reason:   "GOOGLE_API_KEY is set",
		}, nil
	}

	if key, ok := adkenv.Key("OPENAI_API_KEY"); ok {
		name := envOrDefault("OPENAI_MODEL", "gpt-5.6")
		m, err := openaimodel.NewModel(ctx, name, &openaimodel.ClientConfig{APIKey: key})
		if err != nil {
			return nil, ProviderChoice{}, fmt.Errorf("openai: %w", err)
		}
		return m, ProviderChoice{
			Provider: "openai",
			Model:    name,
			Reason:   "OPENAI_API_KEY is set",
		}, nil
	}

	if _, ok := adkenv.Key("ANTHROPIC_API_KEY"); ok {
		return nil, ProviderChoice{}, fmt.Errorf(
			"%w: ANTHROPIC_API_KEY is set, but ADK Go v2.2.0 ships no Anthropic backend "+
				"(providers: gemini, openaimodel, apigee). Set GOOGLE_API_KEY, OPENAI_API_KEY, "+
				"or OLLAMA_BASE_URL instead", ErrNoProvider)
	}

	return nil, ProviderChoice{}, fmt.Errorf(
		"%w: set GOOGLE_API_KEY, OPENAI_API_KEY or OLLAMA_BASE_URL "+
			"(apps/.env is loaded automatically)", ErrNoProvider)
}

func envOrDefault(name, fallback string) string {
	if v, ok := adkenv.Key(name); ok {
		return v
	}
	return fallback
}

// orPlaceholder supplies a non-empty key for endpoints that ignore it. The
// openai-go client rejects an empty API key before any request is made, so a
// local Ollama would fail for a reason that has nothing to do with Ollama.
func orPlaceholder(key string) string {
	if key == "" {
		return "not-needed-for-local-ollama"
	}
	return key
}

// NewRateTool builds the typed function tool.
//
// The schema is INFERRED from RateInput/RateOutput. Note what is not here: no
// hand-written JSON Schema, and no constraint syntax in the struct tags. If you
// need constraints (enum, minimum, required beyond Go's zero values), set
// functiontool.Config.InputSchema explicitly with a *jsonschema.Schema.
func NewRateTool(p Provider) (tool.Tool, error) {
	handler := func(ctx agent.Context, in RateInput) (RateOutput, error) {
		return Convert(ctx, p, in)
	}
	t, err := functiontool.New(functiontool.Config{
		Name: "get_exchange_rate",
		Description: "Returns the exchange rate between two ISO 4217 currencies, " +
			"cross-rated through UAH using National Bank of Ukraine data.",
	}, handler)
	if err != nil {
		return nil, fmt.Errorf("build rate tool: %w", err)
	}
	return t, nil
}

// instruction is deliberately explicit about the failure path.
//
// "Prompt vs contract": this text steers behaviour, but it guarantees nothing.
// The JSON Schema and the typed Go handler are what actually stop a malformed
// call from reaching the rate provider. Teach both, trust only the second.
const instruction = `You answer currency questions for Ukrainian users.

Rules:
- Always call get_exchange_rate for any rate question. Never state a rate from memory.
- The tool returns a source and a date; include both in your answer.
- If the tool returns an error naming an invalid or unknown currency code, correct
  your arguments and call it again at most once. If it still fails, say plainly
  that you could not get the rate. Never invent a number.`

// NewAgent wires the model and tool into an LlmAgent.
func NewAgent(m model.LLM, p Provider) (agent.Agent, error) {
	rateTool, err := NewRateTool(p)
	if err != nil {
		return nil, err
	}
	a, err := llmagent.New(llmagent.Config{
		Name:        "currency_agent",
		Model:       m,
		Description: "Answers currency-exchange questions using live NBU rates.",
		Instruction: instruction,
		Tools:       []tool.Tool{rateTool},
	})
	if err != nil {
		return nil, fmt.Errorf("build agent: %w", err)
	}
	return a, nil
}

// LoadEnv loads apps/.env relative to the working directory, ignoring a missing
// file so the key-free path still runs.
func LoadEnv() {
	if err := adkenv.Load("."); err != nil && !errors.Is(err, adkenv.ErrNotFound) {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
}
