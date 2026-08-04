package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/provider"
)

func TestDelegate(t *testing.T) {
	child := &fakeLLM{steps: []*provider.Response{completeReply("complete", "3 files found")}}
	parent := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"count the files","provider":"child"}`),
		completeReply("complete", "parent done"),
	}}
	a, _ := newTestAgent(t, parent)
	a.Resolver = func(name string) (LLM, error) {
		if name != "child" {
			t.Fatalf("resolver got %q", name)
		}
		return child, nil
	}
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, parent); !strings.Contains(got, "[delegate complete] 3 files found") {
		t.Fatalf("tool result = %q", got)
	}
	// The child ran its own loop with its own LLM and no delegate tool.
	if len(child.all()) == 0 {
		t.Fatal("child loop never ran")
	}
	for _, req := range child.all() {
		for _, td := range req.Tools {
			if td.Function.Name == "delegate" {
				t.Fatal("child registry must not include delegate")
			}
		}
		if req.Messages[0].Role != "system" || !strings.Contains(req.Messages[0].Content, "ephemeral") {
			t.Fatalf("child system prompt = %+v", req.Messages[0])
		}
	}
}

func TestDelegateInheritsParentLLM(t *testing.T) {
	// No provider arg + nil Resolver -> the child consumes the parent's
	// scripted steps. The parent calls delegate, then the child loop runs
	// against the SAME fake, then the parent completes.
	parent := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"subtask"}`),
		completeReply("complete", "done"),
		completeReply("complete", "parent done"),
	}}
	a, _ := newTestAgent(t, parent)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" || res.Reply != "parent done" {
		t.Fatalf("res = %+v", res)
	}
	if got := lastToolResult(t, parent); !strings.Contains(got, "[delegate complete] done") {
		t.Fatalf("tool result = %q", got)
	}
}

func TestDelegateEmptyPrompt(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":""}`),
		completeReply("complete", "ok"),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, f); !strings.Contains(got, "empty prompt") {
		t.Fatalf("tool result = %q", got)
	}
}

func TestDelegateBlocked(t *testing.T) {
	child := &fakeLLM{steps: []*provider.Response{completeReply("blocked", "need access")}}
	parent := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"do it","provider":"child"}`),
		completeReply("complete", "parent done"),
	}}
	a, _ := newTestAgent(t, parent)
	a.Resolver = func(name string) (LLM, error) { return child, nil }
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, parent); !strings.Contains(got, "[delegate blocked] need access") {
		t.Fatalf("tool result = %q", got)
	}
}

func TestDelegateUnknownProvider(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"x","provider":"nope"}`),
		completeReply("complete", "ok"),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, f); !strings.Contains(got, "no resolver") {
		t.Fatalf("tool result = %q", got)
	}
}
