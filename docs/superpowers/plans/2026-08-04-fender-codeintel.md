# Fender Plan 7: Code-Intelligence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `internal/codeintel` — tree-sitter extraction (Go/Python/TS/JS) → JSON incremental store → call graph → query API → MAP.md generation, wired as the D10 search backend.

**Architecture:** One generic tree-sitter walker driven by per-language `langSpec` tables. Store = JSON files in `.fender/codeintel/` (`stamps.json`, `extractions.json`, `graph.json`); refresh re-extracts only stamp-changed files and rebuilds the graph from the extraction cache. Query API feeds the existing `tools.Searcher` function seam. MAP.md body generated per ticket-05 schema.

**Tech Stack:** Go 1.22, `github.com/tree-sitter/go-tree-sitter v0.24.0` + grammar modules (go, python, javascript, typescript — pinned pseudo-versions, all go1.22-compatible; cgo requires gcc). Stdlib JSON. Deps already pinned in go.mod (verified spike: parser/language/node API below).

## Global Constraints

- **Read `AGENTS.md`, `DECISIONS.md` (D16/D19/D20/D34-3), ticket-07 spec first.**
- **Every commit MUST stage `CHANGELOG.md`** — enforced by `.githooks/pre-commit`.
- **Allowed deps:** BurntSushi/toml, mvdan.cc/sh/v3, tree-sitter family (pinned below), modernc.org/sqlite. Nothing else.
- **cgo accepted** (tree-sitter C core). Build requires gcc. Single binary preserved.
- **Verified tree-sitter API (v0.24.0):** `sitter.NewParser()` / `parser.SetLanguage(sitter.NewLanguage(<g>.Language()))` / `parser.Parse(src, nil) *Tree` / `tree.RootNode()` / `n.Kind()` / `n.Child(i uint)` / `n.ChildCount() uint` / `n.NamedChild(i uint)` / `n.ChildByFieldName(f)` / `n.StartPosition().Row` / `n.EndPosition().Row` / `n.ByteRange() (uint, uint)` — node text = `src[start:end]`. Grammar packages expose `Language() unsafe.Pointer`.
- Module path `github.com/H4fizWasabie/fender`; files under `internal/codeintel/`.

---

### Task 1: Dependencies + skeleton (langSpec, dispatch)

**Files:**
- Modify: `go.mod` (already pinned — verify)
- Create: `internal/codeintel/schema.go`
- Create: `internal/codeintel/lang.go`
- Create: `internal/codeintel/schema_test.go`

**Interfaces:**
- Produces:
  - `type Node struct { ID, Label, SourceFile, SourceLoc string }` (json tags `id`, `label`, `source_file`, `source_location`)
  - `type Edge struct { Source, Target, Relation, Confidence string }` (json: `source`, `target`, `relation`, `confidence`)
  - `type langSpec struct { ext []string; symbol map[string]string; name map[string]string; calls []string; callName string; imports []string }` (spec §5)
  - `func specFor(path string) (*langSpec, bool)` — by extension: `.go` → go, `.py` → python, `.ts`/`.tsx` → typescript, `.js`/`.jsx` → javascript; else false

- [ ] **Step 1: Verify pinned deps build**

```bash
go build ./... && go test ./...
```

Expected: green (deps already pinned; `go.mod` says `go 1.22`, tree-sitter v0.24.0 + grammar pseudo-versions).

- [ ] **Step 2: Write the failing test**

`internal/codeintel/schema_test.go`:

```go
package codeintel

import "testing"

func TestSpecFor(t *testing.T) {
	cases := []struct {
		path string
		want string
		ok   bool
	}{
		{"a.go", "go", true},
		{"b.py", "python", true},
		{"c.ts", "typescript", true},
		{"d.tsx", "typescript", true},
		{"e.js", "javascript", true},
		{"f.jsx", "javascript", true},
		{"g.rs", "", false},
		{"h.txt", "", false},
	}
	for _, c := range cases {
		spec, ok := specFor(c.path)
		if ok != c.ok || (ok && spec.name == "") {
			t.Fatalf("%s: ok=%v spec=%+v", c.path, ok, spec)
		}
	}
}
```

- [ ] **Step 3: Run to verify fail**

Run: `go test ./internal/codeintel/ -v`
Expected: FAIL — no package files.

- [ ] **Step 4: Write schema + langSpec**

`internal/codeintel/schema.go`:

