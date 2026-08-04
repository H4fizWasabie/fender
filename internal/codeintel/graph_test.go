package codeintel

import "testing"

func TestBuildResolvesCalls(t *testing.T) {
	ex := Extraction{
		Nodes: []Node{
			{ID: "go:func:a.go:A", Label: "A", SourceFile: "a.go", SourceLoc: "L1"},
			{ID: "go:func:b.go:B", Label: "B", SourceFile: "b.go", SourceLoc: "L1"},
		},
		Edges: []Edge{
			{Source: "go:func:a.go:A", Target: "B", Relation: "calls", Confidence: "INFERRED"},
			{Source: "go:func:a.go:A", Target: "C", Relation: "calls", Confidence: "INFERRED"},
		},
	}
	g := Build(map[string]Extraction{"a.go": ex})
	found := false
	for _, e := range g.Edges {
		if e.Source == "go:func:a.go:A" && e.Relation == "calls" {
			if e.Target == "go:func:b.go:B" && e.Confidence == "EXTRACTED" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("resolved call edge missing: %+v", g.Edges)
	}
	unresolved := false
	for _, e := range g.Edges {
		if e.Source == "go:func:a.go:A" && e.Target == "C" && e.Confidence == "INFERRED" {
			unresolved = true
		}
	}
	if !unresolved {
		t.Fatalf("unresolved call edge missing: %+v", g.Edges)
	}
}

func TestBuildSkipsSelfEdges(t *testing.T) {
	ex := Extraction{
		Nodes: []Node{{ID: "go:func:a.go:A", Label: "A", SourceFile: "a.go"}},
		Edges: []Edge{{Source: "go:func:a.go:A", Target: "A", Relation: "calls", Confidence: "INFERRED"}},
	}
	g := Build(map[string]Extraction{"a.go": ex})
	for _, e := range g.Edges {
		if e.Relation == "calls" {
			t.Fatalf("self edge survived: %+v", e)
		}
	}
}
