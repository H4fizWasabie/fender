package codeintel

import (
	"fmt"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// Extraction is one file's symbols + edges (cached per file).
type Extraction struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// ExtractFile parses src with the language spec for path. Unknown
// extensions → empty extraction, nil error.
func ExtractFile(path string, src []byte) (Extraction, error) {
	spec, ok := specFor(path)
	if !ok {
		return Extraction{}, nil
	}
	lang, err := languageFor(spec)
	if err != nil {
		return Extraction{}, err
	}
	parser := sitter.NewParser()
	if err := parser.SetLanguage(lang); err != nil {
		return Extraction{}, err
	}
	tree := parser.Parse(src, nil)
	if tree == nil {
		return Extraction{}, fmt.Errorf("parse %s: nil tree", path)
	}
	return walk(path, src, tree.RootNode(), spec), nil
}

func languageFor(spec *langSpec) (*sitter.Language, error) {
	switch {
	case contains(spec.ext, ".go"):
		return sitter.NewLanguage(goLang()), nil
	case contains(spec.ext, ".py"):
		return sitter.NewLanguage(pythonLang()), nil
	case contains(spec.ext, ".ts"), contains(spec.ext, ".tsx"):
		return sitter.NewLanguage(tsLang()), nil
	default:
		return sitter.NewLanguage(jsLang()), nil
	}
}
