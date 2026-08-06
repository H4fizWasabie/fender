package main

import (
	"fmt"
	"strings"

	"github.com/H4fizWasabie/fender/internal/agent"
)

// skillTask parses "/<skill> <task...>" and composes a user message that
// carries the skill body inline (D59). Returns (message, true) when the
// text was a skill invocation, (text, false) otherwise.
func skillTask(a *agent.Agent, text string) (string, bool, error) {
	if !strings.HasPrefix(text, "/") || a == nil || a.Skills == nil {
		return text, false, nil
	}
	fields := strings.Fields(text)
	name := strings.TrimPrefix(fields[0], "/")
	if name == "" {
		return text, false, nil
	}
	// known REPL commands are not skills
	switch name {
	case "quit", "help", "model", "mode", "thinking", "compact", "skills":
		return text, false, nil
	}
	s, ok := a.Skills.ByName(name)
	if !ok {
		return text, true, fmt.Errorf("unknown skill %q (see /skills)", name) // isSkill=true: surfaces must error, not run it as a task
	}
	task := strings.TrimSpace(strings.TrimPrefix(text, fields[0]))
	msg := fmt.Sprintf("[skill: %s]\n%s\n\n%s", name, s.Body, task)
	return msg, true, nil
}
