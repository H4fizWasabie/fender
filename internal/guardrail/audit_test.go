package guardrail

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAuditLogsJSONLines(t *testing.T) {
	var buf bytes.Buffer
	a := NewAudit(&buf)
	a.Log("rm -rf /", Refuse)
	a.Log("ls", Run)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d: %q", len(lines), buf.String())
	}
	var entry struct {
		Time    time.Time `json:"time"`
		Command string    `json:"command"`
		Verdict string    `json:"verdict"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Command != "rm -rf /" || entry.Verdict != "REFUSE" {
		t.Fatalf("entry = %+v", entry)
	}
	if entry.Time.IsZero() {
		t.Fatal("missing timestamp (D24)")
	}
}

func TestAuditNilSafe(t *testing.T) {
	var a *Audit
	a.Log("x", Run) // must not panic
}

func TestDefaultTimeout(t *testing.T) {
	if DefaultTimeout != 60*time.Second {
		t.Fatalf("DefaultTimeout = %v", DefaultTimeout)
	}
}
