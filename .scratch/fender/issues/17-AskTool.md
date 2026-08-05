# 17-AskTool

Type: task
Status: open
Blocked by: 16

## Question

D49: one-agent model — "subagent" = one API call to another key (ask tool). No nested agent for the common case.

## Answer

D49 was delivered and live-verified, then **superseded by D50**. The `ask` tool is removed. A second key is now configured only as provider fallback, while `delegate` remains the single child-agent mechanism. This ticket remains historical evidence, not current architecture.

Historical result: one-shot Chat call to a resolver-selected provider with no tools/memory/loop; parent zen (account 1) → two parallel asks to zen-2 (account 2) → OPINION-A and OPINION-B.
