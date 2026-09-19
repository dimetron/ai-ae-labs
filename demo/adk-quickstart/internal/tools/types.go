package tools

// RawProvider represents a provider entry in models.json.
type RawProvider struct {
	ID     string              `json:"id"`
	Name   string              `json:"name"`
	API    string              `json:"api"`
	Doc    string              `json:"doc"`
	Env    []string            `json:"env"`
	NPM    string              `json:"npm"`
	Models map[string]RawModel `json:"models"`
}

// RawModel represents a model entry in models.json.
type RawModel struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Family           string            `json:"family"`
	Attachment       bool              `json:"attachment"`
	Reasoning        bool              `json:"reasoning"`
	ReasoningOptions []ReasoningOption `json:"reasoning_options"`
	ToolCall         bool              `json:"tool_call"`
	StructuredOutput bool              `json:"structured_output"`
	Temperature      bool              `json:"temperature"`
	OpenWeights      bool              `json:"open_weights"`
	Knowledge        string            `json:"knowledge"`
	ReleaseDate      string            `json:"release_date"`
	LastUpdated      string            `json:"last_updated"`
	Modalities       Modalities        `json:"modalities"`
	Limit            Limit             `json:"limit"`
	Cost             Cost              `json:"cost"`
	Experimental     any               `json:"experimental,omitempty"`
	Interleaved      any               `json:"interleaved,omitempty"`
	Provider         any               `json:"provider,omitempty"`
	Status           string            `json:"status"`
}

// ReasoningOption represents configurable reasoning mode options.
type ReasoningOption struct {
	Type   string   `json:"type"`
	Values []string `json:"values,omitempty"`
}

// Modalities describes supported input and output formats.
type Modalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

// Limit represents context window and output token limits.
type Limit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
	Input   int `json:"input,omitempty"`
}

// Cost represents token pricing in USD per 1M tokens.
type Cost struct {
	Input       float64 `json:"input"`
	Output      float64 `json:"output"`
	CacheRead   float64 `json:"cache_read"`
	CacheWrite  float64 `json:"cache_write"`
	InputAudio  float64 `json:"input_audio"`
	OutputAudio float64 `json:"output_audio"`
}

// ProviderInfo is the summary of an inference provider.
type ProviderInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	API         string   `json:"api,omitempty"`
	Doc         string   `json:"doc,omitempty"`
	Env         []string `json:"env,omitempty"`
	ModelsCount int      `json:"models_count"`
}

// LabInfo represents a model creator lab / organization (e.g. OpenAI, Anthropic, DeepSeek).
type LabInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	ModelsCount int      `json:"models_count"`
	TopFamilies []string `json:"top_families,omitempty"`
}

// ModelSummary provides a concise overview of a model.
type ModelSummary struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Lab              string  `json:"lab"`
	ProviderID       string  `json:"provider_id"`
	ProviderName     string  `json:"provider_name"`
	Family           string  `json:"family,omitempty"`
	ContextTokens    int     `json:"context_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	CostInput        float64 `json:"cost_input"`
	CostOutput       float64 `json:"cost_output"`
	Reasoning        bool    `json:"reasoning"`
	ToolCall         bool    `json:"tool_call"`
	StructuredOutput bool    `json:"structured_output"`
	OpenWeights      bool    `json:"open_weights"`
	Description      string  `json:"description,omitempty"`
}

// ModelDetails provides complete specifications for a model.
type ModelDetails struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Lab              string            `json:"lab"`
	ProviderID       string            `json:"provider_id"`
	ProviderName     string            `json:"provider_name"`
	ProviderAPI      string            `json:"provider_api,omitempty"`
	ProviderDoc      string            `json:"provider_doc,omitempty"`
	Description      string            `json:"description"`
	Family           string            `json:"family"`
	Reasoning        bool              `json:"reasoning"`
	ReasoningOptions []ReasoningOption `json:"reasoning_options,omitempty"`
	ToolCall         bool              `json:"tool_call"`
	StructuredOutput bool              `json:"structured_output"`
	OpenWeights      bool              `json:"open_weights"`
	Modalities       Modalities        `json:"modalities"`
	Limit            Limit             `json:"limit"`
	Cost             Cost              `json:"cost"`
	KnowledgeCutoff  string            `json:"knowledge_cutoff,omitempty"`
	ReleaseDate      string            `json:"release_date,omitempty"`
	LastUpdated      string            `json:"last_updated,omitempty"`
	Status           string            `json:"status,omitempty"`
}

// ModelRecommendation represents a ranked recommendation candidate.
type ModelRecommendation struct {
	ModelID       string  `json:"model_id"`
	ModelName     string  `json:"model_name"`
	Lab           string  `json:"lab"`
	ProviderName  string  `json:"provider_name"`
	ContextTokens int     `json:"context_tokens"`
	CostInput     float64 `json:"cost_input"`
	CostOutput    float64 `json:"cost_output"`
	Reasoning     bool    `json:"reasoning"`
	ToolCall      bool    `json:"tool_call"`
	OpenWeights   bool    `json:"open_weights"`
	FitReason     string  `json:"fit_reason"`
	Score         float64 `json:"score"`
}

// --- Tool Input / Output Contracts ---

// ListProvidersInput defines parameters for list_providers tool.
type ListProvidersInput struct {
	Query string `json:"query,omitempty" jsonschema:"Optional query to search providers by ID or name"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum providers to return (default 30, max 100)"`
}

// ListProvidersOutput is the result of list_providers tool.
type ListProvidersOutput struct {
	Total     int            `json:"total"`
	Providers []ProviderInfo `json:"providers"`
}

