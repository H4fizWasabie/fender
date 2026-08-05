// Package provider holds the OpenAI-compatible provider layer (D6, D7, D25).
package provider

// Config is the [providers] section of fender.toml.
type Config struct {
	Mode      string              `toml:"mode"`     // permission mode: strict | balanced | yolo (D21)
	Subagent  string              `toml:"subagent"` // default provider for subagents (D48)
	Providers map[string]Provider `toml:"providers"`
}

// Provider is one OpenAI-compatible endpoint (OpenRouter, Ollama, LM Studio, ...).
type Provider struct {
	BaseURL      string                 `toml:"base_url"`
	APIKey       string                 `toml:"api_key"`
	Models       []string               `toml:"models"`
	DefaultModel string                 `toml:"default_model"`
	ModelConfigs map[string]ModelConfig `toml:"model_configs"`
}

// ModelConfig is per-model thinking configuration (D40).
type ModelConfig struct {
	Thinking       bool              `toml:"thinking"`
	ThinkingLevels map[string]string `toml:"thinking_levels"` // pi-level → provider value; "" = unsupported (null); omission = identity
}
