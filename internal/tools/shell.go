package tools

import (
	"time"

	"github.com/H4fizWasabie/fender/internal/guardrail"
)

// ShellConfig is the guardrail wiring for the shell tool. Guardrails wrap
// tool execution once (D13) — every agent passes through the same config.
// shellTool (Task 3) consumes this.
type ShellConfig struct {
	Mode       guardrail.Mode
	ProjectDir string
	Audit      *guardrail.Audit                       // nil = no audit
	Timeout    time.Duration                          // 0 → guardrail.DefaultTimeout
	Approver   func(cmd, reason string) (bool, error) // nil → ASK is denied
}
