package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestReadPrependsNestedRules(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "internal", "parser")
	os.MkdirAll(sub, 0700)
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("root rules"), 0600)
	os.WriteFile(filepath.Join(sub, "AGENTS.md"), []byte("parser rules"), 0600)
	target := filepath.Join(sub, "a.go")
	os.WriteFile(target, []byte("package parser\n"), 0600)

	loader := func(dir string) string {
		if strings.HasSuffix(dir, "internal/parser") {
			return "\n<<AGENTS.md (nested)>>\nparser rules\n"
		}
		return ""
	}
	tool := readTool(root, loader)
	out, err := tool.Call(context.Background(), map[string]any{"path": target})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "parser rules") {
		t.Fatalf("nested rules missing: %q", out)
	}
	if !strings.Contains(out, "package parser") {
		t.Fatalf("file content missing: %q", out)
	}
}

func TestReadNoLoaderUnchanged(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "a.go")
	os.WriteFile(target, []byte("package x\n"), 0600)
	tool := readTool(root)
	out, err := tool.Call(context.Background(), map[string]any{"path": target})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "AGENTS.md") {
		t.Fatalf("rules leaked without loader: %q", out)
	}
}
