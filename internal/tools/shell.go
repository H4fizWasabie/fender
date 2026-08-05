package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/H4fizWasabie/fender/internal/guardrail"
)

// outputCap bounds command output held in memory (8 MiB). Full output now
// reaches the model via the artifact layer (D31/D38): anything over
// InlineLimit becomes a pointer. This cap is a memory ceiling, not a
// content limit — ticket-03's 64 KiB truncation was lossy and is gone.
const outputCap = 8 << 20 // 8 MiB

// ShellConfig is the guardrail wiring for the shell tool. Guardrails wrap
// tool execution once (D13) — every agent passes through the same config.
type ShellConfig struct {
	Mode       guardrail.Mode
	ProjectDir string
	Audit      *guardrail.Audit                                    // nil = no audit
	Timeout    time.Duration                                       // 0 → guardrail.DefaultTimeout
	Approver   func(context.Context, string, string) (bool, error) // nil → ASK is denied
}

func shellTool(cfg ShellConfig) Tool {
	return Tool{
		Name:        "shell",
		Description: "Run a shell command with bash -c inside the project directory. Commands are judged by the guardrail (destructive fs, privilege, git, secrets, escapes); refused commands never run, others may require approval.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string"},
			},
			"required": []string{"command"},
		},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			cmd, _ := args["command"].(string)
			if strings.TrimSpace(cmd) == "" {
				return "", fmt.Errorf("shell: empty command")
			}
			verdict, reason := guardrail.Judge(cmd, cfg.Mode, cfg.ProjectDir)
			if cfg.Audit != nil {
				cfg.Audit.Log(cmd, verdict)
			}
			switch verdict {
			case guardrail.Refuse:
				return "", fmt.Errorf("shell: REFUSED (%s)", reason)
			case guardrail.Ask:
				if cfg.Approver == nil {
					return "", fmt.Errorf("shell: requires approval (%s); no approver configured", reason)
				}
				ok, err := cfg.Approver(ctx, cmd, reason)
				if err != nil {
					return "", fmt.Errorf("shell: approval error: %v", err)
				}
				if !ok {
					return "", fmt.Errorf("shell: denied by user (%s)", reason)
				}
			}
			timeout := cfg.Timeout
			if timeout == 0 {
				timeout = guardrail.DefaultTimeout
			}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			execCmd := exec.CommandContext(ctx, "bash", "-c", cmd)
			execCmd.Dir = cfg.ProjectDir
			out, err := execCmd.CombinedOutput()
			if ctx.Err() == context.DeadlineExceeded {
				return "", fmt.Errorf("shell: timed out after %s: %s", timeout, capOutput(out))
			}
			if err != nil {
				return "", fmt.Errorf("shell: %v: %s", err, capOutput(out))
			}
			return capOutput(out), nil
		},
	}
}

func capOutput(b []byte) string {
	if len(b) <= outputCap {
		return string(b)
	}
	return fmt.Sprintf("%s\n... (output truncated: %d of %d bytes)\n", string(b[:outputCap]), outputCap, len(b))
}
