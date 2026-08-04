# 04-Context

Type: task
Status: resolved
Blocked by: 03

## Question

Write + execute Plan 4: context artifact engineering (mino port, D31) — 8K inline limit, HEAD/TAIL, read_file never compacted, write-elsewhere, isolate. Port `context_test.go` techniques as Fender's context test suite (budget bounds, head/tail, compaction markers, artifact catalog).

## Answer

Plan 4 done (`docs/superpowers/plans/2026-08-04-fender-context.md`, 7 tasks):

- `internal/context`: Manager (root /tmp/fender/artifacts/<runID>, ContextChars 100K, MaxHistoryTurns 5) — CompactOutput (8K rule; read_file never compacted; 0700/0600 artifact write; per-call paths; write-failure marker), CompactInput (HEAD/TAIL), For() budget arithmetic (system + Σmsgs ≤ ContextChars, turns truncation + marker, artifact catalog ≤2K rides in context), Cleanup(24h) at Run start, Child() isolation.
- Agent loop wired: ingress For(), tool results compacted before history, dedup caches pointers, delegate children get Child().
- Tool caps: shell 64 KiB → 8 MiB (memory ceiling — artifacts carry full output); read 1 MiB stays (never compacted).
- 21 new tests including the load-bearing budget-bound test (acceptance #3); all ticket-03 tests pass unchanged (nil-safe Ctx).

Unblocks 05 (memory/ICM layers).
