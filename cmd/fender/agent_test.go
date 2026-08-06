package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/codeintel"
	"github.com/H4fizWasabie/fender/internal/provider"
)

func TestBuildAgentWithConfig(t *testing.T) {
	cfg := writeConfig(t, `
mode = "balanced"

[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`)
	a, err := buildAgent(cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("nil agent")
	}
	if a.System == "" {
		t.Fatal("system prompt missing")
	}
	if a.Mem == nil || a.Skills == nil || a.Meter == nil {
		t.Fatal("wiring incomplete: Mem/Skills/Meter must be set")
	}
}

func TestBuildAgentWithFallback(t *testing.T) {
	cfg := writeConfig(t, `
mode = "balanced"
fallback = "backup"

[providers.primary]
base_url = "http://localhost:1/v1"
api_key = "primary-key"
models = ["m1"]
default_model = "m1"

[providers.backup]
base_url = "http://localhost:2/v1"
api_key = "backup-key"
models = ["m1"]
default_model = "m1"
`)
	a, err := buildAgent(cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.LLM.(*provider.FallbackClient); !ok {
		t.Fatalf("LLM = %T, want fallback chain", a.LLM)
	}
}

func TestBuildAgentMissingConfig(t *testing.T) {
	if _, err := buildAgent(filepath.Join(t.TempDir(), "absent.toml"), nil, nil); err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestIntelRefreshTool(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	os.WriteFile("main.go", []byte("package main\nfunc Fresh() {}\n"), 0600)

	store, err := codeintel.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	tool := intelRefreshTool(store)
	out, err := tool.Call(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "refreshed") {
		t.Fatalf("output = %q", out)
	}
}

func TestSystemPromptHasCWDAndGuidelines(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	os.WriteFile(filepath.Join(dir, "fender.toml"), []byte(`
mode = "balanced"

[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`), 0600)
	a, err := buildAgent(filepath.Join(dir, "fender.toml"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.System, "Current working directory: "+dir) {
		t.Fatalf("system missing cwd: %q", a.System)
	}
	if strings.Contains(a.System, "TOOL USE") || strings.Contains(a.System, "COMPLETION") {
		t.Fatal("hardcoded sections must be gone (D62)")
	}
	if !strings.Contains(a.System, "Be concise") {
		t.Fatal("core guideline missing")
	}
}

func TestPromptGuidelinesFromConfig(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	os.WriteFile(filepath.Join(dir, "fender.toml"), []byte(`
prompt_guidelines = ["always use gofmt before finishing"]
mode = "balanced"

[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`), 0600)
	a, err := buildAgent(filepath.Join(dir, "fender.toml"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.System, "always use gofmt before finishing") {
		t.Fatalf("config guideline missing: %q", a.System)
	}
}
