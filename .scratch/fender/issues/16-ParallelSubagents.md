# 16-ParallelSubagents

Type: task
Status: open
Blocked by: 15

## Question

D8 parallel subagent dispatch + main-vs-subagent identity + subagent provider config.

## Answer

D48 delivered + **live-verified cross-key**: parent on zen (account 1) → two PARALLEL delegate subagents on zen-2 (account 2's key, via config `subagent = "zen-2"`) → both returned (SUB-A, SUB-B) → parent completed. Parallel tool dispatch (goroutines + WaitGroup, ordered results, dedup pre-checked), context.Manager thread-safe (2 deadlock bugs fixed: record re-lock, Child copylocks — race-clean), identity (Event.Source, Agent.Name, magenta [subagent:<provider>] tags in REPL + dashboard, subagent done suppressed), config `subagent =` default provider. Note: free deepseek occasionally emits malformed multi-tool responses (transient delegate errors — harness recovers via D36); runs take 60-90s (free tier latency).
