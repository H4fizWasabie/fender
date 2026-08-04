package codeintel

import "sort"

// Graph is the in-memory symbol graph: nodes by id, edges, and a
// label→ids index for call resolution (D20 pipeline: build stage).
type Graph struct {
	Nodes         map[string]Node     `json:"nodes"`
	Edges         []Edge              `json:"edges"`
	symbolsByName map[string][]string `json:"-"`
}

// Build unions all extractions and resolves call edges: a call target that
// matches a same-project symbol label is retargeted to that symbol's id with
// confidence EXTRACTED; name-only targets stay INFERRED (D20, spec §3.7).
func Build(extractions map[string]Extraction) *Graph {
	g := &Graph{Nodes: map[string]Node{}, symbolsByName: map[string][]string{}}
	for _, ex := range extractions {
		for _, n := range ex.Nodes {
			g.Nodes[n.ID] = n
			g.symbolsByName[n.Label] = append(g.symbolsByName[n.Label], n.ID)
		}
	}
	// deterministic resolution: sort ids per label (paths sort fine)
	for label := range g.symbolsByName {
		sort.Strings(g.symbolsByName[label])
	}
	for _, ex := range extractions {
		for _, e := range ex.Edges {
			ne := e
			if e.Relation == "calls" {
				if ids := g.symbolsByName[e.Target]; len(ids) > 0 {
					ne.Target = ids[0]
					ne.Confidence = "EXTRACTED"
				}
			}
			if ne.Source == ne.Target {
				continue // self-edge after resolution (recursion) — noise
			}
			g.Edges = append(g.Edges, ne)
		}
	}
	return g
}
