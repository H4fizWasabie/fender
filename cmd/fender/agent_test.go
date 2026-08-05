package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/codeintel"
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
	if a.Mem == nil || a.Skills == nil || a.Ctx == nil {
		t.Fatal("wiring incomplete: Mem/Skills/Ctx must be set")
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
