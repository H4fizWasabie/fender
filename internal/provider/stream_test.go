package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamCollectsDeltas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"content\":\"\"}}]}\n\n" +
				"data: [DONE]\n\n",
		))
	}))
	defer srv.Close()

	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m1"}, DefaultModel: "m1"})
	var got strings.Builder
	resp, err := c.Stream(context.Background(), Request{Model: "m1", Messages: []Message{{Role: "user", Content: "hi"}}}, func(d string) {
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

func TestStreamCollectsToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"read_file\",\"arguments\":\"{\\\"pa\"}}]}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"th\\\":\\\"a.go\\\"}\"}}]}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{}}]}\n\n" +
				"data: [DONE]\n\n",
		))
	}))
	defer srv.Close()

	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m1"}, DefaultModel: "m1"})
	resp, err := c.Stream(context.Background(), Request{Model: "m1", Messages: []Message{{Role: "user", Content: "read"}}}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	tc := resp.Choices[0].Message.ToolCalls
	if len(tc) != 1 || tc[0].ID != "call_1" || tc[0].Function.Name != "read_file" {
		t.Fatalf("tool_calls = %+v", tc)
	}
	if !strings.Contains(tc[0].Function.Arguments, `"path":"a.go"`) {
		t.Fatalf("arguments = %q", tc[0].Function.Arguments)
	}
}

func TestStreamSetsRole(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()
	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m"}, DefaultModel: "m"})
	resp, err := c.Stream(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "x"}}}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Fatalf("role = %q", resp.Choices[0].Message.Role)
	}
}

// D60: the final SSE chunk's usage must reach the accumulated response —
// without it the token meter records zeros on the streaming path.
func TestStreamCapturesUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{}}],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":20,\"prompt_tokens_details\":{\"cached_tokens\":80}}}\n\n" +
				"data: [DONE]\n\n"))
	}))
	defer srv.Close()
	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m"}, DefaultModel: "m"})
	resp, err := c.Stream(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "x"}}}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.PromptTokens != 100 || resp.Usage.CompletionTokens != 20 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
	if resp.Usage.Cached() != 80 {
		t.Fatalf("cached = %d", resp.Usage.Cached())
	}
	m := &Meter{Window: 1000}
	m.Record(resp.Usage)
	if m.UsagePercent() != 10.0 || m.CacheHitRate() != 80.0 {
		t.Fatalf("meter: %+v", m)
	}
}
