package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// user-level global rules
	userDir := filepath.Join(home, ".fender")
	if err := os.MkdirAll(userDir, 0700); err != nil {
		t.Fatal(err)
	}
	userAgents := filepath.Join(userDir, "AGENTS.md")
	os.WriteFile(userAgents, []byte("user rules"), 0600)

	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("project rules"), 0600)
	os.WriteFile(filepath.Join(root, "CONTEXT.md"), []byte("context"), 0600)
	// decoys — must never be detected
	os.WriteFile(filepath.Join(root, "README.md"), []byte("readme"), 0600)
	os.WriteFile(filepath.Join(root, ".cursorrules"), []byte("cursor"), 0600)

	m := New(root)
	got := m.Detect(root)
	if len(got) != 3 {
		t.Fatalf("detected %d files: %+v", len(got), got)
	}
	if got[0].Path != userAgents || got[0].Layer != "user" || got[0].Kind != "AGENTS.md" {
		t.Fatalf("got[0] = %+v", got[0])
	}
	if got[1].Kind != "AGENTS.md" || got[1].Layer != "project" {
		t.Fatalf("got[1] = %+v", got[1])
	}
	if got[2].Kind != "CONTEXT.md" {
		t.Fatalf("got[2] = %+v", got[2])
	}
}

func TestDetectClaudeFallback(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("claude rules"), 0600)
	m := New(root)
	got := m.Detect(root)
	if len(got) != 1 || got[0].Kind != "CLAUDE.md" {
		t.Fatalf("got = %+v", got)
	}
}

func TestDetectNothing(t *testing.T) {
	m := New(t.TempDir())
	if got := m.Detect(t.TempDir()); len(got) != 0 {
		t.Fatalf("got = %+v", got)
	}
}
