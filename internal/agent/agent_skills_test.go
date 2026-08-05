package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/skills"
)

func bundledSkills(t *testing.T) *skills.Registry {
	t.Helper()
	reg, err := skills.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestSkillsComposeSystem(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{completeReply("complete", "done")}}
	a := NewAgent(fake, newTestRegistry(t))
	a.System = "TASK"
	a.Skills = bundledSkills(t)

	// "build features test-first and write integration tests" must match tdd
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "build features test-first and write integration tests"}})

	req := fake.last()
	sys := req.Messages[0].Content
	if !strings.Contains(sys, "laziest") {
		t.Fatalf("ponytail core missing: %q", sys[:200])
	}
	if !strings.Contains(sys, "- tdd:") {
		t.Fatalf("descriptions catalog missing: %q", sys[:400])
	}
	// D56 (cache-correct): matched skill bodies ride in an APPENDED message
	// before the user turn — never in the system prompt.
	var all string
	for _, m := range req.Messages {
		all += m.Content
	}
	if !strings.Contains(all, "# Test-Driven Development") {
		t.Fatalf("matched skill body missing: %q", all[:400])
	}
	if strings.Contains(sys, "# Test-Driven Development") {
		t.Fatal("skill body leaked into the system prompt (cache hazard)")
	}
	// ponytail is always-loaded, never a matched marker
	if strings.Contains(sys, "[skill loaded: ponytail]") {
		t.Fatal("ponytail must not be trigger-matched")
	}
}

func TestSkillsNilUnchanged(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{completeReply("complete", "done")}}
	a := NewAgent(fake, newTestRegistry(t))
	a.System = "ONLY-THIS"
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "go"}})
	if got := fake.last().Messages[0].Content; got != "ONLY-THIS" {
		t.Fatalf("nil Skills changed behavior: %q", got)
	}
}

func TestLoadSkillTool(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{
		toolReply("c1", "load_skill", `{"name":"tdd"}`),
		completeReply("complete", "done"),
	}}
	a := NewAgent(fake, newTestRegistry(t))
	a.Skills = bundledSkills(t)
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "load tdd"}})
	if res == nil || res.Status != "complete" {
		t.Fatalf("result = %+v", res)
	}
	reqs := fake.all()
	last := reqs[len(reqs)-1]
	joined := ""
	for _, m := range last.Messages {
		joined += m.Content
	}
	if !strings.Contains(joined, "# Test-Driven Development") {
		t.Fatalf("load_skill body not returned: %q", joined[:400])
	}
}
