package codeintel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractGo(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "sample.go"))
	if err != nil {
		t.Fatal(err)
	}
	ex, err := ExtractFile("sample.go", src)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, n := range ex.Nodes {
		kinds = append(kinds, n.Label)
	}
	joined := strings.Join(kinds, ",")
	for _, want := range []string{"Foo", "Bar", "Run"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing symbol %s: %v", want, kinds)
		}
	}
	var callEdges int
	for _, e := range ex.Edges {
		if e.Relation == "calls" && e.Confidence == "INFERRED" {
			callEdges++
		}
	}
	if callEdges == 0 {
		t.Fatalf("no inferred call edges: %+v", ex.Edges)
	}
	foundImport := false
	for _, e := range ex.Edges {
		if e.Relation == "imports" {
			foundImport = true
		}
	}
	if !foundImport {
		t.Fatal("no imports edge")
	}
}

func TestExtractUnknownExt(t *testing.T) {
	ex, err := ExtractFile("notes.txt", []byte("hello"))
	if err != nil || len(ex.Nodes) != 0 {
		t.Fatalf("ex=%+v err=%v", ex, err)
	}
}

func TestExtractPython(t *testing.T) {
	ex, err := ExtractFile("mod.py", []byte("def alpha():\n    return 1\n\nclass Beta:\n    def gamma(self):\n        return alpha()\n"))
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, n := range ex.Nodes {
		joined += n.Label + ","
	}
	for _, want := range []string{"alpha", "Beta", "gamma"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s: %s", want, joined)
		}
	}
}
