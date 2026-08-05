package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	ctxpkg "github.com/H4fizWasabie/fender/internal/context"
	"github.com/H4fizWasabie/fender/internal/memory"
	"github.com/H4fizWasabie/fender/internal/provider"
)

func TestDelegateRunsEphemeralChildOnParentProvider(t *testing.T) {
	llm := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"count the files"}`),
		completeReply("complete", "3 files found"),
		completeReply("complete", "parent done"),
	}}
	a, _ := newTestAgent(t, llm)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" || res.Reply != "parent done" {
		t.Fatalf("res = %+v", res)
	}
	if got := lastToolResult(t, llm); !strings.Contains(got, "[delegate complete] 3 files found") {
		t.Fatalf("tool result = %q", got)
	}

	requests := llm.all()
	if len(requests) != 3 {
		t.Fatalf("requests = %d, want parent + child + parent", len(requests))
	}
	childReq := requests[1]
	if childReq.Messages[0].Role != "system" || !strings.Contains(childReq.Messages[0].Content, "ephemeral") {
		t.Fatalf("child system prompt = %+v", childReq.Messages[0])
	}
	for _, td := range childReq.Tools {
		if td.Function.Name == "delegate" {
			t.Fatal("child must not be able to create grandchildren")
		}
	}
}

func TestDelegateEmptyPrompt(t *testing.T) {
	llm := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":""}`),
		completeReply("complete", "ok"),
	}}
	a, _ := newTestAgent(t, llm)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, llm); !strings.Contains(got, "empty prompt") {
		t.Fatalf("tool result = %q", got)
	}
}

func TestDelegateBlocked(t *testing.T) {
	llm := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"do it"}`),
		completeReply("blocked", "need access"),
		completeReply("complete", "parent handled it"),
	}}
	a, _ := newTestAgent(t, llm)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, llm); !strings.Contains(got, "[delegate blocked] need access") {
		t.Fatalf("tool result = %q", got)
	}
}

func TestDelegateChildGetsOwnContext(t *testing.T) {
	llm := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"do the big thing"}`),
		toolReply("call_c1", "shell", `{"command":"printf 'y%.0s' {1..9000}"}`),
		completeReply("complete", "child done"),
		completeReply("complete", "parent done"),
	}}
	a, _ := newTestAgent(t, llm)
	a.Ctx = ctxpkg.New()
	a.Ctx.Root = filepath.Join(t.TempDir(), "parent-run")
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	pointer := ""
	for _, req := range llm.all() {
		for _, msg := range req.Messages {
			if strings.Contains(msg.Content, "[artifact:") {
				pointer = msg.Content
			}
		}
	}
	if pointer == "" {
		t.Fatal("child never compacted")
	}
	_, after, _ := strings.Cut(pointer, " at ")
	path, _, _ := strings.Cut(after, ";")
	if !strings.HasPrefix(path, filepath.Dir(a.Ctx.Root)+"/") || strings.HasPrefix(path, a.Ctx.Root+"/") {
		t.Fatalf("child artifact not isolated: %q", path)
	}
	if strings.Contains(a.Ctx.Catalog(), path) {
		t.Fatal("child artifact recorded in parent catalog")
	}
}

func TestDelegateStreamsChildEvents(t *testing.T) {
	llm := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"count"}`),
		completeReply("complete", "counted"),
		completeReply("complete", "parent done"),
	}}
	a, _ := newTestAgent(t, llm)
	var events []Event
	a.Observer = func(e Event) { events = append(events, e) }
	a.Run(context.Background(), nil)
	for _, e := range events {
		if e.Source == "child" {
			return
		}
	}
	t.Fatalf("no child-sourced event: %+v", events)
}

func TestDelegateChildMemoryHandleIsDistinct(t *testing.T) {
	parent := memory.New(t.TempDir())
	child := parent.Child()
	if child == nil || child == parent {
		t.Fatal("child memory must be a distinct handle")
	}
	if _, err := child.Bootstrap(); err != nil {
		t.Fatal(err)
	}
}
