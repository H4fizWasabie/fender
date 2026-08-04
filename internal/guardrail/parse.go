package guardrail

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

func parseCmd(cmd string) (*syntax.File, error) {
	return syntax.NewParser().Parse(strings.NewReader(cmd), "cmd")
}

// wordLiteral reconstructs a word's literal text when every part is literal
// (plain text or quotes). known=false when the word contains expansions
// ($HOME, $(...), globs) — expanded words are not judged by pattern.
// ponytail: expansion-aware judgment is a later pass; v1 covers literal paths.
func wordLiteral(w *syntax.Word) (string, bool) {
	if w == nil {
		return "", false
	}
	var sb strings.Builder
	for _, p := range w.Parts {
		switch p := p.(type) {
		case *syntax.Lit:
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			sb.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, ip := range p.Parts {
				switch ip := ip.(type) {
				case *syntax.Lit:
					sb.WriteString(ip.Value)
				case *syntax.SglQuoted:
					sb.WriteString(ip.Value)
				default:
					return "", false
				}
			}
		default:
			return "", false
		}
	}
	return sb.String(), true
}

// callName returns the literal command name of a call, if fully literal.
func callName(c *syntax.CallExpr) (string, bool) {
	if c == nil || len(c.Args) == 0 {
		return "", false
	}
	return wordLiteral(c.Args[0])
}

// literalArgs returns the literal arguments, excluding the command name.
// Non-literal words (expansions) are skipped entirely.
func literalArgs(c *syntax.CallExpr) []string {
	var out []string
	for _, w := range c.Args[1:] {
		if s, ok := wordLiteral(w); ok {
			out = append(out, s)
		}
	}
	return out
}

// nonFlagArgs drops leading "-" options and everything after "--".
func nonFlagArgs(args []string) []string {
	var out []string
	done := false
	for _, a := range args {
		if done {
			out = append(out, a)
			continue
		}
		if a == "--" {
			done = true
			continue
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			continue
		}
		out = append(out, a)
	}
	return out
}
