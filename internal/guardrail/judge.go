package guardrail

import (
	"fmt"
	"strings"
)

// Judge parses cmd and returns the verdict for the given mode.
// projectDir anchors relative paths for path-escape and destructive checks.
// Unparseable or empty commands are always REFUSE: the guardrail cannot
// judge what it cannot parse, and the guardrail never guesses (D22).
func Judge(cmd string, mode Mode, projectDir string) (Verdict, string) {
	if mode == "" {
		mode = Balanced
	}
	if strings.TrimSpace(cmd) == "" {
		return Refuse, "empty command"
	}
	file, err := parseCmd(cmd)
	if err != nil {
		return Refuse, "unparseable shell: " + err.Error()
	}
	findings := detect(file, projectDir)
	v := Run
	details := make([]string, 0, len(findings))
	for _, f := range findings {
		if fv := verdictFor(f.cat, f.severe); fv > v {
			v = fv
		}
		details = append(details, fmt.Sprintf("%s[%s]", f.cat, f.detail))
	}
	switch mode {
	case Strict:
		if v != Refuse {
			v = Ask // D21: strict asks for every tool call
		}
	case Yolo:
		if v == Ask {
			v = Run // D21: yolo removes questions, never the guardrail
		}
	}
	if len(details) == 0 {
		return v, ""
	}
	return v, strings.Join(details, ", ")
}
