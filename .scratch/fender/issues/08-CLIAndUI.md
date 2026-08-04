# 08-CLIAndUI

Type: task
Status: resolved
Blocked by: 07
Resolved: 2026-08-04

## Question

Write + execute Plan 8: CLI commands (`fender run`, `fender init`, `fender skill install`) + plain streaming renderer (`internal/ui/`). The loop must work with one provider, two tools, and a REPL before this phase — this ticket finishes the surface.

## Answer

Plan 8 done — Fender v1 COMPLETE. Observer events (delta/tool/done, nil-safe) + optional Streamer interface (provider.Client.StreamChat); buildAgent composition root (providers, guardrail mode + audit file, codeintel searcher w/ fallback, skills merge, memory, context, resolver); REPL with /quit /model /mode /skills /help + live observer rendering; `fender run` (autonomous, exit code by status); `fender init` (workspace + fender.toml scaffold, idempotent); no-args launches REPL. 2 execution fixes: /model error path (config-first), obsolete no-args-usage test removed. Post-v1 backlog: D9 session persistence, D6 Anthropic adapter, D2 GUI, TUI polish, response caching (D35), memory graph/consolidation. Destination reached.
