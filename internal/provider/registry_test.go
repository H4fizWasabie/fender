package provider

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fender.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

const twoProviders = `
[providers.openrouter]
base_url = "https://openrouter.ai/api/v1"
api_key = "k1"
models = ["a", "b"]
default_model = "a"

[providers.ollama]
base_url = "http://localhost:11434/v1"
api_key = "ollama"
models = ["llama3.2"]
`

func TestLoadAndLookup(t *testing.T) {
	r, err := Load(writeConfig(t, twoProviders))
	if err != nil {
		t.Fatal(err)
	}
	c, ok := r.Client("ollama")
	if !ok || c.Model() != "llama3.2" {
		t.Fatalf("ollama client = %v, ok=%v", c, ok)
	}
	if _, ok := r.Client("nope"); ok {
		t.Fatal("unexpected provider nope")
	}
	names := r.Names()
	if !sort.StringsAreSorted(names) || len(names) != 2 {
		t.Fatalf("names = %v", names)
	}
	d, ok := r.Default()
	if !ok || d.Name() != "openrouter" {
		t.Fatalf("default = %v, ok=%v", d, ok)
	}
}

func TestLoadMissingFileErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.toml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidTOMLErrors(t *testing.T) {
	if _, err := Load(writeConfig(t, "not toml [")); err == nil {
		t.Fatal("expected error for invalid toml")
	}
}

func TestWithFallback(t *testing.T) {
	r, err := Load(writeConfig(t, `
fallback = "backup"

[providers.primary]
base_url = "https://primary.example"
api_key = "k1"
models = ["m"]
default_model = "m"

[providers.backup]
base_url = "https://backup.example"
api_key = "k2"
models = ["m"]
`))
	if err != nil {
		t.Fatal(err)
	}
	primary, _ := r.Client("primary")
	if d, _ := r.Default(); d != primary {
		t.Fatalf("default selected fallback provider %q", d.Name())
	}
	configured, err := r.WithFallback(primary)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := configured.(*FallbackClient); !ok {
		t.Fatalf("configured client = %T", configured)
	}
}

func TestWithFallbackRejectsUnknownProvider(t *testing.T) {
	r, err := Load(writeConfig(t, `
fallback = "missing"
[providers.primary]
base_url = "https://primary.example"
api_key = "k1"
models = ["m"]
default_model = "m"
`))
	if err != nil {
		t.Fatal(err)
	}
	primary, _ := r.Client("primary")
	if _, err := r.WithFallback(primary); err == nil {
		t.Fatal("expected missing fallback error")
	}
}
