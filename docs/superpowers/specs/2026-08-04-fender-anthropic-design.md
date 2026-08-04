# Fender — Anthropic Adapter Design (Ticket 11, D6 delivered)

## Scope

1. **Config** — `Provider.API string` (toml `api`, default `"openai"`); `"anthropic"` selects the adapter. Base URL default `https://api.anthropic.com`, path `/v1/messages`.
2. **Wire translation** (`internal/provider/anthropic.go`):
   - Request: system messages → top-level `system` (concatenated); other messages → `content: [{type:"text", text}]`; tool results → `content: [{type:"tool_result", tool_use_id, content}]`; tool calls → `content: [{type:"tool_use", id, name, input}]`; `tools: [{name, description, input_schema}]`; `max_tokens` required by Anthropic — default 8192 when caller sets 0 (never on reasoning... Anthropic has no reasoning_effort; the budget rule is OpenAI-side).
   - Response: `content` blocks → Message{Content, ToolCalls}; `stop_reason` → status hints; usage mapped.
   - Streaming (SSE): `content_block_delta` (text_delta / input_json_delta) → onDelta/onThinking? Anthropic thinking blocks (`thinking_delta`) → map to onThinking when present (bonus, cheap).
3. **Client shape** — `type anthropicClient struct{...}` implementing `agent.LLM` (Chat + StreamChat) and `SetThinking`-compatible (no-op error: thinking config is OpenAI-side; documented). Registry returns whichever client per provider API.
4. **Tests** — mock server: request body shape (system field, tool blocks), response parse (text + tool_use), SSE parse, header assertions.

## Acceptance

1. Prior tests unchanged; registry `Client(name)` returns the right client per api.
2. `api = "anthropic"` provider in fender.toml works end-to-end against a mock (no live key needed).
3. CHANGELOG per commit; ticket resolved.
