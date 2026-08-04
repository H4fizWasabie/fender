// Package context implements D31: mino's artifact engineering for the
// conversation/tool loop — the 8K inline rule, HEAD/TAIL, write-elsewhere,
// isolate (D38). Budget arithmetic in For() keeps every turn within
// ContextChars. Spec: docs/superpowers/specs/2026-08-04-fender-context-design.md.
package context

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// InlineLimit is the 8K rule (D31): tool output above this becomes an
	// artifact pointer, never inline.
	InlineLimit = 8000
	// PreviewLimit is the HEAD/TAIL budget for compacted user input.
	PreviewLimit = 8000
	// DefaultChars is the per-turn context budget (mino parity).
	DefaultChars = 100_000
	// DefaultTurns is the history depth kept by For (0 -> DefaultTurns).
	DefaultTurns = 5
	// SweepAge is the artifact retention window (D31 isolate sweep).
	SweepAge = 24 * time.Hour

	catalogCap = 2000
)

// Artifact is one compacted blob: full content lives at Path, Size chars.
type Artifact struct {
	Label string // tool name or "user input"
	Path  string
	Size  int
}

// Manager is the artifact layer for one agent run. Per-agent instances
// (D38): a child agent gets its own Manager via Child() — no shared mutable
// state across goroutines.
type Manager struct {
	Root            string // artifact root (default /tmp/fender/artifacts/<runID>)
	ContextChars    int    // 0 -> DefaultChars
	MaxHistoryTurns int    // 0 -> DefaultTurns
	runID           string
	turn            int // internal counter -> <Root>/<n>/<tool>.txt
	catalog         []Artifact
}

// New returns a Manager rooted at /tmp/fender/artifacts/<random hex>.
func New() *Manager {
	m := &Manager{ContextChars: DefaultChars, MaxHistoryTurns: DefaultTurns, runID: randomID()}
	m.Root = filepath.Join("/tmp/fender/artifacts", m.runID)
	return m
}

// Child clones the settings with a fresh run dir and catalog (D38: subagent
// isolation — the child's artifacts never mix with the parent's).
func (m *Manager) Child() *Manager {
	c := *m
	c.runID = randomID()
	c.turn = 0
	c.catalog = nil
	c.Root = filepath.Join(filepath.Dir(m.Root), c.runID)
	return &c
}

// randomID returns 16 hex chars; on entropy failure it falls back to a
// nanosecond timestamp (collision needs the same nanosecond — acceptable).
func randomID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// CompactOutput applies the 8K rule (D31): read_file is never compacted —
// its result is the explicit slice the model asked for. Everything else
// over InlineLimit is written to <Root>/<n>/<tool>.txt (0700 dir, 0600
// file) and replaced by a one-line pointer; the artifact is recorded in
// the catalog. A write failure keeps the first InlineLimit chars inline
// with a marker — never silent truncation.
func (m *Manager) CompactOutput(tool, output string) string {
	if tool == "read_file" || len(output) <= InlineLimit {
		return output
	}
	m.turn++
	dir := filepath.Join(m.Root, fmt.Sprintf("%d", m.turn))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return output[:InlineLimit] + "\n[artifact write failed]"
	}
	path := filepath.Join(dir, safeName(tool)+".txt")
	if err := os.WriteFile(path, []byte(output), 0o600); err != nil {
		return output[:InlineLimit] + "\n[artifact write failed]"
	}
	m.record(Artifact{Label: tool, Path: path, Size: len(output)})
	return fmt.Sprintf("[artifact: %s → %d chars at %s; use read_file with offset and limit]", tool, len(output), path)
}

// CompactInput preserves the head + tail of a large user message inline and
// writes the full content elsewhere (D31: preserve head+tail, write
// elsewhere). Returns the compacted text and the recorded artifact (zero
// Artifact when the input stays inline). preview is the HEAD/TAIL budget —
// For() derives it from the available context budget (mino ContextFor).
func (m *Manager) CompactInput(input string, preview int) (string, Artifact) {
	if len(input) <= preview || preview <= 0 {
		return input, Artifact{}
	}
	dir := filepath.Join(m.Root, fmt.Sprintf("input-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return input[:preview], Artifact{}
	}
	path := filepath.Join(dir, "user.txt")
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		return input[:preview], Artifact{}
	}
	head := preview / 2
	tail := preview - head
	art := Artifact{Label: "user input", Path: path, Size: len(input)}
	m.record(art)
	return fmt.Sprintf("[large user input: %d chars at %s; use read_file with offset and limit]\nHEAD:\n%s\n...\nTAIL:\n%s",
		len(input), path, input[:head], input[len(input)-tail:]), art
}

// safeName keeps a label filesystem-safe (mino safePath).
func safeName(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || r == '_' || r == '.' ||
			r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, s)
}

func (m *Manager) record(a Artifact) {
	for _, have := range m.catalog {
		if have.Path == a.Path {
			return
		}
	}
	m.catalog = append(m.catalog, a)
}

// Catalog renders live artifacts, capped at catalogCap chars. The snapshot
// rides in context via For() so the model knows what it can fetch (D31:
// select).
func (m *Manager) Catalog() string {
	if len(m.catalog) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString("Live session artifacts (use read_file(path, offset, limit) when needed):\n")
	for _, a := range m.catalog {
		line := fmt.Sprintf("- %s: %d chars at %s\n", a.Label, a.Size, a.Path)
		if out.Len()+len(line) > catalogCap {
			break
		}
		out.WriteString(line)
	}
	return out.String()
}

// Cleanup removes artifact runs older than maxAge (D31 isolate sweep).
// Sweeps the whole artifacts base dir, not just this run — stale siblings
// from crashed sessions go too.
func (m *Manager) Cleanup(maxAge time.Duration) {
	root := filepath.Dir(m.Root)
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		info, err := e.Info()
		if err == nil && time.Since(info.ModTime()) > maxAge {
			os.RemoveAll(filepath.Join(root, e.Name()))
		}
	}
}

func (m *Manager) chars() int {
	if m.ContextChars > 0 {
		return m.ContextChars
	}
	return DefaultChars
}

func (m *Manager) turns() int {
	if m.MaxHistoryTurns > 0 {
		return m.MaxHistoryTurns
	}
	return DefaultTurns
}
