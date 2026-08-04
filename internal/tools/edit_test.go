package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditFile(t *testing.T) {
	proj := t.TempDir()
	path := filepath.Join(proj, "a.txt")
	os.WriteFile(path, []byte("foo bar baz\n"), 0o644)
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "edit_file", `{"path":"a.txt","old_text":"bar","new_text":"BAR"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "edited") {
		t.Fatalf("out = %q", out)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "foo BAR baz\n" {
		t.Fatalf("file = %q", data)
	}
}

func TestEditFileErrors(t *testing.T) {
	proj := t.TempDir()
	os.WriteFile(filepath.Join(proj, "a.txt"), []byte("x x\n"), 0o644)
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	if _, err := reg.Execute(context.Background(), "edit_file", `{"path":"a.txt","old_text":"zz","new_text":"y"}`); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want not-found error, got %v", err)
	}
	if _, err := reg.Execute(context.Background(), "edit_file", `{"path":"a.txt","old_text":"x","new_text":"y"}`); err == nil || !strings.Contains(err.Error(), "times") {
		t.Fatalf("want ambiguity error, got %v", err)
	}
	if _, err := reg.Execute(context.Background(), "edit_file", `{"path":"../x","old_text":"a","new_text":"b"}`); err == nil {
		t.Fatal("want containment error")
	}
}
