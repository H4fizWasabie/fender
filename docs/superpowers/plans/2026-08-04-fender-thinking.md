# Fender Plan 9: Thinking Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`).

**Goal:** D40 — reasoning_content parsing, reasoning_effort control via pi-style levels, /thinking REPL command, observer thinking events.

**Architecture:** provider parses + sends; agent routes; repl renders. Level state lives on `provider.Client` (`SetThinking`), applied in Chat/Stream before marshal. Streamer interface gains an `onThinking` callback (single breaking change — update `streamFake`).

**Tech Stack:** Go 1.22, stdlib. No new deps.

## Global Constraints

- Read AGENTS.md, DECISIONS.md (D40), ticket-09 spec.
- Every commit stages CHANGELOG.md (hook).
- **Never set max_tokens on reasoning models** (leave 0 — provider ceiling).
- Prior tests unchanged except `streamFake` (Streamer signature).

---

### Task 1: provider — wire format + level control

**Files:** `internal/provider/client.go`, `internal/provider/config.go`, `internal/provider/client_test.go`, `internal/provider/config_test.go`

**Interfaces:**
- `Message.ReasoningContent string` (json `reasoning_content,omitempty`)
- `Request.ReasoningEffort string` (json `reasoning_effort,omitempty`)
- `type ModelConfig struct { Thinking bool; ThinkingLevels map[string]string }` (toml `thinking`, `thinking_levels`)
- `Provider.ModelConfigs map[string]ModelConfig` (toml `model_configs`)
- `func (c *Client) SetThinking(level string) error` — "" = off; validates: thinking=false model → error; level in map with empty-string value (null) → error "unsupported"; map value used for the wire field; default mapping low→low, medium→medium, high→high
- `Client.Stream(ctx, req, onDelta, onThinking func(string))` — parse `delta.reasoning_content`
- `Client.Chat` — parse `message.reasoning_content`; apply stored thinking level: `if c.thinking != "" { req.ReasoningEffort = c.thinking }`

- [ ] **Step 1: Failing tests** — mock server with reasoning_content (chat + SSE), body assert for reasoning_effort on/off, SetThinking validation cases, config decode of model_configs.

- [ ] **Step 2: Implement** — fields, config, SetThinking, parse in Chat/Stream, apply level in both.

- [ ] **Step 3: Verify** — `go test ./internal/provider/ -v` green.

- [ ] **Step 4: Commit** — "feat: thinking wire format (reasoning_content/effort) + per-model config + SetThinking"

### Task 2: agent — thinking events

**Files:** `internal/agent/agent.go`, `internal/agent/observer_test.go`

**Interfaces:**
- `Streamer.StreamChat(ctx, req, onDelta, onThinking func(string)) (*provider.Response, error)`
- `agent.chat`: Streamer path passes `onThinking` → `Observer(Event{Kind: "thinking", Text: d})`; non-stream path: `resp.Choices[0].Message.ReasoningContent` → one thinking event
- Non-streamer LLMs: no thinking events (nothing parsed)

- [ ] **Step 1: Failing test** — extend streamFake with onThinking param; assert thinking events emitted.

- [ ] **Step 2: Implement** — signature + routing.

- [ ] **Step 3: Verify** — `go test ./internal/agent/ -v` + full suite.

- [ ] **Step 4: Commit** — "feat: observer thinking events (Streamer onThinking)"

### Task 3: repl — /thinking + dimmed rendering

**Files:** `cmd/fender/repl.go`, `cmd/fender/repl_test.go`

**Interfaces:**
- `/thinking off|low|medium|high` — `st.agent.LLM.(*provider.Client).SetThinking(level)` (error if LLM isn't a *Client — e.g. tests); track `st.thinking`; print `thinking -> <level>`
- Observer closure: on "thinking" event, render dimmed (`\x1b[2m%s\x1b[0m`) only when `st.thinking != ""`; drop otherwise
- `/help` lists /thinking

- [ ] **Step 1: Failing test** — /thinking nope → error; /help mentions /thinking; dimmed rendering present when level set (feed a task with a fake… simplest: test the observer closure directly via a small exported render helper `renderEvent(out, e, showThinking bool)` used by the observer — test that helper).

- [ ] **Step 2: Implement** — renderEvent helper + slash case + observer wiring.

- [ ] **Step 3: Verify** — `go test ./cmd/fender/ -v` + full suite.

- [ ] **Step 4: Commit** — "feat: /thinking command + dimmed reasoning rendering"

### Task 4: user config + wayfinder resolve

- [ ] **Step 1:** Update `~/.fender/fender.toml` zen provider: add `[providers.zen.model_configs.deepseek-v4-flash-free]` with `thinking = true` + levels map. Smoke: `fender run "say hi"` (verify reasoning works end-to-end via REPL event path; check audit log unaffected).

- [ ] **Step 2:** Resolve ticket 09 + map entry (delivered: wire format, levels, /thinking, dimmed display; rule: no max_tokens on reasoning models).

- [ ] **Step 3:** Commit — "docs: resolve wayfinder ticket 09 (thinking done)"

---

## Self-Review

- Spec coverage: all 6 scope items → Tasks 1–3; tests table → per-task; acceptance → Task 4.
- Placeholders: none. One flagged adaptation: REPL's LLM may be a test fake — SetThinking must type-assert to *provider.Client and error otherwise.
- Type consistency: Streamer signature change ripples to streamFake (one file) + provider.Client.StreamChat.
- CHANGELOG: every task commits with entry.
- Deps: none.
