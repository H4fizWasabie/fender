package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/provider"
)

func TestDashboardServe(t *testing.T) {
	cfg := writeConfig(t, `
[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`)
	d, err := newDashState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	mux, err := newDashboardMux(d)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "dashboard") {
		t.Fatalf("index.html missing dashboard: %.200s", body)
	}
}

// dashFakeLLM completes instantly via the completion protocol.
type dashFakeLLM struct{}

func (d *dashFakeLLM) Chat(ctx context.Context, req provider.Request) (*provider.Response, error) {
	return &provider.Response{Choices: []provider.Choice{{Message: provider.Message{
		Role: "assistant",
		ToolCalls: []provider.ToolCall{{
			ID: "call_c", Type: "function",
			Function: provider.ToolFunction{Name: "complete_task", Arguments: `{"status":"complete","reply":"hi there"}`},
		}},
	}}}}, nil
}

func TestDashStateRun(t *testing.T) {
	cfg := writeConfig(t, `
[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`)
	d, err := newDashState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	d.agent.LLM = &dashFakeLLM{}

	status, reply, err := d.run(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if status != "complete" || reply != "hi there" {
		t.Fatalf("status=%q reply=%q", status, reply)
	}
	if len(d.history) != 2 {
		t.Fatalf("history = %+v", d.history)
	}

	// broadcast reaches subscribers
	ch := make(chan agent.Event, 8)
	d.mu.Lock()
	d.subs[ch] = struct{}{}
	d.mu.Unlock()
	defer func() { d.mu.Lock(); delete(d.subs, ch); d.mu.Unlock() }()
	d.broadcast(agent.Event{Kind: "tool", Text: "shell", Status: "ok"})
	select {
	case e := <-ch:
		if e.Kind != "tool" {
			t.Fatalf("event = %+v", e)
		}
	default:
		t.Fatal("subscriber missed broadcast")
	}
}
