package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/H4fizWasabie/fender/internal/guardrail"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/tools"
)

// fakeLLM is a scripted LLM for loop tests: each Chat consumes one scripted
// response and records the request.
type fakeLLM struct {
	mu    sync.Mutex
	reqs  []provider.Request
	steps []*provider.Response
}

func (f *fakeLLM) Chat(ctx context.Context, req provider.Request) (*provider.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, req)
	if len(f.steps) == 0 {
		return &provider.Response{Choices: []provider.Choice{{Message: provider.Message{Role: "assistant", Content: "(script exhausted)"}}}}, nil
	}
	r := f.steps[0]
	f.steps = f.steps[1:]
	return r, nil
}

func (f *fakeLLM) last() provider.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reqs[len(f.reqs)-1]
}

func (f *fakeLLM) all() []provider.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]provider.Request(nil), f.reqs...)
}

func textReply(s string) *provider.Response {
	return &provider.Response{Choices: []provider.Choice{{Message: provider.Message{Role: "assistant", Content: s}}}}
}

func toolReply(id, name, args string) *provider.Response {
	return &provider.Response{Choices: []provider.Choice{{Message: provider.Message{
		Role: "assistant",
		ToolCalls: []provider.ToolCall{{
			ID: id, Type: "function",
			Function: provider.ToolFunction{Name: name, Arguments: args},
		}},
	}}}}
}

func completeReply(status, reply string) *provider.Response {
	args, _ := json.Marshal(map[string]string{"status": status, "reply": reply})
	return toolReply("call_complete", completionToolName, string(args))
}

// lastToolResult returns the last "tool"-role message content in the last request.
func lastToolResult(t *testing.T, f *fakeLLM) string {
	t.Helper()
	req := f.last()
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "tool" {
			return req.Messages[i].Content
		}
	}
	t.Fatal("no tool result message")
	return ""
}

func newTestAgent(t *testing.T, f *fakeLLM) (*Agent, *tools.Registry) {
	t.Helper()
	proj := t.TempDir()
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	return NewAgent(f, reg), reg
}

func TestRunCompletes(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{completeReply("complete", "all done")}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "do the thing"}})
	if res.Status != "complete" || res.Reply != "all done" || res.Iterations != 1 {
		t.Fatalf("res = %+v", res)
	}
}

func TestRunBlocked(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{completeReply("blocked", "need credentials")}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "blocked" || res.Reply != "need credentials" {
		t.Fatalf("res = %+v", res)
	}
}

func TestRunToolRoundTrip(t *testing.T) {
	proj := t.TempDir()
	os.WriteFile(filepath.Join(proj, "a.txt"), []byte("hello world\n"), 0o644)
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "read_file", `{"path":"a.txt"}`),
		completeReply("complete", "read it"),
	}}
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "read a.txt"}})
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, f); got != "hello world\n" {
		t.Fatalf("tool result = %q", got)
	}
	req := f.last()
	names := map[string]bool{}
	for _, td := range req.Tools {
		names[td.Function.Name] = true
	}
	for _, want := range []string{"read_file", "edit_file", "shell", "search", "complete_task"} {
		if !names[want] {
			t.Fatalf("schema missing %q", want)
		}
	}
}

func TestRunDedup(t *testing.T) {
	proj := t.TempDir()
	os.WriteFile(filepath.Join(proj, "a.txt"), []byte("x"), 0o644)
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "read_file", `{"path":"a.txt"}`),
		toolReply("call_2", "read_file", `{"path":"a.txt"}`),
		completeReply("complete", "ok"),
	}}
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	req := f.last()
	found := false
	for _, m := range req.Messages {
		if m.Role == "tool" && strings.HasPrefix(m.Content, "[already executed]") {
			found = true
		}
	}
	if !found {
		t.Fatal("dedup did not happen")
	}
}

func TestRunUnknownTool(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "no_such_tool", `{}`),
		completeReply("complete", "ok"),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, f); !strings.Contains(got, "unknown tool") {
		t.Fatalf("result = %q", got)
	}
}

func TestRunRejectsInvalidCompletion(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{
		completeReply("sideways", ""), // invalid status + empty reply
		completeReply("complete", "now it's done"),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" || res.Reply != "now it's done" {
		t.Fatalf("res = %+v", res)
	}
	if got := lastToolResult(t, f); !strings.Contains(got, "complete_task") {
		t.Fatalf("result = %q", got)
	}
}

func TestRunStallsWithoutProgress(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{
		textReply("thinking..."), textReply("still thinking..."), textReply("hmm..."),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "stalled" {
		t.Fatalf("status = %q (want stalled)", res.Status)
	}
}


func TestRunMaxIter(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{textReply("x"), textReply("x")}}
	a, _ := newTestAgent(t, f)
	a.MaxIter = 2
	res := a.Run(context.Background(), nil)
	if res.Status != "stalled" {
		t.Fatalf("status = %q (want stalled)", res.Status)
	}
}

func TestRunSystemPrompt(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{completeReply("complete", "ok")}}
	a, _ := newTestAgent(t, f)
	a.System = "be good"
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	m := f.last().Messages[0]
	if m.Role != "system" || m.Content != "be good" {
		t.Fatalf("first message = %+v", m)
	}
}