// ListLabsInput defines parameters for list_labs tool.
type ListLabsInput struct {
	Query string `json:"query,omitempty" jsonschema:"Optional query to search creator labs by name or slug (e.g. openai, anthropic, meta)"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum labs to return (default 30, max 100)"`
}

// ListLabsOutput is the result of list_labs tool.
type ListLabsOutput struct {
	Total int       `json:"total"`
	Labs  []LabInfo `json:"labs"`
}

// ListModelsInput defines parameters for list_models tool.
type ListModelsInput struct {
	Query            string   `json:"query,omitempty" jsonschema:"Text search in model ID, name, or description (e.g. claude, deepseek, reasoning, llama)"`
	Provider         string   `json:"provider,omitempty" jsonschema:"Filter by specific provider ID (e.g. openrouter, openai, groq, hpc-ai)"`
	Lab              string   `json:"lab,omitempty" jsonschema:"Filter by creator lab (e.g. anthropic, deepseek, openai, meta-llama, google, mistralai)"`
	Family           string   `json:"family,omitempty" jsonschema:"Filter by model family (e.g. gpt, claude-sonnet, deepseek-flash, llama)"`
	Reasoning        *bool    `json:"reasoning,omitempty" jsonschema:"Filter models with reasoning/thinking support"`
	ToolCall         *bool    `json:"tool_call,omitempty" jsonschema:"Filter models supporting function/tool calling"`
	StructuredOutput *bool    `json:"structured_output,omitempty" jsonschema:"Filter models supporting structured JSON output"`
	OpenWeights      *bool    `json:"open_weights,omitempty" jsonschema:"Filter open weights (true) or proprietary models (false)"`
	MinContext       int      `json:"min_context,omitempty" jsonschema:"Minimum context window in tokens (e.g. 128000, 1000000)"`
	MaxInputCost     *float64 `json:"max_input_cost,omitempty" jsonschema:"Maximum cost per 1M input tokens in USD (e.g. 0.5)"`
	Limit            int      `json:"limit,omitempty" jsonschema:"Maximum models to return (default 20, max 100)"`
}

// ListModelsOutput is the result of list_models tool.
type ListModelsOutput struct {
	Total  int            `json:"total"`
	Models []ModelSummary `json:"models"`
}

// GetModelDetailsInput defines parameters for get_model_details tool.
type GetModelDetailsInput struct {
	ModelID  string `json:"model_id" jsonschema:"Model ID or name to look up (e.g. deepseek/deepseek-v4-flash, claude-opus-4.7, gpt-5.5)"`
	Provider string `json:"provider,omitempty" jsonschema:"Optional provider ID to disambiguate"`
}

// RecommendModelsInput defines parameters for recommend_models tool.
type RecommendModelsInput struct {
	Task               string   `json:"task" jsonschema:"Description of task or workload (e.g. coding agent with tool use, cheap classification at scale, complex math reasoning, 1M context document analysis, local offline open weights)"`
	MinContext         int      `json:"min_context,omitempty" jsonschema:"Minimum context window in tokens (e.g. 128000)"`
	MaxInputCost       *float64 `json:"max_input_cost,omitempty" jsonschema:"Maximum budget in USD per 1M input tokens"`
	RequireReasoning   bool     `json:"require_reasoning,omitempty" jsonschema:"Require reasoning or extended thinking"`
	RequireToolCall    bool     `json:"require_tool_call,omitempty" jsonschema:"Require function/tool calling support"`
	RequireVision      bool     `json:"require_vision,omitempty" jsonschema:"Require image or vision input support"`
	RequireOpenWeights bool     `json:"require_open_weights,omitempty" jsonschema:"Require open weights model"`
	Limit              int      `json:"limit,omitempty" jsonschema:"Number of recommendations to return (default 5, max 20)"`
}

// RecommendModelsOutput is the result of recommend_models tool.
type RecommendModelsOutput struct {
	Task            string                `json:"task"`
	Recommendations []ModelRecommendation `json:"recommendations"`
}

// CourseRecommendedModel represents an officially recommended course model from .env.example.
type CourseRecommendedModel struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	Provider          string       `json:"provider"`
	Lab               string       `json:"lab"`
	EnvVar            string       `json:"env_var"`
	AlternativeEnvVar string       `json:"alternative_env_var,omitempty"`
	APIKeyURL         string       `json:"api_key_url,omitempty"`
	Tier              string       `json:"tier"`
	Category          string       `json:"category"`
	ContextWindow     int          `json:"context_window"`
	MaxOutputTokens   int          `json:"max_output_tokens"`
	CostPerMillion    Cost         `json:"cost_per_million"`
	Capabilities      Capabilities `json:"capabilities"`
	CourseRole        string       `json:"course_role"`
	RecommendedFor    []string     `json:"recommended_for"`
}

// Capabilities represents the feature flags for a course model.
type Capabilities struct {
	Reasoning        bool `json:"reasoning"`
	ToolCalling      bool `json:"tool_calling"`
	StructuredOutput bool `json:"structured_output"`
	Vision           bool `json:"vision"`
	OpenWeights      bool `json:"open_weights"`
}

// ListCourseModelsInput is the input for list_course_models tool.
type ListCourseModelsInput struct {
	Query string `json:"query,omitempty" jsonschema:"Optional query to filter recommended models"`
}

// ListCourseModelsOutput contains course-curated models from recommended_models.json.
type ListCourseModelsOutput struct {
	Version      string                   `json:"version"`
	Description  string                   `json:"description"`
	DefaultModel string                   `json:"default_model"`
	Models       []CourseRecommendedModel `json:"models"`
}
