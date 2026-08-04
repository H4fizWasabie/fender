package codeintel

import (
	"unsafe"

	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

func goLang() unsafe.Pointer     { return tree_sitter_go.Language() }
func pythonLang() unsafe.Pointer { return tree_sitter_python.Language() }
func tsLang() unsafe.Pointer     { return tree_sitter_typescript.LanguageTypescript() }
func jsLang() unsafe.Pointer     { return tree_sitter_javascript.Language() }
