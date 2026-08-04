package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

const cfg = `
[providers.openrouter]
base_url = "https://openrouter.ai/api/v1"
api_key = "k1"
models = ["a", "b"]
default_model = "a"
`

func TestProvidersCommand(t *testing.T) {
	path := writeConfig(t, cfg)
	var out bytes.Buffer
	if err := runCLI(&out, []string{"--config", path, "providers"}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "openrouter") || !strings.Contains(got, "https://openrouter.ai/api/v1") {
		t.Fatalf("output = %q", got)
	}
	if !strings.Contains(got, "a, b") {
		t.Fatalf("models missing: %q", got)
	}
}

func TestNoArgsShowsUsage(t *testing.T) {
	var out bytes.Buffer
	if err := runCLI(&out, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "usage") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCommand(t *testing.T) {
	var out bytes.Buffer
	err := runCLI(&out, []string{"run", "do something"})
	if err == nil {
		t.Fatal("expected error without config")
	}
}

func TestInitCommand(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runCLI(&out, []string{"init"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".fender", "memory", "PROJECT.md")); err != nil {
		t.Fatal("memory workspace not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "fender.toml")); err != nil {
		t.Fatal("fender.toml not scaffolded")
	}
	if err := runCLI(&out, []string{"init"}); err != nil {
		t.Fatal(err)
	}
}
