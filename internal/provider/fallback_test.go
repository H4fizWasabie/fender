package provider

import (
	"context"
	"errors"
	"testing"
)

type fallbackFake struct {
	name       string
	reply      string
	err        error
	streamText string
	calls      int
	streams    int
}

func (f *fallbackFake) Chat(context.Context, Request) (*Response, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &Response{Choices: []Choice{{Message: Message{Role: "assistant", Content: f.reply}}}}, nil
}

func (f *fallbackFake) StreamChat(_ context.Context, _ Request, onDelta func(string), _ ...func(string)) (*Response, error) {
	f.streams++
	if f.streamText != "" {
		onDelta(f.streamText)
	}
	if f.err != nil {
		return nil, f.err
	}
	return &Response{Choices: []Choice{{Message: Message{Role: "assistant", Content: f.streamText}}}}, nil
}

func (f *fallbackFake) Name() string             { return f.name }
func (f *fallbackFake) Model() string            { return "model" }
func (f *fallbackFake) SetThinking(string) error { return nil }
func (f *fallbackFake) Thinking() string         { return "" }

func TestFallbackChatRetriesBackup(t *testing.T) {
	primary := &fallbackFake{name: "primary", err: errors.New("unavailable")}
	backup := &fallbackFake{name: "backup", reply: "recovered"}
	c := NewFallback(primary, backup)
	resp, err := c.Chat(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Choices[0].Message.Content; got != "recovered" {
		t.Fatalf("reply = %q", got)
	}
	if primary.calls != 1 || backup.calls != 1 {
		t.Fatalf("calls primary=%d backup=%d", primary.calls, backup.calls)
	}
}

func TestFallbackStreamRetriesBeforeOutput(t *testing.T) {
	primary := &fallbackFake{name: "primary", err: errors.New("unavailable")}
	backup := &fallbackFake{name: "backup", streamText: "recovered"}
	c := NewFallback(primary, backup)
	var got string
	if _, err := c.StreamChat(context.Background(), Request{}, func(s string) { got += s }); err != nil {
		t.Fatal(err)
	}
	if got != "recovered" || backup.streams != 1 {
		t.Fatalf("output=%q backup streams=%d", got, backup.streams)
	}
}

func TestFallbackStreamDoesNotRetryPartialOutput(t *testing.T) {
	primary := &fallbackFake{name: "primary", streamText: "partial", err: errors.New("cut off")}
	backup := &fallbackFake{name: "backup", streamText: "duplicate"}
	c := NewFallback(primary, backup)
	var got string
	if _, err := c.StreamChat(context.Background(), Request{}, func(s string) { got += s }); err == nil {
		t.Fatal("expected partial-stream error")
	}
	if got != "partial" || backup.streams != 0 {
		t.Fatalf("output=%q backup streams=%d", got, backup.streams)
	}
}
