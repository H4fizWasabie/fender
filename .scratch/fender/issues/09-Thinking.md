# 09-Thinking

Type: task
Status: open
Blocked by: 08

## Question

Thinking mode (D40): reasoning_content parsing, reasoning_effort control, per-model config, /thinking REPL command, observer thinking events, never set max_tokens on reasoning models.

## Answer

Plan 9 done — thinking mode delivered. reasoning_content parsed (stream + non-stream), reasoning_effort per level (off/low/medium/high) via pi-style per-model thinking_levels map, Client.SetThinking validation (thinking=false models reject, null levels reject), observer "thinking" events (Streamer onThinking, variadic — prior callers compile), REPL /thinking command + dimmed rendering (hidden at off), reply-on-done rendering (complete_task answers now display). Rule: never set max_tokens on reasoning models. Live-verified against opencode zen gateway (deepseek-v4-flash-free): reasoning_effort accepted, reasoning streamed dimmed, answer displayed. Config: ~/.fender/fender.toml zen model_configs.thinking = true.
