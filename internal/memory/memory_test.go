package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureCreatesStructure(t *testing.T) {
	root := t.TempDir()
	m := New(root)
	if err := m.Ensure(); err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(root, ".fender", "memory", "PROJECT.md"),
		filepath.Join(root, ".fender", "memory", "MAP.md"),
		filepath.Join(root, ".fender", "memory", "reference"),
		filepath.Join(root, ".fender", "memory", "working"),
		filepath.Join(root, ".fender", "memory", "facts"),
		filepath.Join(root, ".fender", "skills"),
	}
	for _, p := range want {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
	}
}

func TestEnsureIdempotent(t *testing.T) {
	root := t.TempDir()
	m := New(root)
	if err := m.Ensure(); err != nil {
		t.Fatal(err)
	}
	// user edits PROJECT.md
	proj := filepath.Join(root, ".fender", "memory", "PROJECT.md")
	if err := os.WriteFile(proj, []byte("# user edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.Ensure(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(proj)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# user edit" {
		t.Fatalf("Ensure overwrote user edit: %q", got)
	}
}

func TestEnsureSeedsTemplates(t *testing.T) {
	root := t.TempDir()
	m := New(root)
	if err := m.Ensure(); err != nil {
		t.Fatal(err)
	}
	proj, err := os.ReadFile(filepath.Join(root, ".fender", "memory", "PROJECT.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(proj), "PROJECT.md") {
		t.Fatalf("PROJECT.md template missing marker: %q", proj)
	}
	mapmd, err := os.ReadFile(filepath.Join(root, ".fender", "memory", "MAP.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(mapmd), "ticket 07") {
		t.Fatalf("MAP.md placeholder missing code-intel note: %q", mapmd)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
