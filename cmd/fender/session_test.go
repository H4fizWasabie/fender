package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/provider"
)

// chdir runs f in a fresh cwd (so sessions land in temp, not the repo) and
// restores the original on test end.
func chdir(t *testing.T, dir string) {
	t.Helper()
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

// save → load round-trip preserves messages (temp .fender dir) — D41.
func TestSessionRoundTrip(t *testing.T) {
	chdir(t, t.TempDir())
	s := &sessionFile{
		ID:      "20260804-235959",
		Started: "2026-08-04T23:59:59Z",
		Messages: []provider.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}
	if err := saveSession(s); err != nil {
		t.Fatal(err)
	}
	got, err := loadLatestSession()
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != s.ID || got.Started != s.Started || len(got.Messages) != 2 || got.Messages[1].Content != "hi" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

// loadLatest picks the newest by ID (two files).
func TestLoadLatestPicksNewest(t *testing.T) {
	chdir(t, t.TempDir())
	for _, s := range []*sessionFile{
		{ID: "20260804-010000", Started: "old", Messages: []provider.Message{{Role: "user", Content: "old"}}},
		{ID: "20260804-020000", Started: "new", Messages: []provider.Message{{Role: "user", Content: "new"}}},
	} {
		if err := saveSession(s); err != nil {
			t.Fatal(err)
		}
	}
	got, err := loadLatestSession()
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != "20260804-020000" {
		t.Fatalf("expected newest, got %+v", got)
	}
}

// no sessions → nil, no error.
func TestLoadLatestNone(t *testing.T) {
	chdir(t, t.TempDir())
	got, err := loadLatestSession()
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

// `fender sessions` lists saved sessions.
func TestSessionsCommand(t *testing.T) {
	chdir(t, t.TempDir())
	if err := saveSession(&sessionFile{ID: "20260804-3", Started: "2026", Messages: []provider.Message{{Role: "user", Content: "x"}}}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runCLI(&out, []string{"sessions"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "20260804-3") || !strings.Contains(out.String(), "1 messages") {
		t.Fatalf("output = %q", out.String())
	}
}

// REPL: history resumes only when explicitly selected, then persists into a
// new session file so the original remains an immutable recovery point.
func TestReplExplicitResumeAndPersists(t *testing.T) {
	chdir(t, t.TempDir())
	seed := &sessionFile{ID: "20260804-9", Started: "2026", Messages: []provider.Message{{Role: "user", Content: "hi"}}}
	if err := saveSession(seed); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/quit\n"))
	if err := repl(&out, &errOut, in, "", seed.ID); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "resumed session 20260804-9") {
		t.Fatalf("resume banner missing: %q", out.String())
	}
	entries, err := os.ReadDir(filepath.Join(".fender", "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 { // seed + new session written on /quit
		t.Fatalf("expected a new session saved after /quit, got %d entries", len(entries))
	}
}

func TestReplDefaultsToFreshSession(t *testing.T) {
	chdir(t, t.TempDir())
	seed := &sessionFile{ID: "20260804-9", Started: "2026", Messages: []provider.Message{{Role: "user", Content: "hi"}}}
	if err := saveSession(seed); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/quit\n"))
	if err := repl(&out, &errOut, in, "", ""); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "resumed session") {
		t.Fatalf("default session unexpectedly resumed: %q", out.String())
	}
}
