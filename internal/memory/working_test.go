package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBootstrapPrunesStaleNotes(t *testing.T) {
	root := t.TempDir()
	working := filepath.Join(root, ".fender", "memory", "working")
	m := New(root)
	if err := m.Ensure(); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(working, "fresh.md")
	stale := filepath.Join(working, "stale.md")
	patterns := filepath.Join(working, "patterns.md")
	os.WriteFile(fresh, []byte("f"), 0600)
	os.WriteFile(stale, []byte("s"), 0600)
	os.WriteFile(patterns, []byte("p"), 0600)
	old := time.Now().Add(-NotesMaxAge - time.Hour)
	os.Chtimes(stale, old, old)
	os.Chtimes(patterns, old, old)

	if _, err := m.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal("fresh note removed")
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("stale note not pruned")
	}
	if _, err := os.Stat(patterns); err != nil {
		t.Fatal("patterns.md must never be pruned")
	}
}

func TestWorkingCatalog(t *testing.T) {
	root := t.TempDir()
	m := New(root)
	m.Ensure()
	working := filepath.Join(root, ".fender", "memory", "working")
	os.WriteFile(filepath.Join(working, "alpha.md"), []byte("a"), 0600)
	os.WriteFile(filepath.Join(working, "beta.md"), []byte("b"), 0600)
	m.Bootstrap()
	got := m.Working()
	if len(got) != 2 {
		t.Fatalf("catalog = %v", got)
	}
	if !strings.Contains(got[0], "alpha.md") || !strings.Contains(got[1], "beta.md") {
		t.Fatalf("catalog order/content = %v", got)
	}
}
