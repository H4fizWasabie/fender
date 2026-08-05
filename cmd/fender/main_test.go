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

func TestRunCommand(t *testing.T) {
	// hermetic: explicit config with no usable provider must error fast instead
	// of picking up ~/.fender/fender.toml and calling the real API (D41-era hang).
	path := writeConfig(t, "mode = \"balanced\"\n")
	var out bytes.Buffer
	err := runCLI(&out, []string{"--config", path, "run", "do something"})
	if err == nil {
		t.Fatal("expected error without a usable provider")
	}
}

func TestInitCommand(t *testing.T) {
	home := t.TempDir() // no global config here → template must be written
	t.Setenv("HOME", home)
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

func TestInitSkipsLocalConfigWhenGlobalExists(t *testing.T) {
	// global config exists → init must NOT write a shadowing placeholder
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".fender"), 0700)
	os.WriteFile(filepath.Join(home, ".fender", "fender.toml"), []byte("mode = \"yolo\"\n"), 0600)

	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)

	var out bytes.Buffer
	if err := runCLI(&out, []string{"init"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "fender.toml")); err == nil {
		t.Fatal("local placeholder config written despite global config")
	}
	if !strings.Contains(out.String(), "providers come from") {
		t.Fatalf("output = %q", out.String())
	}
}
