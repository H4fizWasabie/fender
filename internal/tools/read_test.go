package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFile(t *testing.T) {
	proj := t.TempDir()
	os.WriteFile(filepath.Join(proj, "a.txt"), []byte("line1\nline2\nline3\n"), 0o644)
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "read_file", `{"path":"a.txt"}`)
	if err != nil || out != "line1\nline2\nline3\n" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	out, err = reg.Execute(context.Background(), "read_file", `{"path":"a.txt","offset":2,"limit":1}`)
	if err != nil || out != "line2" {
		t.Fatalf("slice out=%q err=%v", out, err)
	}
	out, err = reg.Execute(context.Background(), "read_file", `{"path":"a.txt","offset":9}`)
	if err != nil || out != "" {
		t.Fatalf("past-eof out=%q err=%v", out, err)
	}
	if _, err := reg.Execute(context.Background(), "read_file", `{"path":"missing.txt"}`); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadFileContainment(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	for _, path := range []string{"../outside.txt", "/etc/passwd", ""} {
		if _, err := reg.Execute(context.Background(), "read_file", `{"path":"`+path+`"}`); err == nil {
			t.Fatalf("expected containment error for %q", path)
		}
	}
}
