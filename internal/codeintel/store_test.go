package codeintel

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0700)
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRefreshIncremental(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"main.go":          "package main\nfunc A() {}\n",
		"notes.txt":        "ignore me",
		".fender/x.txt":    "skip me",
		"vendor/y.go":      "package y\nfunc Y() {}\n",
		"node_modules/z.go": "package z\n",
	})
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := s.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("first refresh changed = %d, want 1", changed)
	}
	changed, err = s.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	if changed != 0 {
		t.Fatalf("second refresh changed = %d, want 0", changed)
	}
	// modify main.go → re-extracted
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc A() {}\nfunc B() {}\n"), 0600)
	changed, err = s.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("after touch changed = %d, want 1", changed)
	}
	ex := s.Extractions()[filepath.Join(root, "main.go")]
	if len(ex.Nodes) == 0 {
		t.Fatal("main.go not re-extracted")
	}
}

func TestOpenCreatesDir(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".fender", "codeintel")); err != nil {
		t.Fatalf("codeintel dir missing: %v", err)
	}
	_ = s
}
