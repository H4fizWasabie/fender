# 07-CodeIntel

Type: task
Status: resolved
Blocked by: 06
Resolved: 2026-08-04

## Question

Write + execute Plan 7: code-intel — tree-sitter extraction → symbol index → call graph → query API → MAP.md generation. Incremental index mechanics (biggest unknown). graphify pipeline port.

## Answer

Plan 7 done: internal/codeintel — go-tree-sitter v0.24.0 + grammars (go/python/typescript/javascript, pinned go1.22-compatible), generic walker over langSpec tables (graphify extract pattern), incremental store (mtime+size stamps, dirty-only re-extract, JSON cache, graph.json persist), graph build with call resolution (INFERRED→EXTRACTED same-project, self-edge skip post-resolution), query API (Search/Symbols/Callers/Callees), MAP.md generation (package-dir sections, ticket-05 schema), Searcher adapter (D10 seam), `fender intel refresh|search|map` CLI. Fender self-indexed: 73 files, MAP.md written. 4 execution-caught bugs: Go symbol kind is "function_declaration" (not func_declaration), prune double-Join deleted stamps, self-edge check post-resolution, searcher double-Join. Cluster/embeddings/reports deferred (spec §2). Unblocks 08.
