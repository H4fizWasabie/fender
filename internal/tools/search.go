package tools

// Searcher is the codebase-search backend seam (D10): graphify/cce/codegraph
// plug in here later; v1 ships the walk-based default (Task 4).
type Searcher func(query string) ([]SearchResult, error)

// SearchResult is one match: file, 1-based line, matching text.
type SearchResult struct {
	Path string
	Line int
	Text string
}
