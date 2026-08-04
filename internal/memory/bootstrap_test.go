package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapReadsLayers(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("project rules"), 0600)
	os.WriteFile(filepath.Join(root, "CONTEXT.md"), []byte("context rules"), 0600)
	m := New(root)
	b, err := m.Bootstrap()
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Convention) != 2 {
		t.Fatalf("convention = %+v", b.Convention)
	}
	if !strings.Contains(b.ProjectMD, "PROJECT.md") {
		t.Fatalf("ProjectMD = %q", b.ProjectMD)
	}
	if !strings.Contains(b.MAPMD, "ticket 07") {
		t.Fatalf("MAPMD = %q", b.MAPMD)
	}
}

func TestSystemOrderAndCanonicalSources(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".fender"), 0700)
	os.WriteFile(filepath.Join(home, ".fender", "AGENTS.md"), []byte("USER-RULES"), 0600)

	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("PROJECT-RULES"), 0600)
	os.WriteFile(filepath.Join(root, "CONTEXT.md"), []byte("CONTEXT-RULES"), 0600)
	m := New(root)
	b, _ := m.Bootstrap()
	sys := b.System()
	// order: user first, then project AGENTS, then CONTEXT, then PROJECT.md
	ui := strings.Index(sys, "USER-RULES")
	pi := strings.Index(sys, "PROJECT-RULES")
	ci := strings.Index(sys, "CONTEXT-RULES")
	mi := strings.Index(sys, "PROJECT.md")
	if !(ui < pi && pi < ci && ci < mi) {
		t.Fatalf("order wrong: user=%d project=%d context=%d projectmd=%d\n%s", ui, pi, ci, mi, sys)
	}
	// canonical sources: PROJECT.md never contains copied convention content
	if strings.Contains(b.ProjectMD, "PROJECT-RULES") || strings.Contains(b.ProjectMD, "USER-RULES") {
		t.Fatal("PROJECT.md contains copied convention content")
	}
}

func TestSystemCapTruncatesOldestFirst(t *testing.T) {
	// make the always-loaded layer exceed SystemCap via a huge user AGENTS.md
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".fender"), 0700)
	os.WriteFile(filepath.Join(home, ".fender", "AGENTS.md"), []byte(strings.Repeat("x", SystemCap+1000)), 0600)
	root := t.TempDir()
	m := New(root)
	b, _ := m.Bootstrap()
	sys := b.System()
	if len(sys) > SystemCap {
		t.Fatalf("System() exceeded cap: %d > %d", len(sys), SystemCap)
	}
	if !strings.Contains(sys, "truncated") {
		t.Fatalf("missing truncation marker: %q", sys[:200])
	}
}

func TestBootstrapSkipsUnreadable(t *testing.T) {
	root := t.TempDir()
	bad := filepath.Join(root, "AGENTS.md")
	os.WriteFile(bad, []byte("x"), 0600)
	os.Chmod(bad, 0000)
	m := New(root)
	if _, err := m.Bootstrap(); err != nil {
		t.Fatalf("unreadable convention file must not error: %v", err)
	}
	os.Chmod(bad, 0600)
}
