package codeintel

import "path/filepath"

// langSpec is the per-language extraction table (spec §5): one generic
// walker, one table per language. New language = one table + fixture test.
type langSpec struct {
	ext      []string
	symbol   map[string]string // node kind → symbol kind ("func"|"method"|"class"|"struct"|"interface"|"type"|"import")
	name     map[string]string // node kind → field name holding the identifier
	calls    []string          // call-expression node kinds
	callName string            // field name holding the called name
	imports  []string          // import node kinds
}

var specs = map[string]*langSpec{
	"go": {
		ext: []string{".go"},
		symbol: map[string]string{
			"func_declaration":   "func",
			"method_declaration": "method",
			"type_spec":          "type",
		},
		name: map[string]string{
			"func_declaration":   "name",
			"method_declaration": "name",
			"type_spec":          "name",
		},
		calls:    []string{"call_expression"},
		callName: "function",
		imports:  []string{"import_spec"},
	},
	"python": {
		ext: []string{".py"},
		symbol: map[string]string{
			"function_definition": "func",
			"class_definition":    "class",
		},
		name: map[string]string{
			"function_definition": "name",
			"class_definition":    "name",
		},
		calls:    []string{"call"},
		callName: "function",
		imports:  []string{"import_statement", "import_from_statement"},
	},
	"typescript": {
		ext: []string{".ts", ".tsx"},
		symbol: map[string]string{
			"function_declaration":  "func",
			"method_definition":     "method",
			"class_declaration":     "class",
			"interface_declaration": "interface",
		},
		name: map[string]string{
			"function_declaration":  "name",
			"method_definition":     "name",
			"class_declaration":     "name",
			"interface_declaration": "name",
		},
		calls:    []string{"call_expression"},
		callName: "function",
		imports:  []string{"import_statement"},
	},
	"javascript": {
		ext: []string{".js", ".jsx"},
		symbol: map[string]string{
			"function_declaration": "func",
			"method_definition":    "method",
			"class_declaration":    "class",
		},
		name: map[string]string{
			"function_declaration": "name",
			"method_definition":    "name",
			"class_declaration":    "name",
		},
		calls:    []string{"call_expression"},
		callName: "function",
		imports:  []string{"import_statement"},
	},
}

// specFor returns the language spec for a file path by extension.
func specFor(path string) (*langSpec, bool) {
	ext := filepath.Ext(path)
	for _, s := range specs {
		for _, e := range s.ext {
			if e == ext {
				return s, true
			}
		}
	}
	return nil, false
}
