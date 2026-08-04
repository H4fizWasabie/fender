# 05-Memory

Type: task
Status: resolved
Blocked by: 04
Resolved: 2026-08-04

## Question

Write + execute Plan 5: memory = ICM layers (D14, D17) — PROJECT.md always, MAP.md from code-intel, reference/ + working/ selective. Convention files (AGENTS.md/CLAUDE.md/CONTEXT.md) load DIRECTLY, never copied into PROJECT.md. Memory graph.

## Answer

Plan 5 done: internal/memory — Ensure scaffold (idempotent, seeded templates), Detect (user ~/.fender/AGENTS.md → project AGENTS.md/CLAUDE.md → CONTEXT.md, README/.cursorrules never), Bootstrap + System (provenance markers, 8K cap oldest-first, canonical-sources test-enforced), working prune (7d, patterns.md exempt) + catalog. Agent Mem wired nil-safe; delegates share project memory. 14 new tests (96 total). Memory graph + consolidation + SQLite facts deferred to D9 era (spec approved 2026-08-04). Unblocks 06.
