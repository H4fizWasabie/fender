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
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
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
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	Tools     []ToolDef `json:"tools,omitempty"`
	Stream    bool      `json:"stream,omitempty"`
	MaxTokens int       `json:"max_tokens,omitempty"`
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
}

// Client is one OpenAI-compatible endpoint. Not safe for concurrent use
// beyond what http.Client provides.
type Client struct {
	name   string
	base   string
	apiKey string
	model  string
	models []string
	http   *http.Client
}

func New(name string, p Provider) *Client {
	model := p.DefaultModel
	if model == "" && len(p.Models) > 0 {
		model = p.Models[0] // fall back to first model when no default_model set
	}
	return &Client{
		name:   name,
		base:   strings.TrimSuffix(p.BaseURL, "/"),
		apiKey: p.APIKey,
		model:  model,
		models: p.Models,
		http:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *Client) Name() string  { return c.name }
func (c *Client) Model() string { return c.model }

func (c *Client) Chat(ctx context.Context, req Request) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/chat/completions", bytes.NewReader(body))
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
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

func (c *Client) Stream(ctx context.Context, req Request, onDelta func(string)) (*Response, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/chat/completions", bytes.NewReader(body))
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
			msg := &out.Choices[0].Message
			if ch.Delta.Content != "" {
				msg.Content += ch.Delta.Content
				onDelta(ch.Delta.Content)
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
