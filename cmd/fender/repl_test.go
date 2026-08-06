package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/agent"
)

func TestReplQuit(t *testing.T) {
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/quit\n"))
	if err := repl(&out, &errOut, in, "", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "fender") {
		t.Fatalf("banner missing: %q", out.String())
	}
}

func TestReplHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/help\n/quit\n"))
	if err := repl(&out, &errOut, in, "", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "/quit") || !strings.Contains(out.String(), "/mode") {
		t.Fatalf("help missing commands: %q", out.String())
	}
}

func TestReplUnknownSlash(t *testing.T) {
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/nope\n/quit\n"))
	if err := repl(&out, &errOut, in, "", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unknown skill") && !strings.Contains(out.String(), "unknown command") {
		t.Fatalf("expected unknown-command/skill error: %q", out.String())
	}
}

func TestReplModelUnknown(t *testing.T) {
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/model does-not-exist\n/quit\n"))
	if err := repl(&out, &errOut, in, "", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "error:") {
		t.Fatalf("expected error line: %q", out.String())
	}
}

func TestRenderEventThinkingDimmed(t *testing.T) {
	var out bytes.Buffer
	renderEvent(&out, agent.Event{Kind: "thinking", Text: "hmm"}, true)
	if !strings.Contains(out.String(), "\x1b[2mhmm\x1b[0m") {
		t.Fatalf("dimmed rendering missing: %q", out.String())
	}
	var out2 bytes.Buffer
	renderEvent(&out2, agent.Event{Kind: "thinking", Text: "hmm"}, false)
	if out2.String() != "" {
		t.Fatalf("thinking must be hidden at off: %q", out2.String())
	}
}

func TestReplThinkingUnknownLevel(t *testing.T) {
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/thinking nope\n/quit\n"))
	if err := repl(&out, &errOut, in, "", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "invalid level") {
		t.Fatalf("expected invalid level error: %q", out.String())
	}
}
