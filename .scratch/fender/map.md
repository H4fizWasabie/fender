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

## Not yet specified

- Code-intel incremental index mechanics — biggest unknown; sharpens when Plan 7 starts (graphify pipeline port).

## Out of scope

- Session persistence, Anthropic adapter, GUI, TUI, response caching — spec Deferred list, seams only, never built in v1.
