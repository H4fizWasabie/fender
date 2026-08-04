package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/guardrail"
	"github.com/H4fizWasabie/fender/internal/memory"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/tools"
)

func newTestRegistry(t *testing.T) *tools.Registry {
	t.Helper()
	proj := t.TempDir()
	return tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
}

func TestMemPrependsSystem(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("CONSTITUTION-RULES"), 0600)
	mem := memory.New(root)

	fake := &fakeLLM{steps: []*provider.Response{completeReply("complete", "done")}}
	a := NewAgent(fake, newTestRegistry(t))
	a.System = "TASK-SPECIFIC"
	a.Mem = mem

	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "go"}})
	if res == nil || res.Status != "complete" {
		t.Fatalf("result = %+v", res)
	}
	req := fake.last()
	if len(req.Messages) == 0 || req.Messages[0].Role != "system" {
		t.Fatalf("no system message first: %+v", req.Messages)
	}
	sys := req.Messages[0].Content
	ci := strings.Index(sys, "CONSTITUTION-RULES")
	ti := strings.Index(sys, "TASK-SPECIFIC")
	if ci < 0 || ti < 0 || ci > ti {
		t.Fatalf("memory must prepend before task-specific: %q", sys)
	}
	if !strings.Contains(sys, "<<AGENTS.md (project):") {
		t.Fatalf("missing provenance marker: %q", sys)
	}
}

func TestMemNilUnchanged(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{completeReply("complete", "done")}}
	a := NewAgent(fake, newTestRegistry(t))
	a.System = "ONLY-THIS"
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "go"}})
	req := fake.last()
	if len(req.Messages) == 0 || req.Messages[0].Content != "ONLY-THIS" {
		t.Fatalf("nil Mem changed behavior: %q", req.Messages)
	}
}
