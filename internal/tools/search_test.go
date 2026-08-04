package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearch(t *testing.T) {
	proj := t.TempDir()
	os.WriteFile(filepath.Join(proj, "a.go"), []byte("func Foo() {}\n// Foo is here\n"), 0o644)
	os.WriteFile(filepath.Join(proj, "b.txt"), []byte("no match here\n"), 0o644)
	os.MkdirAll(filepath.Join(proj, ".git"), 0o755)
	os.WriteFile(filepath.Join(proj, ".git", "config"), []byte("Foo\n"), 0o644)
	os.WriteFile(filepath.Join(proj, "bin.dat"), []byte("\x00needle\n"), 0o644)
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "search", `{"query":"foo"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a.go:1: func Foo() {}") || !strings.Contains(out, "a.go:2: // Foo is here") {
		t.Fatalf("out = %q", out)
	}
	if strings.Contains(out, ".git") {
		t.Fatalf(".git not skipped: %q", out)
	}
	if strings.Contains(out, "bin.dat") {
		t.Fatalf("binary not skipped: %q", out)
	}
	if strings.Contains(out, "b.txt") {
		t.Fatalf("non-match leaked: %q", out)
	}
}

func TestSearchNoMatches(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "search", `{"query":"zzz_nothing_zzz"}`)
	if err != nil || out != "no matches" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestSearchCap(t *testing.T) {
	proj := t.TempDir()
	var sb strings.Builder
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&sb, "needle line %d\n", i)
	}
	os.WriteFile(filepath.Join(proj, "big.txt"), []byte(sb.String()), 0o644)
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "search", `{"query":"needle"}`)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, "needle"); n != maxSearchResults {
		t.Fatalf("matches = %d (want %d)", n, maxSearchResults)
	}
}
