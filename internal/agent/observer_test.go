package agent

import (
	"context"
	"testing"

	"github.com/H4fizWasabie/fender/internal/provider"
)

type streamFake struct {
	*fakeLLM
}

func (s *streamFake) StreamChat(ctx context.Context, req provider.Request, onDelta func(string)) (*provider.Response, error) {
	for _, d := range []string{"hel", "lo"} {
		onDelta(d)
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
			found = true
		}
	}
	if !found {
		t.Fatalf("tool event missing: %+v", events)
	}
}
