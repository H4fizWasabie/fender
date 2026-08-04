package provider

import (
	"testing"

	"github.com/BurntSushi/toml"
)

func TestDecodeConfig(t *testing.T) {
	var cfg Config
	if _, err := toml.DecodeFile("testdata/config.toml", &cfg); err != nil {
		t.Fatal(err)
	}
	openrouter, ok := cfg.Providers["openrouter"]
	if !ok {
		t.Fatal("missing provider openrouter")
	}
	if openrouter.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("base_url = %q", openrouter.BaseURL)
	}
	if openrouter.APIKey != "sk-test-123" {
		t.Fatalf("api_key = %q", openrouter.APIKey)
	}
	if len(openrouter.Models) != 2 || openrouter.Models[0] != "openai/gpt-4o-mini" {
		t.Fatalf("models = %v", openrouter.Models)
	}
	if openrouter.DefaultModel != "openai/gpt-4o-mini" {
		t.Fatalf("default_model = %q", openrouter.DefaultModel)
	}
	if cfg.Mode != "balanced" {
		t.Fatalf("mode = %q", cfg.Mode)
	}
}
