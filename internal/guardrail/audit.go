package guardrail

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// DefaultTimeout is the harness-level command timeout (D24), applied by
// the shell tool (Plan 3) to every command.
const DefaultTimeout = 60 * time.Second

// Audit records every judged command (D24): command, verdict, timestamp.
type Audit struct {
	mu sync.Mutex
	w  io.Writer
}

// NewAudit writes JSON lines to w (the loop opens ~/.fender/audit.log).
func NewAudit(w io.Writer) *Audit {
	return &Audit{w: w}
}

type auditEntry struct {
	Time    time.Time `json:"time"`
	Command string    `json:"command"`
	Verdict string    `json:"verdict"`
}

// Log appends one entry. Errors are dropped: auditing must never block
// or fail the loop.
func (a *Audit) Log(cmd string, v Verdict) {
	if a == nil || a.w == nil {
		return
	}
	entry, err := json.Marshal(auditEntry{Time: time.Now(), Command: cmd, Verdict: v.String()})
	if err != nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = a.w.Write(append(entry, '\n'))
}
