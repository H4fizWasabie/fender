package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/provider"
)

type streamFake struct {
	*fakeLLM
}

func (s *streamFake) StreamChat(ctx context.Context, req provider.Request, onDelta func(string), onThinking ...func(string)) (*provider.Response, error) {
	for _, d := range []string{"hel", "lo"} {
		onDelta(d)
	}
	if len(onThinking) > 0 {
		onThinking[0]("hmm...")
	}
	return completeReply("complete", "done"), nil
}

func TestObserverNonStreaming(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{textReply("thinking"), completeReply("complete", "done")}}
	a := NewAgent(fake, newTestRegistry(t))
	var events []Event
	a.Observer = func(e Event) { events = append(events, e) }
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "go"}})
	var deltas []string
	for _, e := range events {
		if e.Kind == "delta" {
			deltas = append(deltas, e.Text)
		}
	}
	if len(deltas) != 2 || deltas[0] != "thinking" || deltas[1] != "" {
		t.Fatalf("deltas = %v", deltas)
	}
	found := false
	for _, e := range events {
		if e.Kind == "done" && e.Status == "complete" {
			found = true
		}
	}
	if !found {
		t.Fatalf("done event missing: %+v", events)
	}
}

func TestObserverStreaming(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{completeReply("complete", "done")}}
	sf := &streamFake{fakeLLM: fake}
	a := NewAgent(sf, newTestRegistry(t))
	var events []Event
	a.Observer = func(e Event) { events = append(events, e) }
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "go"}})
	var deltas []string
	for _, e := range events {
		if e.Kind == "delta" {
			deltas = append(deltas, e.Text)
		}
	}
	if len(deltas) != 2 || deltas[0] != "hel" || deltas[1] != "lo" {
		t.Fatalf("streamed deltas = %v", deltas)
	}
}

func TestObserverToolEvent(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{
		toolReply("c1", "shell", `{"command":"echo hi"}`),
		completeReply("complete", "done"),
	}}
	a := NewAgent(fake, newTestRegistry(t))
	var events []Event
	a.Observer = func(e Event) { events = append(events, e) }
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "run echo"}})
	found := false
	for _, e := range events {
		if e.Kind == "tool" && e.Text == "shell" {
			if !strings.Contains(e.Detail, "hi") {
				t.Fatalf("tool event detail = %q, want real tool output", e.Detail)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("tool event missing: %+v", events)
	}
}

func TestObserverThinkingEvent(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{completeReply("complete", "done")}}
	sf := &streamFake{fakeLLM: fake}
	a := NewAgent(sf, newTestRegistry(t))
	var events []Event
	a.Observer = func(e Event) { events = append(events, e) }
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "go"}})
	found := false
	for _, e := range events {
		if e.Kind == "thinking" && e.Text == "hmm..." {
			found = true
		}
	}
	if !found {
		t.Fatalf("thinking event missing: %+v", events)
	}
}

func TestObserverDetailIsBoundedHeadAndTail(t *testing.T) {
	out := "HEAD" + strings.Repeat("x", maxEventDetail) + "TAIL"
	detail := eventDetail(out)
	if got := len([]rune(detail)); got > maxEventDetail {
		t.Fatalf("detail length = %d, want <= %d", got, maxEventDetail)
	}
	if !strings.HasPrefix(detail, "HEAD") || !strings.HasSuffix(detail, "TAIL") || !strings.Contains(detail, "truncated") {
		t.Fatalf("bounded detail lost head/tail marker: %q", detail)
	}
}

func TestEventJSONTags(t *testing.T) {
	// The dashboard SSE marshals events; the browser switches on lowercase
	// keys — uppercase field names would render nothing (ticket-12 bug).
	data, err := json.Marshal(Event{Kind: "done", Text: "r", Status: "complete"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"kind":"done"`, `"text":"r"`, `"status":"complete"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("event JSON missing %s: %s", want, data)
		}
	}
}
