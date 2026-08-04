# Fender — CLI + UI Design (Ticket 08)

**Date:** 2026-08-04
**Status:** Draft — implements D2/D4/D26/D25/D27; supersedes spec §3.9.
**This is the final ticket: it finishes the user-facing surface and completes Fender v1.**

The plain streaming renderer (D26): `fender` interactive REPL, `fender run "task"` autonomous mode (D4), `fender init` (D25), streaming tool-call visibility, slash commands. The composition root that wires every prior subsystem together.

---

## 1. Scope

1. **Observer + streaming loop support** (`internal/agent`): `Agent.Observer func(Event)` (nil-safe) firing `delta` (model text), `tool` (tool name + status), `done` (result). Optional `Streamer` interface (`StreamChat(ctx, req, onDelta)`) type-asserted from the LLM — `provider.Client` implements it (ticket-01 `Stream`); the loop streams deltas through the observer when both exist, else falls back to `Chat` with one whole-content delta. Zero behavior change for existing tests (nil-safe observer, type assertion).
2. **Composition root** (`cmd/fender/agent.go`): `buildAgent(cfgPath string) (*agent.Agent, error)` wiring everything from `fender.toml`:
   - provider registry → default client as LLM; delegate Resolver = registry (D7: subagents pick providers)
   - guardrail mode from config (`strict|balanced|yolo`, default balanced) → `tools.ShellConfig{Mode, ProjectDir, Timeout, Audit, Approver}`
   - audit log → `~/.fender/audit.log` (append)
   - codeintel store → `Searcher` (D10 seam) — nil-safe: index missing → default searcher
   - skills: `Bundled()` merged with user `~/.fender/skills` + project `.fender/skills` (D27 lookup order)
   - `memory.New(".")`, `ctxpkg.New()` (artifacts), default system prompt
3. **REPL** (`fender` with no args, D26): prompt loop — read line → slash-command dispatch or Agent.Run with accumulated history + observer → render deltas live (minimal ANSI), tool calls as `[tool name: status]` lines, result printed with status color. Slash commands: `/quit`, `/model <provider>`, `/mode <strict|balanced|yolo>`, `/skills`, `/help`. Ctrl-C cancels the current run.
4. **`fender run <task>`** (D4): one-shot autonomous — same assembly, one Run, prints final reply, exit code 0 on complete, 1 on error/blocked/stalled.
5. **`fender init`** (D25): `memory.Ensure()` + scaffold `fender.toml` (commented template: providers section + mode) if missing; idempotent.
6. **`fender skill install`** already exists (ticket 06) — untouched.

## 2. Non-goals (v1)

- Session persistence (D9): the REPL's history lives in memory for the session only.
- Multiline input editing (single-line input; paste works; multiline is a TUI-era nicety).
- Markdown rendering in the terminal (plain text per D26; the GUI era owns rendering).
- `/undo`, conversation branching, streaming token counters — polish, not v1.
- GUI (D2) — explicitly out; the renderer interface (observer events) is the GUI seam.

## 3. Decisions