```go
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
	Relation   string `json:"relation"`  // calls | imports | contains | module
	Confidence string `json:"confidence"` // EXTRACTED | INFERRED
}
```

`internal/codeintel/lang.go`:

```go
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
			"function_declaration":  "func",
			"method_declaration": "method",
			"type_spec":          "type",
		},
		name: map[string]string{
			"function_declaration":   "name",
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
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/codeintel/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/codeintel/ CHANGELOG.md
git commit -m "feat: codeintel skeleton (schema, langSpec tables, extension dispatch) + pinned tree-sitter deps"
```

CHANGELOG:

```markdown
### Added
- codeintel skeleton: graphify node/edge schema, langSpec tables (go/python/typescript/javascript), specFor dispatch
- Pinned tree-sitter deps: go-tree-sitter v0.24.0 + grammar pseudo-versions (go1.22 compatible, cgo)
```

---

### Task 2: Extractor — generic walker + Go fixture

**Files:**
- Create: `internal/codeintel/extract.go`
- Create: `internal/codeintel/extract_test.go`
- Create: `internal/codeintel/testdata/sample.go`

**Interfaces:**
- Consumes: `Node`, `Edge`, `langSpec` (Task 1).
- Produces:
  - `type Extraction struct { Nodes []Node; Edges []Edge }` (json tags)
  - `func ExtractFile(path string, src []byte) (Extraction, error)` — dispatch by specFor; unknown extension → empty Extraction, nil error. Walk: symbol nodes (kind ∈ symbol map) → Node{id: `lang:kind:path:name`, label: name, source_file: path, source_loc: `L<row>`}; call nodes → Edge{source: enclosing-symbol id, target: called name, relation: "calls", confidence: "INFERRED"} (name-only at extraction; graph build resolves to EXTRACTED when same-module); imports → Edge{source: enclosing symbol, target: imported name, relation: "imports", confidence: "EXTRACTED"}; file/module containment edges in graph build (not here).

- [ ] **Step 1: Write the failing test**

`internal/codeintel/extract_test.go`:

```go
package codeintel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractGo(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "sample.go"))
	if err != nil {
		t.Fatal(err)
	}
	ex, err := ExtractFile("sample.go", src)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, n := range ex.Nodes {
		kinds = append(kinds, n.Label)
	}
	joined := strings.Join(kinds, ",")
	for _, want := range []string{"Foo", "Bar", "Run"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing symbol %s: %v", want, kinds)
		}
	}
	// Bar must have a calls edge (INFERRED at extraction)
	var callEdges int
	for _, e := range ex.Edges {
		if e.Relation == "calls" && e.Confidence == "INFERRED" {
			callEdges++
		}
	}
	if callEdges == 0 {
		t.Fatalf("no inferred call edges: %+v", ex.Edges)
	}
	// import edge present
	foundImport := false
	for _, e := range ex.Edges {
		if e.Relation == "imports" {
			foundImport = true
		}
	}
	if !foundImport {
		t.Fatal("no imports edge")
	}
}

func TestExtractUnknownExt(t *testing.T) {
	ex, err := ExtractFile("notes.txt", []byte("hello"))
	if err != nil || len(ex.Nodes) != 0 {
		t.Fatalf("ex=%+v err=%v", ex, err)
	}
}

func TestExtractPython(t *testing.T) {
	ex, err := ExtractFile("mod.py", []byte("def alpha():\n    return 1\n\nclass Beta:\n    def gamma(self):\n        return alpha()\n"))
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, n := range ex.Nodes {
		joined += n.Label + ","
	}
	for _, want := range []string{"alpha", "Beta", "gamma"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s: %s", want, joined)
		}
	}
}
```

`internal/codeintel/testdata/sample.go`:

```go
package sample

import "fmt"

type Foo struct{ X int }

func (f *Foo) Bar() int { return f.X }

func Run() {
	f := &Foo{X: 1}
	fmt.Println(f.Bar())
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/codeintel/ -run TestExtract -v`
Expected: FAIL — `ExtractFile` undefined.

- [ ] **Step 3: Write the extractor**

`internal/codeintel/extract.go`:

```go
package codeintel

import (
	"fmt"
	"path/filepath"
	"strings"

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
```

Grammar loader helpers (`internal/codeintel/grammars.go`):

```go
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
```

Note: verify the typescript binding function name at implementation time (`LanguageTypescript` vs `Language` — the TS module has both TS and TSX). Adjust the call if the name differs.

