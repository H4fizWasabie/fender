package codeintel

import (
	"strings"
	"testing"
)

func testGraph() *Graph {
	ex := Extraction{
		Nodes: []Node{
			{ID: "go:func:internal/agent/a.go:Run", Label: "Run", SourceFile: "internal/agent/a.go", SourceLoc: "L10"},
			{ID: "go:method:internal/agent/a.go:Sub", Label: "Sub", SourceFile: "internal/agent/a.go", SourceLoc: "L20"},
			{ID: "go:func:internal/tools/b.go:Help", Label: "Help", SourceFile: "internal/tools/b.go", SourceLoc: "L3"},
			{ID: "go:func:internal/tools/b.go:Run", Label: "Run", SourceFile: "internal/tools/b.go", SourceLoc: "L9"},
		},
		Edges: []Edge{
			{Source: "go:func:internal/agent/a.go:Run", Target: "go:func:internal/tools/b.go:Help", Relation: "calls", Confidence: "EXTRACTED"},
		},
	}
	return Build(map[string]Extraction{"a.go": ex})
}

func TestSearch(t *testing.T) {
	g := testGraph()
	got := g.Search("run", 10)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2: %+v", len(got), got)
	}
}

func TestSymbolsAndCallers(t *testing.T) {
	g := testGraph()
	syms := g.Symbols("internal/agent/a.go")
	if len(syms) != 2 {
		t.Fatalf("symbols = %+v", syms)
	}
	callers := g.Callers("Help")
	if len(callers) != 1 || callers[0].Label != "Run" {
		t.Fatalf("callers = %+v", callers)
	}
	callees := g.Callees("Run")
	if len(callees) != 1 || callees[0].Label != "Help" {
		t.Fatalf("callees = %+v", callees)
	}
}

func TestGenerateMap(t *testing.T) {
	g := testGraph()
	m := g.GenerateMap()
	for _, want := range []string{"## internal/agent", "## internal/tools", "Run", "Help"} {
		if !strings.Contains(m, want) {
			t.Fatalf("map missing %q:\n%s", want, m)
		}
	}
}
