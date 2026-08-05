package agent

import (
	"context"
	"testing"

	"github.com/H4fizWasabie/fender/internal/provider"
)

// The asked model gets a SINGLE user message and no tools — it's a pure
// API call (D49: the call IS the subagent).
func TestAskReturnsReply(t *testing.T) {
	asked := &fakeLLM{steps: []*provider.Response{textReply("the other model's answer")}}
	parent := &fakeLLM{steps: []*provider.Response{
		toolReply("c1", "ask", `{"prompt":"what do you think?","provider":"other"}`),
		completeReply("complete", "done"),
	}}
	a, _ := newTestAgent(t, parent)
	a.Resolver = func(name string) (LLM, error) { return asked, nil }
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "ask"}})
	// the asked model must have received exactly one user message, no tools
	reqs := asked.all()
	if len(reqs) != 1 {
		t.Fatalf("asked model called %d times, want 1 (one-shot)", len(reqs))
	}
	if len(reqs[0].Tools) != 0 {
		t.Fatalf("asked model received tools: %+v", reqs[0].Tools)
	}
	if len(reqs[0].Messages) != 1 || reqs[0].Messages[0].Role != "user" {
		t.Fatalf("asked model messages = %+v", reqs[0].Messages)
	}
	// the reply must have reached the parent as a tool result
	if got := lastToolResult(t, parent); !containsStr(got, "other model's answer") {
		t.Fatalf("tool result = %q", got)
	}
}

// Default subagent provider applies when provider is omitted (D49).
func TestAskDefaultProvider(t *testing.T) {
	asked := &fakeLLM{steps: []*provider.Response{textReply("answer")}}
	parent := &fakeLLM{steps: []*provider.Response{
		toolReply("c1", "ask", `{"prompt":"hi"}`),
		completeReply("complete", "done"),
	}}
	a, _ := newTestAgent(t, parent)
	a.DefaultSubagent = "default-other"
	a.Resolver = func(name string) (LLM, error) {
		if name != "default-other" {
			t.Fatalf("resolver got %q", name)
		}
		return asked, nil
	}
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "ask"}})
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
}

func TestAskEmptyPrompt(t *testing.T) {
	parent := &fakeLLM{steps: []*provider.Response{
		toolReply("c1", "ask", `{"prompt":""}`),
		completeReply("complete", "done"),
	}}
	a, _ := newTestAgent(t, parent)
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "ask"}})
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, parent); !containsStr(got, "empty prompt") {
		t.Fatalf("tool result = %q", got)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
