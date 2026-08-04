package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicChatTranslation(t *testing.T) {
	var gotBody map[string]any
	var gotKey, gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"content":[{"type":"text","text":"hello from claude"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)
	}))
	defer srv.Close()

	c := NewAnthropic("claude", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"claude-sonnet"}, DefaultModel: "claude-sonnet"})
	resp, err := c.Chat(context.Background(), Request{
		Messages: []Message{
			{Role: "system", Content: "be nice"},
			{Role: "user", Content: "hi"},
		},
		Tools: []ToolDef{{Type: "function", Function: ToolFunctionDef{Name: "read_file", Description: "r", Parameters: map[string]any{"type": "object"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != "k" || gotVersion != "2023-06-01" {
		t.Fatalf("headers: key=%q version=%q", gotKey, gotVersion)
	}
	if gotBody["system"] != "be nice" {
		t.Fatalf("system = %v", gotBody["system"])
	}
	if gotBody["max_tokens"] != float64(8192) {
		t.Fatalf("max_tokens = %v", gotBody["max_tokens"])
	}
	if resp.Choices[0].Message.Content != "hello from claude" {
		t.Fatalf("content = %q", resp.Choices[0].Message.Content)
	}
}

func TestAnthropicToolUseRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"tool_result"`) || !strings.Contains(string(body), `"tool_use"`) {
			t.Fatalf("body missing blocks: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"content":[{"type":"tool_use","id":"tu_1","name":"read_file","input":{"path":"a.go"}}],"stop_reason":"tool_use"}`)
	}))
	defer srv.Close()

	c := NewAnthropic("claude", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m"}, DefaultModel: "m"})
	resp, err := c.Chat(context.Background(), Request{
		Messages: []Message{
			{Role: "user", Content: "read a.go"},
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "call_1", Type: "function", Function: ToolFunction{Name: "read_file", Arguments: `{"path":"a.go"}`}}}},
			{Role: "tool", ToolCallID: "call_1", Content: "contents"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tc := resp.Choices[0].Message.ToolCalls
	if len(tc) != 1 || tc[0].ID != "tu_1" || tc[0].Function.Name != "read_file" || !strings.Contains(tc[0].Function.Arguments, "a.go") {
		t.Fatalf("tool_calls = %+v", tc)
	}
}

func TestAnthropicStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w,
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hel\"}}\n\n"+
				"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"lo\"}}\n\n"+
				"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n"+
				"data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := NewAnthropic("claude", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m"}, DefaultModel: "m"})
	var got strings.Builder
	resp, err := c.StreamChat(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}}, func(d string) {
		got.WriteString(d)
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "hello" {
		t.Fatalf("deltas = %q", got.String())
	}
	if resp.Choices[0].Message.Content != "hello" {
		t.Fatalf("content = %q", resp.Choices[0].Message.Content)
	}
}
