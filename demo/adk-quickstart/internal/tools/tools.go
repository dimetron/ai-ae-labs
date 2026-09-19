package tools

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

//go:embed resources/models.json
var embeddedModelsJSON []byte

//go:embed resources/recommended_models.json
var embeddedRecommendedModelsJSON []byte

var (
	defaultStore        *Store
	defaultTools        []tool.Tool
	defaultCourseModels ListCourseModelsOutput
)

func init() {
	var err error
	defaultStore, err = NewStoreFromJSON(embeddedModelsJSON)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize models store: %v", err))
	}

	if err := json.Unmarshal(embeddedRecommendedModelsJSON, &defaultCourseModels); err != nil {
		panic(fmt.Sprintf("failed to initialize recommended models catalog: %v", err))
	}

	defaultTools, err = BuildTools(defaultStore)
	if err != nil {
		panic(fmt.Sprintf("failed to build tools: %v", err))
	}
}

// All returns all ADK function tools for the Model Expert Agent.
// Enables a clean one-liner registration in llmagent.Config{Tools: tools.All()}.
func All() []tool.Tool {
	return defaultTools
}

// DefaultStore returns the indexed in-memory catalog store.
func DefaultStore() *Store {
	return defaultStore
}

// CourseModels returns the curated list of recommended models from recommended_models.json.
func CourseModels() ListCourseModelsOutput {
	return defaultCourseModels
}

// BuildTools constructs the full suite of ADK function tools from a Store instance.
func BuildTools(store *Store) ([]tool.Tool, error) {
	if store == nil {
		return nil, fmt.Errorf("catalog store cannot be nil")
	}

	listModelsTool, err := functiontool.New(functiontool.Config{
		Name: "list_models",
		Description: "Search, filter, and summarize LLM models in the catalog. " +
			"Supports filtering by query text, provider ID, creator lab (e.g. 'anthropic', 'deepseek', 'openai'), " +
			"model family, reasoning support, tool calling support, structured output, open weights, minimum context window, " +
			"and maximum input cost.",
	}, func(_ agent.Context, in ListModelsInput) (ListModelsOutput, error) {
		return store.ListModels(in), nil
	})
	if err != nil {
		return nil, fmt.Errorf("create list_models tool: %w", err)
	}

	listProvidersTool, err := functiontool.New(functiontool.Config{
		Name: "list_providers",
		Description: "Lists AI inference providers / hosts (e.g. OpenRouter, OpenAI, Groq, Together, HPC-AI, etc.) " +
			"with their API URLs, documentation links, environment variable names, and hosted model counts.",
	}, func(_ agent.Context, in ListProvidersInput) (ListProvidersOutput, error) {
		return store.ListProviders(in), nil
	})
	if err != nil {
		return nil, fmt.Errorf("create list_providers tool: %w", err)
	}

	listLabsTool, err := functiontool.New(functiontool.Config{
		Name: "list_labs",
		Description: "Lists model creator labs / organizations (e.g. OpenAI, Anthropic, Google, DeepSeek, Meta, " +
			"Mistral AI, Moonshot AI, Alibaba Qwen) with total models count and primary model families.",
	}, func(_ agent.Context, in ListLabsInput) (ListLabsOutput, error) {
		return store.ListLabs(in), nil
	})
	if err != nil {
		return nil, fmt.Errorf("create list_labs tool: %w", err)
	}

	getModelDetailsTool, err := functiontool.New(functiontool.Config{
		Name: "get_model_details",
		Description: "Retrieves complete technical parameters and specifications for a specific model ID or name. " +
			"Includes context window tokens, max output tokens, exact pricing (input, output, cache read per 1M tokens), " +
			"reasoning options, modalities, structured output, tool calling support, open weights status, and release date.",
	}, func(_ agent.Context, in GetModelDetailsInput) (*ModelDetails, error) {
		return store.GetModelDetails(in)
	})
	if err != nil {
		return nil, fmt.Errorf("create get_model_details tool: %w", err)
	}

	recommendModelsTool, err := functiontool.New(functiontool.Config{
		Name: "recommend_models",
		Description: "Recommends top-fit models for a specific task or workload (e.g. coding agent, low-cost extraction, " +
			"math reasoning, 1M context document analysis, local open-weights deployment) with fit reasons and trade-offs.",
	}, func(_ agent.Context, in RecommendModelsInput) (RecommendModelsOutput, error) {
		return store.RecommendModels(in), nil
	})
	if err != nil {
		return nil, fmt.Errorf("create recommend_models tool: %w", err)
	}

	listCourseModelsTool, err := functiontool.New(functiontool.Config{
		Name: "list_course_models",
		Description: "Lists the official models recommended for the Prometheus AI Agents Engineering course (.env.example), " +
			"including Gemini 3.7 Flash, Claude Haiku 4.5, Claude Sonnet 5, GPT-5.6 Luna, GPT-4o, DeepSeek V4 Flash, and Llama 3, " +
			"with their specific course roles, tiers, and configuration advice.",
	}, func(_ agent.Context, in ListCourseModelsInput) (ListCourseModelsOutput, error) {
		q := strings.ToLower(strings.TrimSpace(in.Query))
		if q == "" {
			return defaultCourseModels, nil
		}
		var filtered []CourseRecommendedModel
		for _, m := range defaultCourseModels.Models {
			if strings.Contains(strings.ToLower(m.ID), q) ||
				strings.Contains(strings.ToLower(m.Name), q) ||
				strings.Contains(strings.ToLower(m.Provider), q) ||
				strings.Contains(strings.ToLower(m.Tier), q) ||
				strings.Contains(strings.ToLower(m.CourseRole), q) {
				filtered = append(filtered, m)
			}
		}
		return ListCourseModelsOutput{
			Version:      defaultCourseModels.Version,
			Description:  defaultCourseModels.Description,
			DefaultModel: defaultCourseModels.DefaultModel,
			Models:       filtered,
		}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("create list_course_models tool: %w", err)
	}

	return []tool.Tool{
		listModelsTool,
		listProvidersTool,
		listLabsTool,
		getModelDetailsTool,
		recommendModelsTool,
		listCourseModelsTool,
	}, nil
}
