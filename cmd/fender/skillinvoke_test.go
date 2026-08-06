package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/skills"
)

func testSkillAgent(t *testing.T) *agent.Agent {
	t.Helper()
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "myskill")
	os.MkdirAll(skillDir, 0700)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: myskill\ndescription: Test skill.\n---\nSKILL BODY HERE\n"), 0600)
	reg, err := skills.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &agent.Agent{Skills: reg}
	return a
}

func TestSkillTaskComposes(t *testing.T) {
	a := testSkillAgent(t)
	msg, isSkill, err := skillTask(a, "/myskill fix the parser")
	if err != nil || !isSkill {
		t.Fatalf("isSkill=%v err=%v", isSkill, err)
	}
	if !strings.Contains(msg, "SKILL BODY HERE") || !strings.Contains(msg, "fix the parser") {
		t.Fatalf("message = %q", msg)
	}
}

func TestSkillTaskUnknownSkill(t *testing.T) {
	a := testSkillAgent(t)
	_, isSkill, err := skillTask(a, "/nope do something")
	if !isSkill {
		t.Fatal("unknown skill must still be flagged as a skill attempt")
	}
	if err == nil || !strings.Contains(err.Error(), "unknown skill") {
		t.Fatalf("err = %v", err)
	}
}

func TestSkillTaskPlainTextUntouched(t *testing.T) {
	a := testSkillAgent(t)
	msg, isSkill, err := skillTask(a, "just a normal task")
	if err != nil || isSkill || msg != "just a normal task" {
		t.Fatalf("msg=%q isSkill=%v err=%v", msg, isSkill, err)
	}
}

func TestSkillTaskBuiltinSlashUntouched(t *testing.T) {
	a := testSkillAgent(t)
	_, isSkill, err := skillTask(a, "/help")
	if err != nil || isSkill {
		t.Fatalf("isSkill=%v err=%v", isSkill, err)
	}
}
