// Package provider holds the OpenAI-compatible provider layer (D6, D7, D25).
package provider

// Config is the [providers] section of fender.toml.
type Config struct {
	Mode          string              `toml:"mode"`           // permission mode: strict | balanced | yolo (D21)
	Fallback      string              `toml:"fallback"`       // backup provider/key after a failed model request (D50)
	MaxIterations int                 `toml:"max_iterations"` // agent loop cap, 0 = 30 (D54)
	Providers     map[string]Provider `toml:"providers"`
}

// Provider is one OpenAI-compatible endpoint (OpenRouter, Ollama, LM Studio, ...).
type Provider struct {
	BaseURL      string                 `toml:"base_url"`
	APIKey       string                 `toml:"api_key"`
	Path         string                 `toml:"path"` // API path prefix, default "/v1" (OpenRouter: "/api/v1")
	Models       []string               `toml:"models"`
	DefaultModel string                 `toml:"default_model"`
	ModelConfigs map[string]ModelConfig `toml:"model_configs"`
}

// ModelConfig is per-model thinking configuration (D40).
type ModelConfig struct {
	Thinking       bool              `toml:"thinking"`
	ThinkingLevels map[string]string `toml:"thinking_levels"` // pi-level → provider value; "" = unsupported (null); omission = identity
}
