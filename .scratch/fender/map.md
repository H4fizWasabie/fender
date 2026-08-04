# Fender — wayfinder map

## Destination

Fender v1: a working, from-scratch Go coding agent — single binary, OpenAI-compatible provider layer, guardrailed shell execution (sh-parser verdicts), one-loop agent with subagent-as-a-tool, mino-style context engineering, ICM memory layers, 23 bundled skills, code-intel, CLI + streaming UI. Spec: `docs/superpowers/specs/2026-08-04-fender-design.md`.

## Notes

- Constitution: `AGENTS.md` (D1–D37), `DECISIONS.md`, design spec — read before any build work.
- **Execution carried into the map**: tickets are build phases (write plan → test-first tasks → changelog'd commits), per the AGENTS.md build order. This overrides wayfinder's plan-only default.
- Every commit MUST stage `CHANGELOG.md` (hook-enforced, `.githooks/`).
- Allowed deps only: `BurntSushi/toml`, `mvdan.cc/sh/v3`, `go-tree-sitter`, `modernc.org/sqlite`.
- Reference repos (study material, not vendored): `~/Desktop/fender-references/` (code-context-engine, codegraph, graphify, skills, ponytail, mino).
- Discipline: ponytail (laziest working solution); `executing-plans` skill for plan execution.

## Decisions so far

<!-- the index — one line per closed ticket: gist + link -->

- [01-Foundation](issues/01-Foundation.md) — Plan 1 done: module, config types, chat client (non-streaming + SSE), registry, `fender providers` CLI; 3 plan bugs fixed (stream chunk panic, tool-call fragment merge, model fallback). Unblocks 02.
- [02-Guardrail](issues/02-Guardrail.md) — Plan 2 done: modes/verdicts/8 detectors/Judge/audit/timeout, sh/v3@v3.10.0 pinned; 4 plan bugs fixed. Unblocks 03.
- [03-ToolsAndLoop](issues/03-ToolsAndLoop.md) — Plan 3 done: tools (read_file/edit_file/shell with guardrail wiring/search with Searcher seam) + ONE agent loop (complete_task protocol, dedup, D36 orientation on thrash, delegate subagent-as-a-tool); provider client defaults model. Unblocks 04.
- [04-Context](issues/04-Context.md) — Plan 4 done: internal/context artifact engineering (8K rule, HEAD/TAIL, For() budget arithmetic ≤ ContextChars, artifact catalog, 24h sweep, Child() isolation); loop wired; shell cap 8 MiB; budget-bound test first-class (acceptance #3). Unblocks 05.
- [05-Memory](issues/05-Memory.md) — Plan 5 done: .fender/ workspace (Ensure idempotent), convention detection + direct load (user→project AGENTS.md/CLAUDE.md→CONTEXT.md), always-loaded System (8K cap, canonical-sources enforced), working prune (7d, patterns.md exempt); agent Mem nil-safe, delegates share. Memory graph/consolidation/facts deferred to D9 era (user-approved). Unblocks 06.
- [06-Skills](issues/06-Skills.md) — Plan 6 done: 23 skills vendored (MIT) + regex frontmatter parser + Bundled() + registry (Merge shadowing) + Match (word-overlap, model-invokable only) + load_skill tool + skill install; catalog model-invokable-only (cap 6000). Unblocks 07.
- [07-CodeIntel](issues/07-CodeIntel.md) — Plan 7 done: tree-sitter extraction (go/python/ts/js, langSpec tables), incremental store (stamps), graph build (call resolution), query API, MAP.md generation, intel CLI; Fender self-indexed 73 files; 4 execution-caught bugs fixed. Cluster/embeddings deferred. Unblocks 08.
- [10-Persistence](issues/10-Persistence.md) — Plan 10 done (D9): session save/resume/list, artifact-pointer compatible; auto-resume + --new. Unblocks 13 (consolidation).
- [09-Thinking](issues/09-Thinking.md) — Plan 9 done: thinking mode — reasoning_content parsing, reasoning_effort levels (pi-style map), /thinking REPL + dimmed display, reply-on-done rendering; live-verified on zen/deepseek. Post-v1 backlog unchanged.
- [08-CLIAndUI](issues/08-CLIAndUI.md) — Plan 8 done — **DESTINATION REACHED, Fender v1 complete**: observer/streamer loop, buildAgent composition root, REPL (/quit /model /mode /skills /help), fender run, fender init. Post-v1 backlog: D9 persistence, D6 Anthropic, D2 GUI, TUI, response caching, memory graph.

## Not yet specified


## Out of scope

- Session persistence, Anthropic adapter, GUI, TUI, response caching — spec Deferred list, seams only, never built in v1.
