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

func TestChatSendsRequestAndParsesResponse(t *testing.T) {
	var gotBody map[string]any
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"hello from mock"}}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
	}))
	defer srv.Close()

	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m1"}, DefaultModel: "m1"})
	resp, err := c.Chat(context.Background(), Request{
		Model:    "m1",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer k" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotBody["model"] != "m1" {
		t.Fatalf("model = %v", gotBody["model"])
	}
	if resp.Choices[0].Message.Content != "hello from mock" {
		t.Fatalf("content = %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Fatalf("prompt_tokens = %d", resp.Usage.PromptTokens)
	}
}

func TestChatParsesToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"a.go\"}"}}]}}]}`)
	}))
	defer srv.Close()

	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m1"}, DefaultModel: "m1"})
	resp, err := c.Chat(context.Background(), Request{Model: "m1", Messages: []Message{{Role: "user", Content: "read a file"}}})
	if err != nil {
		t.Fatal(err)
	}
	tc := resp.Choices[0].Message.ToolCalls
	if len(tc) != 1 || tc[0].ID != "call_1" || tc[0].Function.Name != "read_file" {
		t.Fatalf("tool_calls = %+v", tc)
	}
}

func TestChatReturnsErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"bad key"}}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m1"}, DefaultModel: "m1"})
	_, err := c.Chat(context.Background(), Request{Model: "m1", Messages: []Message{{Role: "user", Content: "x"}}})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v", err)
	}
}
