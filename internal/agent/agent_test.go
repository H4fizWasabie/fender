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
	"github.com/H4fizWasabie/fender/internal/memory"
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

func TestRunDoesNotAccumulateBootstrapSystem(t *testing.T) {
	proj := t.TempDir()
	f := &fakeLLM{steps: []*provider.Response{
		completeReply("complete", "first"),
		completeReply("complete", "second"),
	}}
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	a.System = "base system"
	a.Mem = memory.New(proj)
	if err := a.Mem.Ensure(); err != nil {
		t.Fatal(err)
	}
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "one"}})
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "two"}})
	if a.System != "base system" {
		t.Fatalf("Agent.System accumulated per-run bootstrap content: %q", a.System)
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

func TestRunAcceptsConversationalProse(t *testing.T) {
	// D53: pure-prose turns are CHAT answers — after two nags the harness
	// accepts the last prose instead of stalling (this is what locked the
	// dashboard input while the model "waited" for the user).
	f := &fakeLLM{steps: []*provider.Response{
		textReply("Question 1: what do the PDFs look like?"),
		textReply("Question 1: what do the PDFs look like?"),
		textReply("Question 1: what do the PDFs look like?"),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q (want complete)", res.Status)
	}
	if res.Reply != "Question 1: what do the PDFs look like?" {
		t.Fatalf("reply = %q", res.Reply)
	}
	// the conversational escape must have fired WITHOUT an orientation turn
	for _, m := range f.last().Messages {
		if strings.Contains(m.Content, "orientation turn") {
			t.Fatal("orientation must not fire for conversational prose")
		}
	}
}

func TestRunOrientationOnToolError(t *testing.T) {
	// D52: the first failing call errors; identical repeats hit the dedup
	// cache ("[already executed]") — no re-execution. The repeat counter
	// (not the error counter) triggers orientation.
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "read_file", `{"path":"nope.txt"}`),
		toolReply("call_2", "read_file", `{"path":"nope.txt"}`),
		toolReply("call_3", "read_file", `{"path":"nope.txt"}`),
		toolReply("call_4", "read_file", `{"path":"nope.txt"}`),
		completeReply("complete", "recovered"),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	oriented := 0
	for _, m := range f.last().Messages { // full history: counts injected turns
		if strings.Contains(m.Content, "orientation turn") {
			oriented++
		}
	}
	if oriented != 1 {
		t.Fatalf("orientation turns = %d (want exactly 1)", oriented)
	}
}

func TestRunOrientationOnRepeatedCall(t *testing.T) {
	// A successful call repeated with identical args hits the dedup cache
	// ("[already executed]") — the second repeat triggers orientation.
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "read_file", `{"path":"a.txt"}`),
		toolReply("call_2", "read_file", `{"path":"a.txt"}`),
		toolReply("call_3", "read_file", `{"path":"a.txt"}`),
		toolReply("call_4", "read_file", `{"path":"a.txt"}`),
		completeReply("complete", "ok"),
	}}
	proj := t.TempDir()
	os.WriteFile(filepath.Join(proj, "a.txt"), []byte("x"), 0o644)
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	oriented := 0
	for _, m := range f.last().Messages { // full history: counts injected turns
		if strings.Contains(m.Content, "orientation turn") {
			oriented++
		}
	}
	if oriented != 1 {
		t.Fatalf("orientation turns = %d (want exactly 1)", oriented)
	}
}

func TestRunStallsAfterOrientationOnErrors(t *testing.T) {
	// DISTINCT failing calls (no dedup) keep growing the error counter —
	// after orientation, 2 more errors stall the run (D52/D53 unchanged).
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "read_file", `{"path":"nope1.txt"}`),
		toolReply("call_2", "read_file", `{"path":"nope2.txt"}`),
		toolReply("call_3", "read_file", `{"path":"nope3.txt"}`),
		toolReply("call_4", "read_file", `{"path":"nope4.txt"}`),
		toolReply("call_5", "read_file", `{"path":"nope5.txt"}`),
		toolReply("call_6", "read_file", `{"path":"nope6.txt"}`),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "stalled" {
		t.Fatalf("status = %q (want stalled)", res.Status)
	}
}

func TestRunMaxIter(t *testing.T) {
	// tool-error loop (distinct calls): the iteration cap still bounds it
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("c1", "read_file", `{"path":"nope1.txt"}`),
		toolReply("c2", "read_file", `{"path":"nope2.txt"}`),
	}}
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