The walker (`internal/codeintel/walk.go`):

```go
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
	// file node
	ex.Nodes = append(ex.Nodes, Node{ID: fileID, Label: filepath.Base(path), SourceFile: path, SourceLoc: "L1"})
	ex.Edges = append(ex.Edges, Edge{Source: fileID, Target: fileID, Relation: "module", Confidence: "EXTRACTED"})

	// find enclosing symbol per node (name-stack for call edges)
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
			ex.Edges = append(ex.Edges, Edge{Source: id, Target: id, Relation: "contains", Confidence: "EXTRACTED"})
			enclosing = id
		}
	}
	if contains(spec.imports, kind) {
		// import edge from the enclosing symbol (or file) to the imported name
		imp := strings.Trim(text(src, n), "\"'")
		imp = strings.TrimPrefix(imp, "fmt.") // python: from X import Y -> X
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
```

Note: Go's `import_spec` node text is `"fmt"` (quoted) — trimmed. Python's `import_from_statement` text is `from X import Y` — the whole statement is used as target; acceptable for v1 (module edges are the graph-build stage's job). The `module` edge for the file uses source=target=fileID — graph build re-points it.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/codeintel/ -run TestExtract -v`
Expected: PASS. (Fix the typescript binding name if vet complains.)

- [ ] **Step 5: Commit**

```bash
git add internal/codeintel/ CHANGELOG.md
git commit -m "feat: tree-sitter extractor (generic walker, go/python/js/ts grammars)"
```

CHANGELOG:

```markdown
### Added
- Extractor: ExtractFile — generic walker over langSpec tables, symbol nodes, calls (INFERRED) + imports (EXTRACTED) edges
```

---

### Task 3: Store + incremental refresh

**Files:**
- Create: `internal/codeintel/store.go`
- Create: `internal/codeintel/store_test.go`

**Interfaces:**
- Consumes: `Extraction`, `ExtractFile` (Task 2).
- Produces:
  - `type Stamp struct { MtimeNanos int64; Size int64 }`
  - `type Store struct` with:
    - `func Open(root string) (*Store, error)` — dir `.fender/codeintel/` under root, created if missing
    - `func (s *Store) Refresh() (int, error)` — walk project files (skip `.git`, `.fender`, `vendor`, `node_modules`, hidden dirs); stamp-compare; re-extract dirty; persist stamps + extractions; return changed count
    - `func (s *Store) Extractions() map[string]Extraction`
    - `func (s *Store) Rebuild() (*Graph, error)` — graph build (Task 4) + persist graph.json

- [ ] **Step 1: Write the failing test**

`internal/codeintel/store_test.go`:

```go
package codeintel

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0700)
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRefreshIncremental(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"main.go":        "package main\nfunc A() {}\n",
		"notes.txt":      "ignore me",
		".fender/x.txt":  "skip me",
		"vendor/y.go":    "package y\nfunc Y() {}\n",
		"node_modules/z.go": "package z\n",
	})
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := s.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("first refresh changed = %d, want 1", changed)
	}
	// second refresh: nothing changed
	changed, err = s.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	if changed != 0 {
		t.Fatalf("second refresh changed = %d, want 0", changed)
	}
	// touch main.go → re-extracted
	os.Chtimes(filepath.Join(root, "main.go"), os.Mtime, os.Mtime)
	// (Chtimes with same time may not change mtime; use write instead)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc A() {}\nfunc B() {}\n"), 0600)
	changed, err = s.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("after touch changed = %d, want 1", changed)
	}
	ex := s.Extractions()["main.go"]
	if len(ex.Nodes) == 0 {
		t.Fatal("main.go not re-extracted")
	}
}

func TestOpenCreatesDir(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".fender", "codeintel")); err != nil {
		t.Fatalf("codeintel dir missing: %v", err)
	}
	_ = s
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/codeintel/ -run TestRefresh -v`
Expected: FAIL — `Open` undefined.

- [ ] **Step 3: Write the store**

`internal/codeintel/store.go`:

```go
package codeintel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Stamp struct {
	MtimeNanos int64 `json:"mtime_nanos"`
	Size       int64 `json:"size"`
}

// Store owns .fender/codeintel/: stamps, extraction cache, graph (D34-3).
type Store struct {
	dir          string
	projectDir   string
	stamps       map[string]Stamp
	extractions  map[string]Extraction
	graph        *Graph
}

func Open(root string) (*Store, error) {
	dir := filepath.Join(root, ".fender", "codeintel")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, projectDir: root, stamps: map[string]Stamp{}, extractions: map[string]Extraction{}}
	s.load()
	return s, nil
}

func (s *Store) load() {
	if data, err := os.ReadFile(filepath.Join(s.dir, "stamps.json")); err == nil {
		json.Unmarshal(data, &s.stamps)
	}
	if data, err := os.ReadFile(filepath.Join(s.dir, "extractions.json")); err == nil {
		json.Unmarshal(data, &s.extractions)
	}
}

func (s *Store) save() error {
	stamps, err := json.Marshal(s.stamps)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.dir, "stamps.json"), stamps, 0600); err != nil {
		return err
	}
	ex, err := json.Marshal(s.extractions)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "extractions.json"), ex, 0600)
}

// skipDirs are never indexed (build/vendor noise).
var skipDirs = map[string]bool{".git": true, ".fender": true, "vendor": true, "node_modules": true, ".venv": true, "dist": true, "build": true}

// Refresh re-extracts only stamp-changed files (D34-3, graphify cache.py
// pattern). Returns the number of files re-extracted.
func (s *Store) Refresh() (int, error) {
	changed := 0
	err := filepath.WalkDir(s.projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != s.projectDir && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if _, ok := specFor(path); !ok {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		st := Stamp{MtimeNanos: info.ModTime().UnixNano(), Size: info.Size()}
		if prev, ok := s.stamps[path]; ok && prev == st {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		ex, err := ExtractFile(path, src)
		if err != nil {
			return nil
		}
		s.extractions[path] = ex
		s.stamps[path] = st
		changed++
		return nil
	})
	if err != nil {
		return changed, err
	}
	// prune stamps/extractions for deleted files
	for path := range s.stamps {
		if _, err := os.Stat(filepath.Join(s.projectDir, path)); err != nil {
			delete(s.stamps, path)
			delete(s.extractions, path)
		}
	}
	if changed > 0 {
		return changed, s.save()
	}
	return changed, nil
}

func (s *Store) Extractions() map[string]Extraction { return s.extractions }

func (s *Store) Rebuild() (*Graph, error) {
	g := Build(s.extractions)
	s.graph = g
	data, err := json.Marshal(g)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(s.dir, "graph.json"), data, 0600); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *Store) LoadGraph() *Graph {
	if s.graph != nil {
		return s.graph
	}
	if data, err := os.ReadFile(filepath.Join(s.dir, "graph.json")); err == nil {
		g := &Graph{}
		json.Unmarshal(data, g)
		s.graph = g
	}
	return s.graph
}

var _ = sort.Strings // reserved
var _ = time.Now
```

Note: drop the reserved `var _` lines if unused. `Stamp` comparison relies on `==` of two structs (comparable — int64 fields, fine).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/codeintel/ -run TestRefresh -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/codeintel/ CHANGELOG.md
git commit -m "feat: incremental store (stamps, extraction cache, refresh, graph persist)"
```

CHANGELOG:

```markdown
### Added
- Store: Open/Refresh (mtime+size stamps, dirty-only re-extract, skip dirs), extractions cache, graph.json persist
```

---

### Task 4: Graph build + confidence resolution

**Files:**
- Create: `internal/codeintel/graph.go`
- Create: `internal/codeintel/graph_test.go`

**Interfaces:**
- Consumes: `Extraction`, `Node`, `Edge` (Tasks 1–2).
- Produces:
  - `type Graph struct { Nodes map[string]Node; Edges []Edge; symbolsByName map[string][]string }`
  - `func Build(extractions map[string]Extraction) *Graph` — union nodes; re-point module edges to file nodes; resolve calls: target matches a same-project symbol label → retarget to its id, confidence EXTRACTED; else keep name-target INFERRED. Skip self-edges (source==target). Persistable (json round-trip).

- [ ] **Step 1: Write the failing test**

`internal/codeintel/graph_test.go`:

```go
package codeintel

import (
	"testing"
)

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
	// A → B resolves to EXTRACTED against b.go's B
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
	// A → C stays INFERRED name-target
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
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/codeintel/ -run TestBuild -v`
Expected: FAIL — `Build` undefined.

- [ ] **Step 3: Write the graph**

`internal/codeintel/graph.go`:

```go
package codeintel

// Graph is the in-memory symbol graph: nodes by id, edges, and a
// label→ids index for call resolution (D20 pipeline: build stage).
type Graph struct {
	Nodes          map[string]Node   `json:"nodes"`
	Edges          []Edge            `json:"edges"`
	symbolsByName  map[string][]string `json:"-"`
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
	for _, ex := range extractions {
		for _, e := range ex.Edges {
			if e.Source == e.Target {
				continue // self-edge (recursion) — noise
			}
			ne := e
			if e.Relation == "calls" {
				if ids := g.symbolsByName[e.Target]; len(ids) > 0 {
					ne.Target = ids[0]
					ne.Confidence = "EXTRACTED"
				}
			}
			g.Edges = append(g.Edges, ne)
		}
	}
	return g
}
```

Note: `symbolsByName` takes the FIRST id for a label (file order is map-iterated — nondeterministic). Fix: sort ids per label at build end (deterministic first hit). Implementer: after collecting, sort each label's ids (they're all `lang:kind:path:name` — path sorts fine). This keeps tests deterministic.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/codeintel/ -run TestBuild -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/codeintel/ CHANGELOG.md
git commit -m "feat: graph build with call resolution (INFERRED→EXTRACTED same-project)"
```

CHANGELOG:

```markdown
### Added
- Graph: Build() — node union, self-edge skip, call resolution to same-project symbols (EXTRACTED), name-only stays INFERRED
```

---

### Task 5: Query API + MAP.md generation

**Files:**
- Create: `internal/codeintel/query.go`
- Create: `internal/codeintel/query_test.go`

**Interfaces:**
- Consumes: `Graph`, `Node` (Task 4).
- Produces:
  - `func (g *Graph) Search(q string, limit int) []Node` — case-insensitive substring on Label; sort by label; limit
  - `func (g *Graph) Symbols(path string) []Node` — nodes with SourceFile == path, sorted by SourceLoc
  - `func (g *Graph) Callers(name string) []Node` — nodes that call the symbol (by label or id)
  - `func (g *Graph) Callees(name string) []Node` — nodes the symbol calls
  - `func (g *Graph) GenerateMap() string` — MAP.md body: `## <top-level-dir>` sections; per module: package doc comment (from the module's file node label — v1: first file's first symbol comment not extracted; use file list + exported symbol count), key files (top 5 by symbol count), exported symbols (top 20, `- <label> (<kind>)`)

- [ ] **Step 1: Write the failing test**

`internal/codeintel/query_test.go`:

```go
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
	for _, want := range []string{"## agent", "## tools", "Run", "Help"} {
		if !strings.Contains(m, want) {
			t.Fatalf("map missing %q:\n%s", want, m)
		}
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/codeintel/ -run "TestSearch|TestSymbols|TestGenerateMap" -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Write the query API**

`internal/codeintel/query.go`:

```go
package codeintel

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Search finds symbols whose label contains q (case-insensitive).
func (g *Graph) Search(q string, limit int) []Node {
	var out []Node
	q = strings.ToLower(q)
	for _, n := range g.Nodes {
		if strings.Contains(strings.ToLower(n.Label), q) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Symbols lists a file's nodes, sorted by source location.
func (g *Graph) Symbols(path string) []Node {
	var out []Node
	for _, n := range g.Nodes {
		if n.SourceFile == path {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SourceLoc < out[j].SourceLoc })
	return out
}

// Callers returns the nodes that call the symbol with the given label or id.
func (g *Graph) Callers(name string) []Node {
	var out []Node
	targets := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Label == name || n.ID == name {
			targets[n.ID] = true
		}
	}
	for _, e := range g.Edges {
		if e.Relation == "calls" && targets[e.Target] {
			if n, ok := g.Nodes[e.Source]; ok {
				out = append(out, n)
			}
		}
	}
	return out
}

// Callees returns the nodes the symbol calls.
func (g *Graph) Callees(name string) []Node {
	var out []Node
	srcs := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Label == name || n.ID == name {
			srcs[n.ID] = true
		}
	}
	for _, e := range g.Edges {
		if e.Relation == "calls" && srcs[e.Source] {
			if n, ok := g.Nodes[e.Target]; ok {
				out = append(out, n)
			}
		}
	}
	return out
}

// GenerateMap produces the MAP.md body (ticket-05 schema): one
// "## <module>" section per top-level directory.
func (g *Graph) GenerateMap() string {
	type module struct {
		dir     string
		files   map[string]int // file → symbol count
		symbols []Node
	}
	mods := map[string]*module{}
	var order []string
	for _, n := range g.Nodes {
		if n.Label == filepath.Base(n.SourceFile) && strings.Contains(n.ID, ":file:") {
			continue // file nodes are not symbols
		}
		top := topDir(n.SourceFile)
		m, ok := mods[top]
		if !ok {
			m = &module{dir: top, files: map[string]int{}}
			mods[top] = m
			order = append(order, top)
		}
		m.symbols = append(m.symbols, n)
		m.files[n.SourceFile]++
	}
	sort.Strings(order)
	var sb strings.Builder
	sb.WriteString("# MAP.md — navigation (Layer 1)\n\n")
	sb.WriteString("_Generated by code-intel (ticket 07). Schema per ticket 05 — hand-edits to the body are overwritten on regeneration._\n\n")
	for _, dir := range order {
		m := mods[dir]
		fmt.Fprintf(&sb, "## %s\n\n", m.dir)
		// key files (top 5 by symbol count)
		type fc struct {
			path string
			n    int
		}
		var files []fc
		for p, n := range m.files {
			files = append(files, fc{p, n})
		}
		sort.Slice(files, func(i, j int) bool { return files[i].n > files[j].n })
		if len(files) > 5 {
			files = files[:5]
		}
		for _, f := range files {
			fmt.Fprintf(&sb, "- `%s` (%d symbols)\n", f.path, f.n)
		}
		// exported symbols (top 20)
		sort.Slice(m.symbols, func(i, j int) bool { return m.symbols[i].Label < m.symbols[j].Label })
		limit := len(m.symbols)
		if limit > 20 {
			limit = 20
		}
		sb.WriteString("\nSymbols:\n")
		for _, n := range m.symbols[:limit] {
			fmt.Fprintf(&sb, "- %s (%s)\n", n.Label, kindOf(n.ID))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func topDir(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) > 1 {
		return parts[0]
	}
	return "root"
}

func kindOf(id string) string {
	parts := strings.Split(id, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return "?"
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/codeintel/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/codeintel/ CHANGELOG.md
git commit -m "feat: query API (Search/Symbols/Callers/Callees) + MAP.md generation"
```

CHANGELOG:

```markdown
### Added
- Query API: Search (substring), Symbols(path), Callers/Callees; GenerateMap (ticket-05 schema)
```

---

### Task 6: Searcher adapter + `fender intel` CLI

**Files:**
- Create: `internal/codeintel/searcher.go`
- Create: `internal/codeintel/searcher_test.go`
- Modify: `cmd/fender/main.go`

**Interfaces:**
- Consumes: `Store`, `Graph` (Tasks 3–5), `tools.SearchResult` (existing).
- Produces:
  - `func (s *Store) Searcher() tools.Searcher` — closure: Search(query) → []SearchResult{Path, Line, Text}; graph loaded via LoadGraph (refresh-aware); line = SourceLoc row
  - CLI: `fender intel refresh` (Refresh + Rebuild), `fender intel search <q>` (Searcher over the graph), `fender intel map` (Rebuild + GenerateMap → write `.fender/memory/MAP.md`)

- [ ] **Step 1: Write the failing test**

`internal/codeintel/searcher_test.go`:

```go
package codeintel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearcherAdapter(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc RunLoop() {}\n"), 0600)
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Refresh(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rebuild(); err != nil {
		t.Fatal(err)
	}
	searcher := s.Searcher()
	res, err := searcher("RunLoop")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Path != filepath.Join(root, "main.go") {
		t.Fatalf("results = %+v", res)
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/codeintel/ -run TestSearcher -v`
Expected: FAIL — `Searcher` undefined.

- [ ] **Step 3: Write the adapter**

`internal/codeintel/searcher.go`:

```go
package codeintel

import (
	"path/filepath"
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
				Path: filepath.Join(s.projectDir, n.SourceFile),
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
```

- [ ] **Step 4: Wire the CLI**

In `cmd/fender/main.go` — add to the usage lines and switch:

```go
	case "intel":
		return intelCommand(out, fs.Args()[1:])
```

```go
func intelCommand(out io.Writer, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: fender intel <refresh|search|map>")
	}
	s, err := codeintel.Open(".")
	if err != nil {
		return err
	}
	switch args[0] {
	case "refresh":
		n, err := s.Refresh()
		if err != nil {
			return err
		}
		if _, err := s.Rebuild(); err != nil {
			return err
		}
		fmt.Fprintf(out, "refreshed %d file(s)\n", n)
	case "search":
		if len(args) < 2 {
			return fmt.Errorf("usage: fender intel search <query>")
		}
		searcher := s.Searcher()
		res, err := searcher(strings.Join(args[1:], " "))
		if err != nil {
			return err
		}
		for _, r := range res {
			fmt.Fprintf(out, "%s:%d: %s\n", r.Path, r.Line, r.Text)
		}
	case "map":
		if _, err := s.Refresh(); err != nil {
			return err
		}
		g, err := s.Rebuild()
		if err != nil {
			return err
		}
		body := g.GenerateMap()
		mapPath := filepath.Join(".fender", "memory", "MAP.md")
		if err := os.WriteFile(mapPath, []byte(body), 0600); err != nil {
			return err
		}
		fmt.Fprintf(out, "wrote %s\n", mapPath)
	default:
		return fmt.Errorf("unknown intel command %q", args[0])
	}
	return nil
}
```

Add imports: `os`, `github.com/H4fizWasabie/fender/internal/codeintel`. Update usage text with `intel refresh|search|map`.

- [ ] **Step 5: Run tests + smoke**

```bash
go test ./internal/codeintel/ ./cmd/fender/ -v
go build ./cmd/fender && ./fender intel refresh && ./fender intel search "Run" | head -5 && rm -f fender
```

Expected: tests pass; refresh indexes Fender itself; search finds symbols. NOTE: `.fender/` in the Fender repo will now contain codeintel data — add `.fender/` to `.gitignore` if not already there (check: memory workspace should not be committed).

- [ ] **Step 6: Commit**

```bash
git add internal/codeintel/ cmd/fender/ .gitignore CHANGELOG.md
git commit -m "feat: fender intel CLI (refresh/search/map) + Searcher adapter"
```

CHANGELOG:

```markdown
### Added
- `fender intel refresh|search|map`: incremental index, symbol search, MAP.md generation; Searcher adapter for the D10 seam
```

---

### Task 7: Wayfinder resolve + frontier

**Files:**
- Modify: `.scratch/fender/issues/07-CodeIntel.md`
- Modify: `.scratch/fender/map.md`

- [ ] **Step 1: Full verification**

```bash
go build ./... && go vet ./... && go test ./...
```

Expected: build clean, vet clean, all tests PASS. Then commit a smoke of the map generation: `./fender intel map` writes `.fender/memory/MAP.md` with `## agent`, `## tools` sections (verify by reading it back).

- [ ] **Step 2: Resolve the ticket** (mirror tickets 05/06 Answer format: delivered, test count, fixes, unblocks)

- [ ] **Step 3: Update the map's decisions index**

- [ ] **Step 4: Commit**

```bash
git add .scratch/fender/ CHANGELOG.md
git commit -m "docs: resolve wayfinder ticket 07 (codeintel done, frontier 08)"
```

CHANGELOG:

```markdown
### Changed
- Wayfinder: ticket 07 resolved — codeintel delivered; frontier → 08 (CLI + UI)
```

---

## Self-Review Notes

- **Spec coverage:** §1 scope 1–7 → Tasks 1–6; §3 decisions 1–9 → Tasks 1–6; §4 API → Tasks 1–6; §6 test table → each task's tests by name; §7 acceptance → Task 7. Non-goals (§2) explicitly not built (no cluster/embeddings/reports).
- **Placeholders:** none — every code step has full source. Two flagged adaptation points (verified by spike, but confirm at implementation): (a) typescript binding name (`LanguageTypescript` vs `Language`); (b) graph `symbolsByName` determinism (sort ids per label).
- **Type consistency:** `Node`/`Edge`/`Extraction`/`Stamp`/`Graph`/`Store` signatures consistent across tasks; `tools.SearchResult`/`tools.Searcher` are existing types (seam, no signature change). Node ids `lang:kind:path:name` consistent in walker, graph, query.
- **CHANGELOG:** every task ends with an entry + commit (hook-enforced).
- **Deps:** tree-sitter family pinned in Task 1 (go1.22-compatible, verified via proxy go.mod inspection + spike build).
- **Known simplifications (ponytail):** import edges use raw statement text as target (module resolution is a later refinement); MAP.md doc comments deferred (file-node labels carry module names); graph rebuild is full-from-cache (incremental edges deferred per spec §2).
