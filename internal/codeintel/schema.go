// Package codeintel implements the code-intel core (D16): tree-sitter
// extraction → symbol index → call graph → query API, with graphify's
// node/edge schema and confidence labels (D20).
package codeintel

// Node is one symbol (graphify schema, D20).
type Node struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	SourceFile string `json:"source_file"`
	SourceLoc  string `json:"source_location"` // "L42" or "L12-C34"
}

// Edge is one relationship with a confidence label (D20).
type Edge struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Relation   string `json:"relation"`   // calls | imports | contains | module
	Confidence string `json:"confidence"` // EXTRACTED | INFERRED
}
