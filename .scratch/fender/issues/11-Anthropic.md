# 11-Anthropic

Type: task
Status: open
Blocked by: 10

## Question

D6 Anthropic adapter: api = "anthropic" provider, same LLM interface, wire translation.

## Answer

Plan 11 done — D6 delivered. api = "anthropic" config flag; anthropic.go Messages API transport (system field, tool_use/tool_result blocks, input_json_delta streaming, thinking_delta → onThinking, max_tokens default 8192, x-api-key + anthropic-version headers); registry dispatch; *Client delegates to the wire when set. 3 mock-server tests (chat translation, tool round-trip, SSE). No live key needed — mock-verified.
