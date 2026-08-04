package codeintel

import (
	"strconv"
	"strings"

	"github.com/H4fizWasabie/fender/internal/tools"
)

// Searcher adapts the graph to the D10 search seam (tools.Searcher).
// The graph is reloaded per call — staleness bounded by the last refresh.
func (s *Store) Searcher() tools.Searcher {
	return func(q string) ([]tools.SearchResult, error) {
		g := s.LoadGraph()
		if g == nil {
			return nil, nil
		}
		nodes := g.Search(q, 50)
		out := make([]tools.SearchResult, 0, len(nodes))
		for _, n := range nodes {
			out = append(out, tools.SearchResult{
				Path: n.SourceFile,
				Line: lineOf(n.SourceLoc),
				Text: n.Label,
			})
		}
		return out, nil
	}
}

func lineOf(loc string) int {
	loc = strings.TrimPrefix(loc, "L")
	parts := strings.SplitN(loc, "-", 2)
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		return 1
	}
	return n
}
