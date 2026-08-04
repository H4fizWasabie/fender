// Package guardrail is the harness-level safety layer (D11, D12, D21-24):
// permission modes, sh-parser verdicts, command timeout, audit log.
package guardrail

import "fmt"

// Mode is the permission mode from fender.toml (D21).
type Mode string

const (
	Strict   Mode = "strict"   // ASK for every tool call
	Balanced Mode = "balanced" // verdict table
	Yolo     Mode = "yolo"     // ASK becomes RUN; REFUSE stays hard
)

// ParseMode converts a fender.toml mode string to a Mode.
func ParseMode(s string) (Mode, error) {
	switch Mode(s) {
	case Strict, Balanced, Yolo:
		return Mode(s), nil
	}
	return "", fmt.Errorf("unknown permission mode %q (want strict, balanced, or yolo)", s)
}

// Verdict is the guardrail's decision for a command (D21).
type Verdict int

const (
	Run    Verdict = iota // execute without asking
	Ask                   // prompt the user
	Refuse                // never execute, in any mode (D22)
)

func (v Verdict) String() string {
	switch v {
	case Run:
		return "RUN"
	case Ask:
		return "ASK"
	default:
		return "REFUSE"
	}
}

// Category is a judged command class (D23).
type Category string

const (
	CatDestructiveFS   Category = "destructive_fs"
	CatPrivilege       Category = "privilege"
	CatGitIrreversible Category = "git_irreversible"
	CatPipeToShell     Category = "pipe_to_shell"
	CatRunaway         Category = "runaway"
	CatTTYHanger       Category = "tty_hanger"
	CatProtectedPath   Category = "protected_path"
	CatPathEscape      Category = "path_escape"
)

// verdictFor is the balanced-mode table: category x severity -> verdict.
// strict and yolo are derived in Judge (strict: ASK all non-REFUSE;
// yolo: ASK becomes RUN).
func verdictFor(c Category, severe bool) Verdict {
	switch c {
	case CatDestructiveFS:
		if severe {
			return Refuse // system root / project ancestor: hard
		}
		return Run
	case CatPrivilege:
		if severe {
			return Refuse // mkfs, shutdown class: hard (D22)
		}
		return Ask
	case CatPipeToShell, CatRunaway:
		if severe {
			return Refuse // curl|sh, fork bomb: hard (D22)
		}
		return Ask
	case CatGitIrreversible, CatTTYHanger, CatProtectedPath, CatPathEscape:
		return Ask
	}
	return Run
}
