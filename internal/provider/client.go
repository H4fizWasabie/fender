package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"` // deepseek-style
	Reasoning        string     `json:"reasoning,omitempty"`         // openrouter-style
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDef struct {
	Type     string          `json:"type"`
	Function ToolFunctionDef `json:"function"`
}

type ToolFunctionDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type Request struct {
	Model           string    `json:"model"`
	Messages        []Message `json:"messages"`
	Tools           []ToolDef `json:"tools,omitempty"`
	Stream          bool      `json:"stream,omitempty"`
	MaxTokens       int       `json:"max_tokens,omitempty"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
}

type Response struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Message Message `json:"message"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	CacheReadTokens  int `json:"prompt_cache_hit_tokens,omitempty"`        // deepseek/openrouter style
	Details          struct {
		CachedTokens int `json:"cached_tokens,omitempty"` // openai style
	} `json:"prompt_tokens_details,omitempty"`
}

// Cached returns the prompt tokens served from the provider cache,
// whichever wire format the provider used (D56).
func (u Usage) Cached() int {
	if u.CacheReadTokens > 0 {
		return u.CacheReadTokens
	}
	return u.Details.CachedTokens
}

// Client is one OpenAI-compatible endpoint. Not safe for concurrent use
// beyond what http.Client provides.
type Client struct {
	name      string
	base      string
	path      string // API path prefix ("/v1" default; OpenRouter "/api/v1")
	apiKey    string
	model     string
	models    []string
	mc        ModelConfig    // per-model config (thinking), zero when unset
	thinking  string         // reasoning_effort value to send; "" = off (omit)
	anthropic *anthropicWire // non-nil = Messages API transport (D42)
	http      *http.Client
}

func New(name string, p Provider) *Client {
	model := p.DefaultModel
	if model == "" && len(p.Models) > 0 {
		model = p.Models[0] // fall back to first model when no default_model set
	}
	path := p.Path
	if path == "" {
		path = "/v1"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return &Client{
		name:   name,
		base:   strings.TrimSuffix(p.BaseURL, "/"),
		path:   path,
		apiKey: p.APIKey,
		model:  model,
		models: p.Models,
		mc:     p.ModelConfigs[model],
		http:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *Client) Name() string     { return c.name }
func (c *Client) Model() string    { return c.model }
func (c *Client) BaseURL() string  { return c.base }
func (c *Client) Models() []string { return c.models }

// SetThinking sets the reasoning_effort level (D40). "" = off (field
// omitted — provider default thinking). Validates against the model's
// thinking_levels map: a level mapped to "" (null) is unsupported; models
// with thinking=false reject every level.
func (c *Client) SetThinking(level string) error {
	if level == "" {
		c.thinking = ""
		return nil
	}
	if !c.mc.Thinking {
		return fmt.Errorf("model %s does not support thinking (add model_configs.%s.thinking = true)", c.model, c.model)
	}
	value := level
	if v, ok := c.mc.ThinkingLevels[level]; ok {
		value = v
	}
	if value == "" {
		return fmt.Errorf("thinking level %q unsupported by %s", level, c.model)
	}
	c.thinking = value
	return nil
}

// Thinking returns the active reasoning_effort value ("" = off).
func (c *Client) Thinking() string { return c.thinking }

func (c *Client) Chat(ctx context.Context, req Request) (*Response, error) {
	if c.anthropic != nil {
		return c.anthropic.Chat(ctx, req)
	}
	if req.Model == "" {
		req.Model = c.model // the loop sends no model; the client knows its own
	}
	if c.thinking != "" {
		req.ReasoningEffort = c.thinking
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+c.path+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("%s: %s: %s", c.name, resp.Status, strings.TrimSpace(string(msg)))
	}
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("%s: decode: %w", c.name, err)
	}
	return &out, nil
}

// streamChunk mirrors the streaming chunk shape: choices[].delta.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string     `json:"content"`
			ReasoningContent string     `json:"reasoning_content"`
			Reasoning        string     `json:"reasoning"`
			ToolCalls        []ToolCall `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

func (c *Client) Stream(ctx context.Context, req Request, onDelta func(string), onThinking ...func(string)) (*Response, error) {
	if c.anthropic != nil {
		return c.anthropic.StreamChat(ctx, req, onDelta, onThinking...)
	}
	if req.Model == "" {
		req.Model = c.model
	}
	if c.thinking != "" {
		req.ReasoningEffort = c.thinking
	}
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+c.path+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("%s: %s: %s", c.name, resp.Status, strings.TrimSpace(string(msg)))
	}

	out := &Response{}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var tcBuf []ToolCall // accumulated tool calls across chunks, by index
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // ignore malformed keep-alive chunks
		}
		for _, ch := range chunk.Choices {
			if len(out.Choices) == 0 {
				out.Choices = append(out.Choices, Choice{})
			}
			if out.Choices[0].Message.Role == "" {
				out.Choices[0].Message.Role = "assistant" // streamed messages need a role (bug fix)
			}
			msg := &out.Choices[0].Message
			if ch.Delta.Content != "" {
				msg.Content += ch.Delta.Content
				onDelta(ch.Delta.Content)
			}
			if r := ch.Delta.ReasoningContent; r != "" {
				msg.ReasoningContent += r
				if len(onThinking) > 0 {
					onThinking[0](r)
				}
			} else if r := ch.Delta.Reasoning; r != "" { // openrouter alias
				msg.ReasoningContent += r
				if len(onThinking) > 0 {
					onThinking[0](r)
				}
			}
			for i, tc := range ch.Delta.ToolCalls {
				if i >= len(tcBuf) {
					tcBuf = append(tcBuf, make([]ToolCall, i+1-len(tcBuf))...)
				}
				cur := &tcBuf[i]
				if tc.ID != "" {
					cur.ID = tc.ID
				}
				if tc.Type != "" {
					cur.Type = tc.Type
				}
				if tc.Function.Name != "" {
					cur.Function.Name = tc.Function.Name
				}
				cur.Function.Arguments += tc.Function.Arguments // fragments append per index
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(tcBuf) > 0 {
		out.Choices[0].Message.ToolCalls = tcBuf
	}
	return out, nil
}

// StreamChat implements agent.Streamer: streams deltas, accumulates the
// full response (tool calls included).
func (c *Client) StreamChat(ctx context.Context, req Request, onDelta func(string), onThinking ...func(string)) (*Response, error) {
	return c.Stream(ctx, req, onDelta, onThinking...)
}