| # | Decision |
|---|----------|
| 1 | **Observer events are the renderer seam.** The loop emits events; the UI renders them; a future GUI subscribes to the same events (D2 seam). Event struct: `{Kind: "delta"\|"tool"\|"done", Text string, Status string}`. |
| 2 | **Streaming is a type assertion, not an interface change.** `type Streamer interface { StreamChat(ctx, req, onDelta func(string)) (*provider.Response, error) }` — the loop checks `if st, ok := a.LLM.(Streamer); ok && a.Observer != nil`. Existing fakes/tests unaffected. |
| 3 | **REPL keeps history in memory** across turns (session semantics without persistence): messages accumulate; `Ctx.For` bounds them per turn (MaxHistoryTurns=5, D38). Agent.Run receives the full history each turn. |
| 4 | **Slash commands are parsed by the REPL, not the model** — `/quit`, `/model <provider>` (swaps the agent's LLM), `/mode <mode>` (rebuilds guardrail mode — live config switch), `/skills` (lists installed + bundled), `/help`. Unknown `/x` → error line, no agent run. |
| 5 | **`fender run` prints only the final reply** (+ status line to stderr) — autonomous mode is quiet; the REPL is chatty. |
| 6 | **Exit codes:** `fender run` → 0 on complete, 1 on blocked/stalled/error/cancelled. REPL → 0 on /quit or EOF. |
| 7 | **Audit lives at `~/.fender/audit.log`** (append-only, timestamps) — the guardrail audit (D24) gains a file sink in the composition root. |
| 8 | **Default system prompt** (cmd/fender const): "You are Fender, a coding agent. Work autonomously within your tools. When the task is done, call complete_task with the final reply." — the disciplines (ponytail, skills, memory) compose via Agent wiring, not the prompt. |

## 4. Module API — `internal/agent` additions

```go
// Event is one observable loop event (the renderer seam, spec §3.1).
type Event struct {
    Kind   string // "delta" | "tool" | "done"
    Text   string // delta text / tool description
    Status string // tool status ("ok"|"error"|"cached") or result status
}

// Observer receives loop events; nil-safe.
Observer func(Event)

// Streamer is the optional streaming capability of an LLM (spec §3.2).
type Streamer interface {
    StreamChat(ctx context.Context, req provider.Request, onDelta func(string)) (*provider.Response, error)
}
```

`provider.Client` gains:

```go
// StreamChat implements agent.Streamer — streams deltas, accumulates the
// full response (tool calls included).
func (c *Client) StreamChat(ctx context.Context, req provider.Request, onDelta func(string)) (*provider.Response, error)
```

Loop changes in `Run` (nil-safe, single call site — the LLM call):
- `if a.Observer != nil`: after Chat → `a.Observer(Event{Kind: "delta", Text: msg.Content})` (one delta for non-streaming); on Streamer path → per-chunk deltas; tool execution → `a.Observer(Event{Kind: "tool", Text: name, Status: status})`; return → `a.Observer(Event{Kind: "done", Status: result.Status})`.

## 5. Composition root — `cmd/fender/agent.go`

```go
// buildAgent wires every subsystem from fender.toml (spec §1.2).
func buildAgent(cfgPath string) (*agent.Agent, *provider.Registry, error)
```

Steps:
1. `provider.LoadDefault()` or `Load(cfgPath)`; mode = `guardrail.Mode(cfg.Mode)` (empty → balanced)
2. LLM = default provider client (`r.Default()`); Resolver = a `provider.Resolver` adapter (check `agent.Resolver`'s existing shape — ticket 03 delegate used it)
3. audit := `guardrail.NewAuditFile("~/.fender/audit.log")` (check the audit API from ticket 02; wrap if needed)
4. Approver: nil → ASK denied (per ShellConfig comment); the REPL sets a prompt-based approver for interactive mode; `fender run` passes nil (strict-mode ASKs get denied — documented behavior, D12 partial-results)
5. tools := `tools.New(".", ShellConfig{...}, searcher)`; searcher = codeintel store's `Searcher()` when the index has been refreshed, else `tools.DefaultSearcher(".")` — decide by stat of `.fender/codeintel/graph.json`
6. skills: `skills.Bundled()` → `.Merge(project, user)` via `skills.Load`
7. `agent.NewAgent(llm, tools)` + System + Mem + Skills + Ctx

## 6. REPL — `cmd/fender/repl.go`

```go
func repl(out, errOut io.Writer, in *bufio.Reader, cfgPath string) error
```

- Banner: `fender 0.1.0 — type /help for commands`
- Loop: print `> `, read line; blank → continue; `/quit`/EOF → exit 0; other `/x` → dispatch; else Agent.Run with observer rendering:
  - delta → print text (no newline buffering — flush per event)
  - tool → `\n  [tool name: status]\n` (cached → `[cached]`)
  - done → print `\n<status>: reply` (complete → green `✓`, blocked → yellow, error → red; minimal ANSI)
- Agent rebuilt per `/mode`/`/model` (buildAgent returns a fresh agent; history is carried in the REPL's messages slice)
- Ctrl-C: context cancel → run returns cancelled → loop continues

## 7. Tests

| Test | Technique |
|------|-----------|
| Observer: fake LLM (non-streamer) → one delta event with full content, tool event per tool call, done event | loop events |
| Observer nil → existing behavior unchanged (ticket-03/04 tests pass untouched) | nil-safety |
| Streamer path: fake Streamer → per-chunk deltas, response accumulated | streaming |
| Client.StreamChat wraps Stream (mock SSE server) | provider |
| REPL: feed `"hi\n/quit\n"` → fake-agent run + clean exit | repl loop |
| REPL slash: `/mode yolo` rebuilds agent; `/model nope` → error line; `/help` prints commands | dispatch |
| `fender run` completes → exit 0, reply on stdout; blocked → exit 1 | CLI |
| `fender init` idempotent, scaffolds config + memory workspace | CLI |

## 8. Acceptance criteria

1. `go build ./... && go vet ./... && go test ./...` green; all prior tickets' tests unchanged (observer nil-safe).
2. `fender` REPL works end-to-end against a real provider (or a mock SSE server in tests).
3. `fender run "list configured providers"` works; `fender init` scaffolds; `fender skill install` still works.
4. Streaming deltas render live (mock SSE test proves the event path).
5. `CHANGELOG.md` updated on every commit (hook-enforced).
6. Wayfinder ticket 08 resolved — **Fender v1 complete**; map notes v1 done + post-v1 backlog (D9/D6/D2 deferred items).

## 9. Deferred (with seams)

| Item | Decision | Seam |
|------|----------|------|
| Session persistence | D9 | REPL history is in-memory; `Ctx.For` bounds; `.fender/memory/sessions/` reserved |
| GUI | D2 | observer events = renderer seam |
| Multiline input | polish | bufio reader, TUI era |
| Markdown rendering | D26 plain text | renderer seam |
| Token counters / streaming stats | polish | observer could carry usage |
| Anthropic adapter | D6 | provider interface |
