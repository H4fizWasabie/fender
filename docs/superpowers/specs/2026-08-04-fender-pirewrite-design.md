# Fender — Pi-Rewrite Design (Ticket 21, D61)

**Status:** Approved in discussion (2026-08-05). Implementation pending.
**Identity shift:** Fender is a pi-style coding agent in Go — lightweight loop, no protocol tax. Not a mino derivative.

## 1. The loop change (the core)

**Universal end condition: a response with no tool calls = done.**

```
response has tool calls  → execute (sequential), continue
response is text only     → that text is the reply. Turn ends. No nag, no protocol.
empty response            → error
maxIter                   → stalled
```

Deleted: the nag message, `completionError` paths, `complete_task` schema + parsing, the protocol state in the loop.

## 2. complete_task removed entirely

- Not in the tool schemas; the loop has no completion branch.
- Delegate children: `child.Run` returns the child's final text as the tool result — no blocked/complete ceremony. `subagentSystem` prompt updated accordingly (drop "call complete_task" instructions).
- Internal `Result.Status` (stalled/error/cancelled) remains for the harness; "blocked" status disappears from the model-facing contract. The REPL/dashboard status rendering adjusts (no blocked color path needed — or keep it for legacy results).
- D53's conversational acceptance becomes the default (no nags to escape from).

## 3. No mino verification

No file-claim os.Stat pass. The user is the completion verifier (chat); autonomous runs are bounded by maxIter + D52 thrash protection.

## 4. System prompt rewrite (pi-style, ~40 lines)

Replace `defaultSystem` with engineered guidance:
- Identity: coding agent, autonomous within tools
- Tool use: prefer tools over guessing; read before editing; verify with tests
- Output style: concise, milestones not narration (keep D-conciseness)
- Ambiguity: ask in prose (the turn ends; the user answers — no protocol needed)
- Completion: when the work is done, stop — the final text is the answer
- No protocol instructions (no complete_task mentions)

## 5. Tool-description enrichment

Per tool (read_file, edit_file, shell, search, delegate, ask?, intel_refresh, load_skill): when/why guidance in the description. Focus: shell (command hygiene), read_file (offset/limit slices), edit_file (unique old_text), delegate (self-contained subtasks, final text = answer).

## 6. Kept (unchanged)

Guardrail (modes + verdicts), ICM memory, skills + ponytail, steering (D58), meter (D60), /compact, sessions (D41), tool dedup + D52 normalization, maxIter config (D54).

## 7. Dropped

Consolidation (D43 — session-end distillation; delete consolidate.go + REPL/dashboard wiring + tests). Artifact machinery (already gone, D56).

## 8. Tests

- Loop: text-only response → complete with that text (1 iteration, no nag, no extra LLM calls — assert call count)
- Loop: tool call then text → text is the reply
- Loop: tool errors → stall path unchanged (D52)
- Delegate: child text = tool result (no complete_task in child schemas — assert)
- complete_task absent from schemas
- System prompt contains guidance (no "complete_task" string)
- REPL/dashboard: no blocked-status dependency breaks (statuses rendering)
- Consolidation removal: no consolidate references compile

## 9. Acceptance

1. `go build/vet/test ./...` green, race clean
2. No "complete_task" or "nag" strings anywhere in the codebase
3. A pure-chat exchange: 1 user message → 1 model call → turn ends (test-enforced call count)
4. The go-test loop protection still stalls (D52 regression test green)
5. CHANGELOG per commit; ticket resolved
