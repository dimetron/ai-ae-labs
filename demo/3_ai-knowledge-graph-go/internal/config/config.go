// Package config handles TOML configuration loading.
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Config holds all application configuration.
type Config struct {
	LLM             LLMConfig             `toml:"llm"`
	Chunking        ChunkingConfig        `toml:"chunking"`
	Standardization StandardizationConfig `toml:"standardization"`
	Inference       InferenceConfig       `toml:"inference"`
	Visualization   VisualizationConfig   `toml:"visualization"`
}

// LLMConfig holds LLM provider settings.
type LLMConfig struct {
	Model       string  `toml:"model"`
	APIKey      string  `toml:"api_key"`
	BaseURL     string  `toml:"base_url"`
	MaxTokens   int     `toml:"max_tokens"`
	Temperature float64 `toml:"temperature"`
}

// ChunkingConfig holds text chunking parameters.
type ChunkingConfig struct {
	ChunkSize int `toml:"chunk_size"`
	Overlap   int `toml:"overlap"`
}

// StandardizationConfig holds entity standardization settings.
type StandardizationConfig struct {
	Enabled            bool `toml:"enabled"`
	UseLLMForEntities  bool `toml:"use_llm_for_entities"`
}

// InferenceConfig holds relationship inference settings.
type InferenceConfig struct {
	Enabled             bool `toml:"enabled"`
	UseLLMForInference  bool `toml:"use_llm_for_inference"`
	ApplyTransitive     bool `toml:"apply_transitive"`
}

// VisualizationConfig holds visualization settings.
type VisualizationConfig struct {
	EdgeSmooth interface{} `toml:"edge_smooth"`
}

// Load reads and parses a TOML configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	// Apply defaults.
	if cfg.Chunking.ChunkSize == 0 {
		cfg.Chunking.ChunkSize = 500
	}
	if cfg.Chunking.Overlap == 0 {
		cfg.Chunking.Overlap = 50
	}
	if cfg.LLM.MaxTokens == 0 {
		cfg.LLM.MaxTokens = 4096
	}
	if cfg.LLM.Temperature == 0 {
		cfg.LLM.Temperature = 0.2
	}

	return &cfg, nil
}
