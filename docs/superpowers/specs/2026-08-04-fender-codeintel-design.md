# Fender — Code-Intelligence Design (Ticket 07)

**Date:** 2026-08-04
**Status:** Draft — implements D16/D19/D20/D34(3); supersedes spec §3.7.
**Reference sources:** `~/Desktop/fender-references/` — graphify (pipeline, extraction schema, cache.py), codegraph (grammars, query ergonomics), cce (retrieval).

The code-intel core built in Go from day one: tree-sitter extraction → symbol index → call graph → query API. MAP.md generation (D17: MAP.md is the front door, code-intel is the card catalog — ticket 05 reserved MAP.md's body for this ticket, schema unchanged).

---

## 1. Scope

Build `internal/codeintel`:

1. **Parser substrate** — `go-tree-sitter` (official bindings, cgo) + official grammar Go modules: Go, Python, TypeScript/JavaScript. cgo accepted (gcc required at build; single binary preserved). No parser rewriting (D19).
2. **Per-language extractors** — ONE generic tree-sitter walker + per-language kind tables (graphify's `extract_<lang>` pattern → a data table, not code per language). Output: graphify's node/edge schema with EXTRACTED/INFERRED confidence (D20).
3. **Incremental store** (D34 layer 3) — `.fender/codeintel/` with `stamps.json` (path → mtime+size), `extractions.json` (path → extraction), `graph.json` (built). Refresh re-extracts only changed files (graphify cache.py semantic-cache pattern).
4. **Graph build** — nodes/edges → in-memory graph (adjacency + symbol table); call edges resolved: same-module exact-name = EXTRACTED, name-only = INFERRED (graphify second pass).
5. **Query API** — `Search` (Searcher adapter for the D10 seam), `Symbols(path)`, `Callers(name)`, `Callees(name)`, `Refresh()`, `GenerateMap()`.
6. **MAP.md generation** (D17) — `## <module>` sections from the graph: package doc comment, key files, exported symbol list. Writes `.fender/memory/MAP.md` body, preserves ticket-05 schema.
7. **Agent wiring** — `fender intel refresh` CLI command + `intel` tool set (search/symbols/callers) registered when a codeintel store exists. Refresh on demand, NOT at Run start (sessions stay fast).

## 2. Non-goals (v1) — deferred with seams

| Item | Why | Seam |
|------|-----|------|
| **Cluster / community detection** (graphify stage) | Agent navigation wants symbol queries, not communities. Graphify's god-node analysis is report-oriented. | graph.json keeps `community` field optional; stage function signature reserved in build |
| **Report / export** (GRAPH_REPORT.md, Obsidian vault, HTML) | Fender is an agent, not a doc generator; MAP.md + queries serve the agent | `GenerateMap()` is the only export; others are graphify repo features |
| **Embeddings + semantic search** (D34 layer 5) | Symbol names are short — prefix/substring match suffices; embeddings add an API + cache for fuzzy recall the agent doesn't need in v1 | `Search` seam returns `[]SearchResult`; a semantic backend can slot behind it |
| **More languages** (Rust, C/C++, Java, Lua...) | v1 covers Fender's own codebase + the user's known projects (Go, Python, TS/JS) | langSpec table: one entry + fixture test per language |
| **LSP integration** | LSP is a live service, not an index (discussion 2026-08-04); the index answers symbol queries without a server | none — index is self-contained |
| **Incremental graph edges** | Full graph rebuild from cached extractions is cheap (extraction is the expensive part) | `build()` takes all extractions; swap for incremental later |

## 3. Decisions

| # | Decision |
|---|----------|
| 1 | **Languages v1: Go + Python + TypeScript/JavaScript.** One generic walker, per-language `langSpec` tables (extensions, symbol node kinds + name fields, call node kinds + target field, import kinds). New language = one table + fixture test. |
| 2 | **cgo is accepted** for tree-sitter (the C core is the point of tree-sitter). Build requires gcc. Single binary preserved. This is the ONE cgo exception; everything else stays pure Go (modernc.org/sqlite philosophy). |
| 3 | **Store = JSON files, not SQLite.** `.fender/codeintel/{stamps.json, extractions.json, graph.json}`. Codebases up to ~100K symbols are comfortably held in memory as JSON; SQLite buys nothing at this scale and adds a schema. |
| 4 | **Incremental = mtime+size stamps** (graphify cache.py pattern). Refresh: walk files → compare stamps → re-extract dirty → rebuild graph from all cached extractions. Extraction cache is authoritative; graph is derived (rebuildable). |
| 5 | **Pipeline cut: detect → extract → build → query + map.** Cluster/analyze/report/export deferred (non-goals). The D20 pipeline is honored in spirit — one stage per package file — but the agent-facing stages end at query. |
| 6 | **Schema = graphify's** (D20): nodes `{id, label, source_file, source_location}`, edges `{source, target, relation, confidence}` with `EXTRACTED | INFERRED`. ids are stable strings (`go:package:name`, `go:func:path:name` — format `lang:kind:path:name`). |
| 7 | **Call edges: EXTRACTED when the callee resolves in the same module; INFERRED when name-only** (cross-module/external). Imports: EXTRACTED. contains/file/module edges: EXTRACTED. |
| 8 | **Refresh is on-demand** — `fender intel refresh` command + `intel_refresh` tool. Never at Run start (a slow session boot is worse than a stale index; the agent calls refresh when it senses drift). |
| 9 | **MAP.md generation** — `## <top-level-dir>` sections; each lists: package doc comment (first doc-comment node in the module's main file), key files (by symbol count, top 5), exported symbols (top 20 by name). Replaces the ticket-05 placeholder body; schema untouched. |

## 4. Module API — `internal/codeintel`

```go
package codeintel

// ---- schema (graphify D20) ----
type Node struct {
    ID           string `json:"id"`
    Label        string `json:"label"`
    SourceFile   string `json:"source_file"`
    SourceLoc    string `json:"source_location"` // "L42" or "L12-C34"
}

type Edge struct {
    Source     string `json:"source"`
    Target     string `json:"target"`
    Relation   string `json:"relation"` // calls | imports | contains | module
    Confidence string `json:"confidence"` // EXTRACTED | INFERRED
}

// ---- store ----
type Store struct { ... }          // .fender/codeintel/ owner
func Open(root string) (*Store, error)   // creates dir if missing
func (s *Store) Stamps() map[string]Stamp // path → {MtimeNanos, Size}

// ---- extract ----
type Extraction struct {           // per-file result (cached)
    Nodes  []Node `json:"nodes"`
    Edges  []Edge `json:"edges"`
}
func ExtractFile(path string) (Extraction, error)  // language by extension; skip unknown

// ---- graph ----
type Graph struct { ... }
func Build(extractions map[string]Extraction) *Graph
func (g *Graph) Search(q string, limit int) []tools.SearchResult
func (g *Graph) Symbols(path string) []Node
func (g *Graph) Callers(name string) []Node
func (g *Graph) Callees(name string) []Node

// ---- refresh + map ----
func (s *Store) Refresh() (int, error)        // re-extract dirty, rebuild graph, persist; returns changed file count
func (s *Store) GenerateMap(g *Graph) string  // MAP.md body (schema from ticket 05)
```

Notes:
- `tools.SearchResult` is reused for Search (the D10 seam) — codeintel imports `internal/tools` for the type only (no cycle: tools doesn't import codeintel).
- `Searcher` adapter: `func (s *Store) Searcher() tools.Searcher` returns a closure over the current graph (refresh-aware: re-reads graph.json per call — staleness bounded by last refresh).
- Node id format: `lang:kind:path:name` (e.g. `go:func:internal/agent/agent.go:Run`); call edges reference callee by label when unresolved (INFERRED).
- Language dispatch by extension map: `.go` → go, `.py` → python, `.ts`/`.tsx` → typescript, `.js`/`.jsx` → javascript. Unknown extensions skipped (extraction returns empty, not error).

## 5. langSpec shape (one table per language)

```go
type langSpec struct {
    ext      []string
    symbol   map[string]string // node kind → symbol kind ("func"|"method"|"class"|"struct"|"interface"|"type"|"import")
    name     map[string]string // node kind → field name holding the identifier
    calls    []string          // call-expression node kinds
    callName string            // field name holding the called name
    imports  []string          // import node kinds
}
```

v1 tables (kinds verified against tree-sitter grammars at implementation time via fixture tests):

| Language | symbol kinds | calls | imports |
|----------|-------------|-------|---------|
| Go | func_declaration, method_declaration, type_declaration | call_expression (function field) | import_declaration |
| Python | function_definition, class_definition | call (function field) | import_statement, import_from_statement |
| TS/JS | function_declaration, method_definition, class_declaration, interface_declaration | call_expression (function field) | import_statement |

## 6. Tests

| Test | Technique |
|------|-----------|
| Extract Go file → expected nodes (func/method/type/import) + calls edge EXTRACTED | fixture: a small Go file with intra-module calls |
| Extract Python + TS fixtures → node kinds | fixture per language |
| Unknown extension → empty extraction, no error | resilience |
| Refresh: touch one file → only it re-extracts (stamp compare), changed count = 1 | incremental |
| Refresh idempotent: no changes → 0 re-extracted | stamps |
| Graph build: calls resolve same-module → EXTRACTED; external name-only → INFERRED | confidence |
| Search: prefix/substring on symbol names → SearchResults with file/line | query |
| Symbols(path), Callers, Callees round-trip | query |
| GenerateMap: module sections, doc comment, key files | MAP schema (ticket 05) |
| Searcher adapter wired into tools.New → search tool returns symbol hits | integration |
| Store round-trip: persist + reopen → same graph | persistence |

## 7. Acceptance criteria

1. `go build ./...` (with cgo/gcc) green, `go vet ./...` clean, `go test ./...` green.
2. All existing tests unchanged (codeintel is additive; tools.Searcher is a function type — no signature change).
3. `fender intel refresh` on Fender's own repo indexes it; `fender intel search "RunLoop"` returns the agent loop symbols; MAP.md generated into `.fender/memory/` (ticket-05 schema).
4. Incremental: second refresh re-extracts 0 files.
5. `CHANGELOG.md` updated on every commit (hook-enforced).
6. Wayfinder ticket 07 resolved; frontier → 08 (CLI + UI).

## 8. Deferred (with seams)

| Item | Decision | Seam |
|------|----------|------|
| Cluster/communities, god nodes | graphify stage, report-oriented | `community` field optional on nodes; stage fn reserved |
| Report/export (HTML/Obsidian) | doc-generator features | `GenerateMap` is the only exporter |
| Embeddings/semantic search | D34 layer 5, not v1 | Searcher func type |
| More languages | add when needed | langSpec + fixture per language |
| LSP source | live service, not index | none |
| Incremental edges | rebuild-from-cache is cheap | build() input is the extraction cache |
| Auto-refresh at Run start | sessions stay fast | `intel_refresh` tool is the trigger |
