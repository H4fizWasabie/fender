// Package provider holds the OpenAI-compatible provider layer (D6, D7, D25).
package provider

// Config is the [providers] section of fender.toml.
type Config struct {
	Providers map[string]Provider `toml:"providers"`
}

// Provider is one OpenAI-compatible endpoint (OpenRouter, Ollama, LM Studio, ...).
type Provider struct {
	BaseURL      string   `toml:"base_url"`
	APIKey       string   `toml:"api_key"`
	Models       []string `toml:"models"`
	DefaultModel string   `toml:"default_model"`
}
