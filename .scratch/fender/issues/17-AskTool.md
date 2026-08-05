# 17-AskTool

Type: task
Status: open
Blocked by: 16

## Question

D49: one-agent model — "subagent" = one API call to another key (ask tool). No nested agent for the common case.

## Answer

D49 delivered. ask tool: one-shot Chat call to a resolver-selected provider (its own key), no tools/memory/loop, reply as tool result. Tests: one-shot (exactly 1 call, no tools, single user message), default provider, empty prompt. Live: parent zen (acct 1) → 2 parallel asks zen-2 (acct 2) → OPINION-A OPINION-B. delegate retained for nested research subtasks.
