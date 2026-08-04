# 13-Consolidation

Type: task
Status: open
Blocked by: 10

## Question

D32 layer 6: background small-model distillation of session logs → facts (markdown files) + episodes. Unblocked by D9 persistence.

## Answer

Plan 13 done — D32 layer 6 delivered. consolidate.go: session-end distillation (≥4 exchanges, unconsolidated) → facts as .md files (frontmatter date/subject, slug names, dedup by subject) + episode appended to sessions/episodes.md; placeholder filtering, JSON-only parse, session marked consolidated + persisted. REPL fires it in a goroutine at quit/EOF (silent failures → re-distilled next time). 3 tests: write path, small-session skip, dedup.