func TestRunKeepsLargeOutputInline(t *testing.T) {
	// D56 (pi-style): tool output stays inline — no artifact pointers,
	// the model sees the real result, no re-read round trips.
	proj := t.TempDir()
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "shell", `{"command":"printf 'y%.0s' {1..18000}"}`),
		completeReply("complete", "done"),
	}}
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Yolo, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "run it"}})
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	got := lastToolResult(t, f)
	if strings.Contains(got, "[artifact:") {
		t.Fatalf("artifact pointer must not exist anymore: %.80q", got)
	}
	if !strings.Contains(got, strings.Repeat("y", 17000)) {
		t.Fatal("full output must be inline")
	}
}

func TestRunReadFileStaysInline(t *testing.T) {
	proj := t.TempDir()
	big := strings.Repeat("r", 17000)
	if err := os.WriteFile(filepath.Join(proj, "big.txt"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "read_file", `{"path":"big.txt"}`),
		completeReply("complete", "ok"),
	}}
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, f); got != big {
		t.Fatalf("read_file result altered: %d chars", len(got))
	}
}

func TestRunDedupReplaysOutputInline(t *testing.T) {
	proj := t.TempDir()
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "shell", `{"command":"printf 'y%.0s' {1..18000}"}`),
		toolReply("call_2", "shell", `{"command":"printf 'y%.0s' {1..18000}"}`),
		completeReply("complete", "ok"),
	}}
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Yolo, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	found := false
	for _, m := range f.last().Messages {
		if m.Role == "tool" && strings.HasPrefix(m.Content, "[already executed]") && !strings.Contains(m.Content, "[artifact:") {
			found = true
		}
	}
	if !found {
		t.Fatal("dedup replay must show the real output, no artifact pointers")
	}
}

func TestRunKeepsLargeUserInputInline(t *testing.T) {
	// D56: the user's full input reaches the model — no HEAD/TAIL.
	f := &fakeLLM{steps: []*provider.Response{completeReply("complete", "ok")}}
	a, _ := newTestAgent(t, f)
	big := strings.Repeat("t", 30000)
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: big}})
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if msg := f.last().Messages[0]; !strings.Contains(msg.Content, strings.Repeat("t", 30000)) {
		t.Fatalf("task truncated: %.100q", msg.Content)
	}
}

func TestToolCallsExecuteSequentiallyInModelOrder(t *testing.T) {
	proj := t.TempDir()
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	var order []string
	reg.Add(tools.Tool{
		Name: "record", Parameters: map[string]any{"type": "object"},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			id, _ := args["id"].(string)
			order = append(order, id)
			return id, nil
		},
	})
	multi := &provider.Response{Choices: []provider.Choice{{Message: provider.Message{
		Role: "assistant",
		ToolCalls: []provider.ToolCall{
			{ID: "c1", Type: "function", Function: provider.ToolFunction{Name: "record", Arguments: `{"id":"a"}`}},
			{ID: "c2", Type: "function", Function: provider.ToolFunction{Name: "record", Arguments: `{"id":"b"}`}},
			{ID: "c3", Type: "function", Function: provider.ToolFunction{Name: "record", Arguments: `{"id":"c"}`}},
		},
	}}}}
	fake := &fakeLLM{steps: []*provider.Response{
		multi,
		completeReply("complete", "done"),
	}}
	a := NewAgent(fake, reg)
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "probe thrice"}})
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := strings.Join(order, ""); got != "abc" {
		t.Fatalf("execution order = %q", got)
	}
}

// D52: cosmetically different shell commands dedup to one execution.
func TestShellCommandNormalizationDedups(t *testing.T) {
	proj := t.TempDir()
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	fake := &fakeLLM{steps: []*provider.Response{
		toolReply("c1", "shell", `{"command":"go test ./..."}`),
		toolReply("c2", "shell", `{"command":"go test  ./... 2>&1 | tail -5"}`),
		completeReply("complete", "done"),
	}}
	a := NewAgent(fake, reg)
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "run tests twice"}})
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	got := lastToolResult(t, fake)
	if !strings.Contains(got, "[already executed]") {
		t.Fatalf("second call not deduped: %q", got)
	}
}

func TestNormalizeCmd(t *testing.T) {
	cases := map[string]string{
		`go test ./...`:                    `go test ./...`,
		`go test  ./... 2>&1`:              `go test ./...`,
		`go test ./... | tail -5`:          `go test ./...`,
		`echo "hi" > out.txt`:              `echo hi`,
		`go test ./pkgA`:                   `go test ./pkgA`, // different target stays different
	}
	for in, want := range cases {
		if got := normalizeCmd(in); got != want {
			t.Fatalf("normalizeCmd(%q) = %q, want %q", in, got, want)
		}
	}
}
