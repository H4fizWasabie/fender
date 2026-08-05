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

// anthropicClient implements agent.LLM (Chat + StreamChat) against the
// Anthropic Messages API (D42): OpenAI-shaped Request/Response in, Messages
// wire format out.
type anthropicClient struct {
	name   string
	base   string
	apiKey string
	model  string
	http   *http.Client
}

func NewAnthropic(name string, p Provider) *Client {
	base := p.BaseURL
	if base == "" {
		base = "https://api.anthropic.com"
	}
	model := p.DefaultModel
	if model == "" && len(p.Models) > 0 {
		model = p.Models[0]
	}
	// Anthropic-shaped client is NOT a *Client (different wire format), but
	// the registry stores *Client. Wrap: an anthropicClient that embeds the
	// shared surface via a separate type would break the registry type.
	// Solution: registry stores the LLM-er via a small interface — see
	// registry.go change below. For now, construct the anthropic client
	// through the *Client adapter:
	c := &Client{
		name:   name,
		base:   strings.TrimSuffix(base, "/"),
		apiKey: p.APIKey,
		model:  model,
		models: p.Models,
		http:   &http.Client{Timeout: 10 * time.Minute},
	}
	c.anthropic = &anthropicWire{name: name, base: c.base, apiKey: p.APIKey, model: model, http: c.http}
	return c
}

// anthropicWire is the Messages API transport. *Client delegates Chat/Stream
// to it when set (registry picks it via api = "anthropic").
type anthropicWire struct {
	name   string
	base   string
	apiKey string
	model  string
	http   *http.Client
}

// ---- wire types ----

type anthroContent struct {
	Type      string `json:"type"` // text | tool_use | tool_result
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   any    `json:"content,omitempty"` // tool_result content
}

type anthroMessage struct {
	Role    string          `json:"role"` // user | assistant
	Content []anthroContent `json:"content"`
}

type anthroTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"`
}

type anthroRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []anthroMessage `json:"messages"`
	Tools     []anthroTool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream,omitempty"`
}

type anthroResponse struct {
	Content []struct {
		Type  string `json:"type"`
		Text  string `json:"text,omitempty"`
		ID    string `json:"id,omitempty"`
		Name  string `json:"name,omitempty"`
		Input any    `json:"input,omitempty"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// toRequest translates the OpenAI-shaped Request.
func (w *anthropicWire) toRequest(req Request) anthroRequest {
	var out anthroRequest
	out.Model = req.Model
	if out.Model == "" {
		out.Model = w.model
	}
	out.MaxTokens = req.MaxTokens
	if out.MaxTokens == 0 {
		out.MaxTokens = 8192 // required by Anthropic
	}
	var sys []string
	for _, m := range req.Messages {
		if m.Role == "system" {
			sys = append(sys, m.Content)
			continue
		}
		am := anthroMessage{Role: m.Role}
		if m.Role == "tool" {
			am.Role = "user"
			am.Content = []anthroContent{{Type: "tool_result", ToolUseID: m.ToolCallID, Content: m.Content}}
		} else if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				var input any
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				am.Content = append(am.Content, anthroContent{
					Type: "tool_use", ID: tc.ID, Name: tc.Function.Name, Input: input,
				})
			}
		} else {
			am.Content = []anthroContent{{Type: "text", Text: m.Content}}
		}
		out.Messages = append(out.Messages, am)
	}
	out.System = strings.Join(sys, "\n")
	for _, t := range req.Tools {
		out.Tools = append(out.Tools, anthroTool{Name: t.Function.Name, Description: t.Function.Description, InputSchema: t.Function.Parameters})
	}
	return out
}

// toResponse translates an anthroResponse into the OpenAI shape.
func (w *anthropicWire) toResponse(ar anthroResponse) *Response {
	out := &Response{}
	msg := Message{Role: "assistant"}
	for _, b := range ar.Content {
		switch b.Type {
		case "text":
			msg.Content += b.Text
		case "tool_use":
			args, _ := json.Marshal(b.Input)
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID: b.ID, Type: "function",
				Function: ToolFunction{Name: b.Name, Arguments: string(args)},
			})
		}
	}
	out.Choices = []Choice{{Message: msg}}
	out.Usage = Usage{PromptTokens: ar.Usage.InputTokens, CompletionTokens: ar.Usage.OutputTokens}
	return out
}

// Chat is the non-streaming Messages call (agent.LLM).
func (w *anthropicWire) Chat(ctx context.Context, req Request) (*Response, error) {
	body, err := json.Marshal(w.toRequest(req))
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, w.base+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	w.headers(httpReq)
	resp, err := w.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("%s: %s: %s", w.name, resp.Status, strings.TrimSpace(string(msg)))
	}
	var ar anthroResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("%s: decode: %w", w.name, err)
	}
	return w.toResponse(ar), nil
}

// StreamChat streams SSE deltas (agent.Streamer).
func (w *anthropicWire) StreamChat(ctx context.Context, req Request, onDelta func(string), onThinking ...func(string)) (*Response, error) {
	ar := w.toRequest(req)
	ar.Stream = true
	body, err := json.Marshal(ar)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, w.base+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	w.headers(httpReq)
	resp, err := w.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("%s: %s: %s", w.name, resp.Status, strings.TrimSpace(string(msg)))
	}

	out := &Response{}
	msg := Message{Role: "assistant"}
	out.Choices = []Choice{{Message: msg}}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var ev struct {
			Type  string `json:"type"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				Thinking    string `json:"thinking"`
			} `json:"delta"`
			ContentBlock struct {
				Type  string `json:"type"`
				ID    string `json:"id"`
				Name  string `json:"name"`
				Input any    `json:"input"`
			} `json:"content_block"`
			Message struct {
				StopReason string `json:"stop_reason"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "content_block_start":
			if ev.ContentBlock.Type == "tool_use" {
				msg.ToolCalls = append(msg.ToolCalls, ToolCall{
					ID: ev.ContentBlock.ID, Type: "function",
					Function: ToolFunction{Name: ev.ContentBlock.Name},
				})
			}
		case "content_block_delta":
			switch ev.Delta.Type {
			case "text_delta":
				msg.Content += ev.Delta.Text
				onDelta(ev.Delta.Text)
			case "input_json_delta":
				if len(msg.ToolCalls) > 0 {
					msg.ToolCalls[len(msg.ToolCalls)-1].Function.Arguments += ev.Delta.PartialJSON
				}
			case "thinking_delta":
				if len(onThinking) > 0 {
					onThinking[0](ev.Delta.Thinking)
				}
			}
		case "message_delta":
			_ = ev.Message.StopReason
		}
	}
	out.Choices[0].Message = msg
	return out, nil
}

func (w *anthropicWire) headers(r *http.Request) {
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("x-api-key", w.apiKey)
	r.Header.Set("anthropic-version", "2023-06-01")
}
