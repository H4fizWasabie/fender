package skills

import (
	"strings"
	"testing"
)

func mkSkill(name, desc string, invokable bool) Skill {
	return Skill{Name: name, Description: desc, Body: "body of " + name, ModelInvokable: invokable}
}

func TestMatchFindsDiagnosingBugs(t *testing.T) {
	reg := &Registry{all: map[string]Skill{
		"diagnosing-bugs": mkSkill("diagnosing-bugs", "Diagnosis loop for hard bugs. Use when something is broken or failing.", true),
		"tdd":             mkSkill("tdd", "Test-driven development, red-green-refactor.", true),
	}}
	got := reg.Match("can you diagnose this bug, something keeps failing")
	if len(got) != 1 || got[0].Name != "diagnosing-bugs" {
		t.Fatalf("got = %+v", got)
	}
}

func TestMatchSkipsUserInvoked(t *testing.T) {
	reg := &Registry{all: map[string]Skill{
		"ask-matt": mkSkill("ask-matt", "Ask which skill fits. Use when the user wants a router.", false),
	}}
	if got := reg.Match("ask which skill fits"); len(got) != 0 {
		t.Fatalf("user-invoked skill must not auto-match: %+v", got)
	}
}

func TestMatchTopNBudget(t *testing.T) {
	reg := &Registry{all: map[string]Skill{
		"a": mkSkill("a", strings.Repeat("alpha beta gamma delta epsilon zeta eta theta ", 50), true),
		"b": mkSkill("b", "alpha beta gamma delta epsilon zeta eta theta", true),
		"c": mkSkill("c", "alpha beta gamma delta epsilon zeta eta theta", true),
		"d": mkSkill("d", "alpha beta gamma delta epsilon zeta eta theta", true),
	}}
	got := reg.Match("alpha beta gamma delta epsilon zeta eta theta")
	if len(got) > MatchTopN {
		t.Fatalf("matched %d > top %d", len(got), MatchTopN)
	}
	total := 0
	for _, s := range got {
		total += len(s.Body)
	}
	if total > BodyBudget {
		t.Fatalf("body budget exceeded: %d", total)
	}
}

func TestMatchNoHit(t *testing.T) {
	reg := &Registry{all: map[string]Skill{
		"tdd": mkSkill("tdd", "Test-driven development, red-green-refactor.", true),
	}}
	if got := reg.Match("what is the weather in kuala lumpur"); len(got) != 0 {
		t.Fatalf("got = %+v", got)
	}
}
