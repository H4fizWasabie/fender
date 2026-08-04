package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestReplQuit(t *testing.T) {
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/quit\n"))
	if err := repl(&out, &errOut, in, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "fender") {
		t.Fatalf("banner missing: %q", out.String())
	}
}

func TestReplHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/help\n/quit\n"))
	if err := repl(&out, &errOut, in, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "/quit") || !strings.Contains(out.String(), "/mode") {
		t.Fatalf("help missing commands: %q", out.String())
	}
}

func TestReplUnknownSlash(t *testing.T) {
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/nope\n/quit\n"))
	if err := repl(&out, &errOut, in, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unknown command") {
		t.Fatalf("expected unknown-command error: %q", out.String())
	}
}

func TestReplModelUnknown(t *testing.T) {
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/model does-not-exist\n/quit\n"))
	if err := repl(&out, &errOut, in, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "error:") {
		t.Fatalf("expected error line: %q", out.String())
	}
}
