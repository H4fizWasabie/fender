# Fender — Thinking Mode Design (Ticket 09)

**Date:** 2026-08-04
**Status:** Approved (discussion, D40). pi-style thinking levels, ponytail-sized.

## Scope

1. **Wire format** — parse `message.reasoning_content` (non-streaming) and `delta.reasoning_content` (streaming) into `provider.Message.ReasoningContent` / a stream callback.
2. **Control** — `reasoning_effort` request field, set per level. `off` = field omitted (provider default thinking). Verified: opencode zen gateway accepts `reasoning_effort: "low"` (HTTP 200).
3. **Config** — per-model `model_configs.<model>.{thinking, thinking_levels}` in fender.toml (backward compatible: `models` list stays string array).
4. **Levels** — REPL `/thinking off|low|medium|high`; clamp/validate against the model's map (null = unsupported → error; `thinking = false` → error); default `off`.
5. **Display** — observer `"thinking"` events; REPL renders dimmed when level != off, hidden at off.
6. **Budget rule** — never set max_tokens on reasoning models (leave 0 = provider ceiling). Documented in AGENTS.md-adjacent changelog + this spec.

## API

```go
// provider
type ModelConfig struct {
    Thinking       bool              `toml:"thinking"`
    ThinkingLevels map[string]string `toml:"thinking_levels"` // pi-level → provider value; omission = identity; "off" = omit field
}
// Provider gains: ModelConfigs map[string]ModelConfig `toml:"model_configs"`
// Request gains:  ReasoningEffort string `json:"reasoning_effort,omitempty"`
// Message gains:  ReasoningContent string `json:"reasoning_content,omitempty"`
func (c *Client) SetThinking(level string) error // validates against model map; "" = off
// Stream: onThinking func(string) callback for reasoning deltas

// agent
// Streamer: StreamChat(ctx, req, onDelta, onThinking func(string))
// Event gains Kind "thinking"

// repl
// /thinking off|low|medium|high — sets on the agent's LLM (Client), rebuild not needed
// thinking events rendered dimmed (\x1b[2m) when level != off
```

## Tests

| Test | Technique |
|------|-----------|
| Non-stream: reasoning_content parsed into Message | mock JSON |
| Stream: reasoning deltas via onThinking, content unaffected | mock SSE |
| SetThinking: valid level stored; unsupported (null map) errors; thinking=false model errors; off clears | validation |
| Request: level set → reasoning_effort in body; off → absent | mock server body assert |
| Agent: thinking deltas → observer "thinking" events | streamFake extension |
| REPL: /thinking high works; /thinking nope errors; dimmed rendering present when set | repl test |

## Acceptance

1. All prior tests unchanged (Streamer signature change: update streamFake only).
2. `go build/vet/test` green; changelog on every commit.
3. User config: zen model config with thinking=true + levels map.
4. Wayfinder ticket 09 resolved.
