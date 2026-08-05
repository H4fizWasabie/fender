package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	ctxpkg "github.com/H4fizWasabie/fender/internal/context"
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
	if got := lastToolResult(t, parent); !strings.Contains(got, "[delegate complete via child] 3 files found") {
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
	if got := lastToolResult(t, parent); !strings.Contains(got, "[delegate complete via parent-model] done") {
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
	if got := lastToolResult(t, parent); !strings.Contains(got, "[delegate blocked via child] need access") {
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

func TestDelegateChildGetsOwnContext(t *testing.T) {
	child := &fakeLLM{steps: []*provider.Response{
		toolReply("call_c1", "shell", `{"command":"printf 'y%.0s' {1..9000}"}`),
		completeReply("complete", "child done"),
	}}
	parent := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"do the big thing","provider":"child"}`),
		completeReply("complete", "parent done"),
	}}
	a, _ := newTestAgent(t, parent)
	a.Resolver = func(name string) (LLM, error) { return child, nil }
	a.Ctx = ctxpkg.New()
	a.Ctx.Root = filepath.Join(t.TempDir(), "parent-run")
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	// The child loop compacted its big output into ITS run dir.
	pointer := ""
	for _, req := range child.all() {
		for _, m := range req.Messages {
			if strings.Contains(m.Content, "[artifact:") {
				pointer = m.Content
			}
		}
	}
	if pointer == "" {
		t.Fatal("child never compacted")
	}
	_, after, _ := strings.Cut(pointer, " at ")
	path, _, _ := strings.Cut(after, ";")
	if !strings.HasPrefix(path, filepath.Dir(a.Ctx.Root)+"/") {
		t.Fatalf("child artifact outside child root: %q", path)
	}
	if strings.HasPrefix(path, a.Ctx.Root+"/") {
		t.Fatal("child artifact leaked into parent root")
	}
	if strings.Contains(a.Ctx.Catalog(), path) {
		t.Fatal("child artifact recorded in parent catalog")
	}
}

// D48: subagent events stream through the parent observer, source-tagged.
func TestDelegateStreamsSourceTaggedEvents(t *testing.T) {
	child := &fakeLLM{steps: []*provider.Response{completeReply("complete", "3 files found")}}
	parent := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"count the files","provider":"child"}`),
		completeReply("complete", "parent done"),
	}}
	a, _ := newTestAgent(t, parent)
	a.Resolver = func(name string) (LLM, error) { return child, nil }
	var events []Event
	a.Observer = func(e Event) { events = append(events, e) }
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "delegate it"}})
	sawChildDelta := false
	for _, e := range events {
		if e.Source == "subagent:child" {
			sawChildDelta = true
		}
	}
	if !sawChildDelta {
		t.Fatalf("no source-tagged subagent events: %+v", events)
	}
}

// D48: config `subagent =` provides the default provider for delegates
// that omit the provider argument.
func TestDelegateDefaultSubagent(t *testing.T) {
	child := &fakeLLM{steps: []*provider.Response{completeReply("complete", "done")}}
	parent := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"go"}`), // no provider arg
		completeReply("complete", "parent done"),
	}}
	a, _ := newTestAgent(t, parent)
	a.DefaultSubagent = "default-child"
	a.Resolver = func(name string) (LLM, error) {
		if name != "default-child" {
			t.Fatalf("resolver got %q, want default-child", name)
		}
		return child, nil
	}
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "delegate"}})
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
}
