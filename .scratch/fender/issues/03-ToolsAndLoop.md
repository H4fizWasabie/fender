# 03-ToolsAndLoop

Type: task
Status: resolved
Blocked by: 02

## Question

Write + execute Plan 3: tools (read, edit, shell, search) + the ONE agent loop — flat loop by default, orientation turn only on thrash (D36), subagent-as-a-tool (same Agent type in a goroutine). Wire types consume the provider layer from ticket 01.

## Answer

Plan 3 done (`docs/superpowers/plans/2026-08-04-fender-tools-loop.md`, 8 tasks):

- Tools (`internal/tools`): read_file (1-based offset/limit slices, project containment), edit_file (unique exact-match replace), shell (Judge verdicts — REFUSE hard in all modes, ASK via injectable approver, 60s timeout, audit every command, 64 KiB cap), search (walk-based default behind the Searcher seam for graphify/cce/codegraph).
- Agent loop (`internal/agent`): ONE flat loop (mino skeleton, D37) — complete_task completion protocol, in-run tool dedup (D32), no-progress stall, max-iter cap; adaptive OODA (D36): one orientation turn on tool errors / repeated calls / text-only thrash; delegate tool (D13): same Agent in a goroutine, per-subagent provider via Resolver (D7).
- Provider: Chat/Stream default the model when omitted.

Unblocks 04 (context/artifact engineering).
