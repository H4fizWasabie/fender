package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/provider"
)

// TestWiringSurface is the reachability audit (D64): every feature the
// agent ships must be reachable from a real config. A refactor that
// silently unwires something (like the D50-era drops) fails HERE, in CI,
// not in production. Run by name in CI as its own step.
func TestWiringSurface(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	os.WriteFile(filepath.Join(dir, "fender.toml"), []byte(`
mode = "balanced"
fallback = "mock-2"
max_iterations = 60
context_window = 1000000
reserve_tokens = 16384
prompt_guidelines = ["test guideline"]

[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"

[providers.mock-2]
base_url = "http://localhost:1/v1"
api_key = "k2"
models = ["m2"]
default_model = "m2"

[providers.claude]
base_url = "https://api.anthropic.com"
api = "anthropic"
api_key = "k3"
models = ["claude-sonnet"]
default_model = "claude-sonnet"
`), 0600)
	cfg := filepath.Join(dir, "fender.toml")
	a, err := buildAgent(cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Full tool registry — every tool the agent must carry.
	wantTools := []string{"read_file", "edit_file", "shell", "search", "delegate", "load_skill", "intel_refresh"}
	have := map[string]bool{}
	for _, n := range a.ToolNames() {
		have[n] = true
	}
	for _, want := range wantTools {
		if !have[want] {
			t.Fatalf("tool %q not registered (wiring surface): %v", want, a.ToolNames())
		}
	}

	// 2. Config dispatch — fallback + anthropic both reachable.
	if a.Meter == nil || a.Meter.Window != 1000000 {
		t.Fatalf("meter window not wired: %+v", a.Meter)
	}
	if !strings.Contains(a.System, "test guideline") || !strings.Contains(a.System, "Current working directory:") {
		t.Fatalf("prompt wiring incomplete: %.300q", a.System)
	}
	// anthropic dispatch (from the same config)
	reg, err := provider.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := reg.Client("claude"); !ok || !c.IsAnthropic() {
		t.Fatal("anthropic dispatch not wired")
	}
	if c, ok := reg.Client("mock"); !ok || c.IsAnthropic() {
		t.Fatal("openai provider misrouted")
	}

	// 3. Skill invocation reachable on the real bundled registry.
	if _, isSkill, err := skillTask(a, "/tdd write a test first"); !isSkill || err != nil {
		t.Fatalf("skill invocation not wired: isSkill=%v err=%v", isSkill, err)
	}

	// 4. CLI command surface — each command routes (non-blocking ones).
	var out bytes.Buffer
	for _, cmd := range []string{"providers", "run", "init", "sessions", "skill", "intel"} {
		err := runCLI(&out, []string{"--config", cfg, cmd})
		if err != nil && strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("command %q dropped from the CLI surface: %v", cmd, err)
		}
	}
}
