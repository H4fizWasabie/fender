# 16-ParallelSubagents

Type: task
Status: open
Blocked by: 15

## Question

D8 parallel subagent dispatch + main-vs-subagent identity + subagent provider config.

## Answer

D48 was delivered and live-verified as recorded below, then **superseded by D50**. Fender now executes tools sequentially and uses one synchronous ephemeral child on the main agent's provider fallback chain. This ticket remains historical evidence, not current architecture.

Historical result: parent on zen (account 1) → two parallel delegate subagents on zen-2 (account 2's key, via config `subagent = "zen-2"`) → both returned (SUB-A, SUB-B) → parent completed. Parallel tool dispatch used goroutines + WaitGroup with ordered results and pre-checked dedup; context.Manager was made thread-safe and remains so.
