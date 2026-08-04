package codeintel

import (
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// walk collects symbols + edges from one parse tree.
// Edges at extraction time: imports EXTRACTED; calls INFERRED with the
// called name as target (graph build resolves same-module to EXTRACTED).
func walk(path string, src []byte, root *sitter.Node, spec *langSpec) Extraction {
	var ex Extraction
	fileID := fmt.Sprintf("%s:file:%s", fileExt(path), path)
	ex.Nodes = append(ex.Nodes, Node{ID: fileID, Label: filepath.Base(path), SourceFile: path, SourceLoc: "L1"})
	walkNode(path, src, root, spec, &ex, "")
	return ex
}

func walkNode(path string, src []byte, n *sitter.Node, spec *langSpec, ex *Extraction, enclosing string) {
	kind := n.Kind()
	if skind, ok := spec.symbol[kind]; ok {
		nameNode := n.ChildByFieldName(spec.name[kind])
		if nameNode != nil {
			name := text(src, nameNode)
			row := n.StartPosition().Row
			id := fmt.Sprintf("%s:%s:%s:%s", fileExt(path), skind, path, name)
			ex.Nodes = append(ex.Nodes, Node{ID: id, Label: name, SourceFile: path, SourceLoc: fmt.Sprintf("L%d", row+1)})
			enclosing = id
		}
	}
	if contains(spec.imports, kind) {
		imp := strings.Trim(text(src, n), "\"'")
		if i := strings.Index(imp, "."); i > 0 && strings.HasPrefix(imp, "from ") {
			imp = strings.TrimPrefix(imp, "from ")
		}
		ex.Edges = append(ex.Edges, Edge{Source: enclosingOrFile(ex, path, enclosing), Target: imp, Relation: "imports", Confidence: "EXTRACTED"})
	}
	if contains(spec.calls, kind) {
		target := n.ChildByFieldName(spec.callName)
		if target != nil {
			name := text(src, target)
			ex.Edges = append(ex.Edges, Edge{Source: enclosingOrFile(ex, path, enclosing), Target: name, Relation: "calls", Confidence: "INFERRED"})
		}
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		walkNode(path, src, n.Child(i), spec, ex, enclosing)
	}
}

func enclosingOrFile(ex *Extraction, path, enclosing string) string {
	if enclosing != "" {
		return enclosing
	}
	return fmt.Sprintf("%s:file:%s", fileExt(path), path)
}

func text(src []byte, n *sitter.Node) string {
	start, end := n.ByteRange()
	return string(src[start:end])
}

func fileExt(path string) string {
	return strings.TrimPrefix(filepath.Ext(path), ".")
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
