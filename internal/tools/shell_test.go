package tools

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/H4fizWasabie/fender/internal/guardrail"
)

func TestShellRun(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "shell", `{"command":"echo hi"}`)
	if err != nil || out != "hi\n" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestShellRefuseNeverRuns(t *testing.T) {
	proj := t.TempDir()
	called := false
	reg := New(proj, ShellConfig{
		Mode:       guardrail.Yolo, // REFUSE is hard in all modes (D22)
		ProjectDir: proj,
		Approver: func(_ context.Context, cmd, reason string) (bool, error) {
			called = true
			return true, nil
		},
	}, nil)
	_, err := reg.Execute(context.Background(), "shell", `{"command":"rm -rf /"}`)
	if err == nil || !strings.Contains(err.Error(), "REFUSED") {
		t.Fatalf("want REFUSED, got %v", err)
	}
	if called {
		t.Fatal("approver must not be consulted for REFUSE")
	}
}

func TestShellAskApproved(t *testing.T) {
	proj := t.TempDir()
	approved := false
	reg := New(proj, ShellConfig{
		Mode:       guardrail.Balanced,
		ProjectDir: proj,
		Approver: func(_ context.Context, cmd, reason string) (bool, error) {
			approved = true
			return true, nil
		},
	}, nil)
	if _, err := reg.Execute(context.Background(), "shell", `{"command":"tee /tmp/fender-test-ask"}`); err != nil {
		t.Fatalf("approved ASK should run: %v", err)
	}
	if !approved {
		t.Fatal("approver not called")
	}
}

func TestShellAskDenied(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{
		Mode:       guardrail.Balanced,
		ProjectDir: proj,
		Approver:   func(context.Context, string, string) (bool, error) { return false, nil },
	}, nil)
	_, err := reg.Execute(context.Background(), "shell", `{"command":"tee /tmp/fender-test-deny"}`)
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("want denied, got %v", err)
	}
}

func TestShellAskNoApproverDenies(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	_, err := reg.Execute(context.Background(), "shell", `{"command":"tee /tmp/fender-test-noappr"}`)
	if err == nil || !strings.Contains(err.Error(), "approval") {
		t.Fatalf("want approval error, got %v", err)
	}
}

func TestShellTimeout(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj, Timeout: 100 * time.Millisecond}, nil)
	_, err := reg.Execute(context.Background(), "shell", `{"command":"sleep 5"}`)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("want timeout, got %v", err)
	}
}

func TestShellAuditsEveryCommand(t *testing.T) {
	proj := t.TempDir()
	var buf bytes.Buffer
	reg := New(proj, ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj, Audit: guardrail.NewAudit(&buf)}, nil)
	if _, err := reg.Execute(context.Background(), "shell", `{"command":"echo hi"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Execute(context.Background(), "shell", `{"command":"rm -rf /"}`); err == nil {
		t.Fatal("expected REFUSE")
	}
	if got := strings.Count(buf.String(), "\n"); got != 2 {
		t.Fatalf("audit lines = %d: %q", got, buf.String())
	}
}

func TestShellWorksInProjectDir(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "shell", `{"command":"pwd"}`)
	if err != nil || strings.TrimSpace(out) != proj {
		t.Fatalf("out=%q err=%v (want cwd=%q)", out, err, proj)
	}
}
