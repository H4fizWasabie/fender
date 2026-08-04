package codeintel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearcherAdapter(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc RunLoop() {}\n"), 0600)
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Refresh(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rebuild(); err != nil {
		t.Fatal(err)
	}
	searcher := s.Searcher()
	res, err := searcher("RunLoop")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Path != filepath.Join(root, "main.go") {
		t.Fatalf("results = %+v", res)
	}
}
