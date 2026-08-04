# Fender Plan 4: Context Artifact Engineering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** D31 in Go — `internal/context`, mino's artifact engineering ported: the 8K inline rule (tool output > 8000 chars → one-line artifact pointer, full content written elsewhere, 0600), HEAD/TAIL for large user input, `read_file` never compacted, per-run isolation with a 24h sweep, budget arithmetic (`For`) that bounds every turn, artifact catalog riding in context — wired into the ONE agent loop.

**Architecture:** `internal/context` is a plain struct package (no interfaces — one implementation, ponytail rule): `Manager{Root, ContextChars, MaxHistoryTurns}` with `CompactOutput` (compress), `CompactInput` (preserve head+tail), `For` (budget arithmetic + turns truncation + catalog), `Cleanup` (isolate sweep), `Child` (per-subagent isolation, D38). `internal/agent` consumes it nil-safely: `Agent.Ctx` (nil = ticket-03 behavior, existing tests untouched), `Run` compacts the incoming task + truncates history via `For` at ingress and compacts every tool result except `read_file` before it enters history; the dedup cache stores the pointer. The `delegate` tool gives children `Ctx.Child()`. Tool caps adjust: shell 64 KiB → 8 MiB (memory ceiling — artifacts carry full output; 64 KiB was lossy), read 1 MiB stays (never-compacted tool needs an inline ceiling).

**Tech Stack:** Go 1.22, stdlib only (`crypto/rand`, `os`, `path/filepath`, `strings`, `time`). Imports `internal/provider` for `Message` (already exists). Zero new dependencies. Reference (study, not vendor): `~/Desktop/mino/artifacts.go`, `~/Desktop/mino/session.go` (`ContextMessages`/`ContextFor`), `~/Desktop/mino/context_test.go` — the test blueprint D31 names.

## Global Constraints

- **Read `AGENTS.md`, `DECISIONS.md` (D31, D37, D38), and the design spec `docs/superpowers/specs/2026-08-04-fender-context-design.md` first.** They are the constitution.
- **Every commit MUST stage `CHANGELOG.md`** with a `[Unreleased]` entry — enforced by `.githooks/pre-commit`.
- **Allowed dependencies:** `BurntSushi/toml`, `mvdan.cc/sh/v3`, `go-tree-sitter`, `modernc.org/sqlite`. Nothing else. This plan adds zero dependencies.
- **No panic in library code.** Explicit errors, `log/slog` for logging.
- **No new abstractions with one implementation.** `Manager` is a struct; nil-safe `Agent.Ctx` keeps the ticket-03 loop semantics when unset.
- Module path: `github.com/H4fizWasabie/fender`. New files: `internal/context/context.go`, `internal/context/for.go` + tests. Flat over nested.
- **`internal/agent` imports the stdlib `context` package already — alias the new package `ctxpkg "github.com/H4fizWasabie/fender/internal/context"` in agent.go, agent_test.go, delegate_test.go.**
- **Acceptance #3 is load-bearing** (user requirement): the budget-bound test (`TestForBudgetBound`, Task 3) asserts `len(system) + Σ message content ≤ ContextChars`. It is a first-class task, not an afterthought.
- **Existing ticket-03 tests must pass unchanged** — nil `Ctx` everywhere except the new integration tests.
- Artifact root is `/tmp/fender/artifacts/<runID>` (D38) — never the project tree. Tests override `Root` with `t.TempDir()`.

**Known v1 limitations (documented, not fixed):**
- The budget bound guarantees `system + Σ msgs ≤ ContextChars` as long as `system` + the current user turn fit alone; the drop loop never removes the last pair (the current user turn survives — mino's rule).
- Artifact pointers are plain paths in history; a file swept mid-session makes its pointer stale — the catalog lists what was recorded, and stale reads fail loudly, same as mino.
- `CompactInput` write failure falls back to `input[:preview]` inline (never silent truncation without the `[artifact write failed]` marker).

---

### Task 1: context package core — Manager, CompactOutput (8K rule), Cleanup

**Files:**
- Create: `internal/context/context.go`
- Create: `internal/context/context_test.go`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: nothing (stdlib only).
- Produces (later tasks rely on these exact names):
  - `const InlineLimit = 8000`, `const PreviewLimit = 8000`, `const DefaultChars = 100_000`, `const DefaultTurns = 5`, `const SweepAge = 24 * time.Hour` (unexported: `catalogCap = 2000`)
  - `type Artifact struct { Label string; Path string; Size int }`
  - `type Manager struct { Root string; ContextChars int; MaxHistoryTurns int; runID string; turn int; catalog []Artifact }`
  - `func New() *Manager` — root `/tmp/fender/artifacts/<16-hex>`, `ContextChars = DefaultChars`, `MaxHistoryTurns = DefaultTurns`
  - `func (m *Manager) Child() *Manager` — same settings, fresh runID/turn/catalog, `Root = filepath.Join(filepath.Dir(m.Root), newRunID)`
  - `func (m *Manager) CompactOutput(tool, output string) string` — `read_file` or ≤ `InlineLimit` → passthrough; else write `<Root>/<n>/<tool>.txt` (0700 dir, 0600 file, `n` = internal counter), record catalog, return `[artifact: <tool> → <N> chars at <path>; use read_file with offset and limit]`; write failure → `output[:InlineLimit] + "\n[artifact write failed]"`
  - `func (m *Manager) Catalog() string` — capped at `catalogCap` chars, `""` when empty, header `"Live session artifacts (use read_file(path, offset, limit) when needed):\n"`, lines `- <label>: <size> chars at <path>\n`
  - `func (m *Manager) Cleanup(maxAge time.Duration)` — sweep `filepath.Dir(m.Root)` entries older than `maxAge` (whole artifacts base, like mino)
  - `func (m *Manager) Child()` in Task 1, `CompactInput` in Task 2, `For` in Task 3

- [ ] **Step 1: Study the reference.** Read `~/Desktop/mino/artifacts.go` (whole file, 89 lines) and `~/Desktop/mino/context_test.go` `TestCompactToolOutputWritesArtifact` + `TestPrepareToolOutputKeepsReadSliceInline`. The port is deliberate, not transcription.

- [ ] **Step 2: Write the failing test** — `internal/context/context_test.go`:

```go
package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestManager points the artifact root at a temp dir so tests never
// touch /tmp/fender (and Cleanup never sweeps real runs).
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m := New()
	m.Root = filepath.Join(t.TempDir(), "run")
	return m
}

// artifactPath extracts the path from "[artifact: tool → N chars at PATH; ...]".
func artifactPath(t *testing.T, pointer string) string {
	t.Helper()
	_, after, ok := strings.Cut(pointer, " at ")
	if !ok {
		t.Fatalf("no path in %q", pointer)
	}
	path, _, _ := strings.Cut(after, ";")
	return path
}

func TestCompactOutputUnderLimitInline(t *testing.T) {
	m := newTestManager(t)
	output := strings.Repeat("x", InlineLimit)
	if got := m.CompactOutput("shell", output); got != output {
		t.Fatalf("inline output changed")
	}
}

func TestCompactOutputWritesArtifact(t *testing.T) {
	m := newTestManager(t)
	output := strings.Repeat("x", InlineLimit+1)
	got := m.CompactOutput("shell", output)
	if !strings.Contains(got, "[artifact:") {
		t.Fatalf("no artifact pointer: %.60q", got)
	}
	path := artifactPath(t, got)
	data, err := os.ReadFile(path)
	if err != nil || string(data) != output {
		t.Fatalf("artifact read: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat artifact: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("artifact perms = %o, want 600", info.Mode().Perm())
	}
	if cat := m.Catalog(); !strings.Contains(cat, path) {
		t.Fatalf("catalog missing artifact: %q", cat)
	}
}

func TestCompactOutputKeepsReadInline(t *testing.T) {
	m := newTestManager(t)
	output := strings.Repeat("x", InlineLimit+1)
	if got := m.CompactOutput("read_file", output); got != output {
		t.Fatal("read_file was compacted (D31: the one tool never compacted)")
	}
}

func TestCompactOutputNoOverwrite(t *testing.T) {
	m := newTestManager(t)
	p1 := artifactPath(t, m.CompactOutput("shell", strings.Repeat("a", InlineLimit+1)))
	p2 := artifactPath(t, m.CompactOutput("shell", strings.Repeat("b", InlineLimit+1)))
	if p1 == p2 {
		t.Fatal("repeated tool calls share an artifact path")
	}
	if data, _ := os.ReadFile(p1); string(data) != strings.Repeat("a", InlineLimit+1) {
		t.Fatal("first artifact overwritten")
	}
}

func TestChildIsolation(t *testing.T) {
	m := newTestManager(t)
	m.CompactOutput("shell", strings.Repeat("x", InlineLimit+1))
	c := m.Child()
	if c.Root == m.Root {
		t.Fatalf("child root %q equals parent root", c.Root)
	}
	if c.Catalog() != "" {
		t.Fatal("child catalog must start empty")
	}
	if c.ContextChars != m.ContextChars || c.MaxHistoryTurns != m.MaxHistoryTurns {
		t.Fatal("child settings diverged from parent")
	}
}

func TestCleanupRemovesStale(t *testing.T) {
	// Sweep granularity is RUN dirs (base-dir entries, mino-style): the
	// whole stale run goes, the fresh run stays.
	base := t.TempDir()
	m := New()
	m.Root = filepath.Join(base, "run-fresh")
	if err := os.MkdirAll(m.Root, 0o700); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(base, "run-stale", "1", "old.txt")
	if err := os.MkdirAll(filepath.Dir(stale), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("z"), 0o600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	os.Chtimes(filepath.Dir(filepath.Dir(stale)), past, past)
	m.Cleanup(time.Hour)
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("stale run survived sweep")
	}
	if _, err := os.Stat(m.Root); err != nil {
		t.Fatal("fresh run swept")
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `cd /home/hafiz/Desktop/Fender && go test ./internal/context/`
Expected: FAIL — build error `undefined: New` / `undefined: Manager` (package doesn't exist yet).

- [ ] **Step 4: Write the implementation** — `internal/context/context.go`:

```go
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
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd /home/hafiz/Desktop/Fender && go test ./internal/context/`
Expected: PASS — all 6 tests.

- [ ] **Step 6: Commit**

```bash
cd /home/hafiz/Desktop/Fender
# CHANGELOG.md: add under [Unreleased] > Added:
#   - internal/context: Manager core — 8K rule (CompactOutput), artifact write (0700/0600, per-call paths), catalog, 24h Cleanup sweep (D31/D38)
git add internal/context/context.go internal/context/context_test.go CHANGELOG.md
git commit -m "feat: context package core — 8K rule, artifact write, isolate sweep (D31)"
```

---

### Task 2: CompactInput — HEAD/TAIL preservation

**Files:**
- Modify: `internal/context/context.go` (add `CompactInput`)
- Modify: `internal/context/context_test.go`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: `Manager`, `Artifact`, `PreviewLimit`, `safeName`, `record` from Task 1.
- Produces (Task 3 relies on this):
  - `func (m *Manager) CompactInput(input string, preview int) (string, Artifact)` — `len(input) <= preview || preview <= 0` → `(input, Artifact{})`; else write full content to `<Root>/input-<unixnano>/user.txt` (0700/0600), record artifact, return `"[large user input: <N> chars at <path>; use read_file with offset and limit]\nHEAD:\n<input[:preview/2]>\n...\nTAIL:\n<input[len(input)-tail:]>` where `tail = preview - preview/2`, plus the artifact. Write failure → `(input[:preview], Artifact{})`.

- [ ] **Step 1: Write the failing test** — append to `internal/context/context_test.go`:

```go
func TestCompactInputSmallInline(t *testing.T) {
	m := newTestManager(t)
	got, art := m.CompactInput("short task", PreviewLimit)
	if got != "short task" || art.Path != "" {
		t.Fatalf("got %q art %+v", got, art)
	}
}

func TestCompactInputHeadTail(t *testing.T) {
	m := newTestManager(t)
	head, tail := PreviewLimit/2, PreviewLimit-PreviewLimit/2
	// Distinct middle so we can assert it never leaks inline.
	input := strings.Repeat("A", head) + strings.Repeat("B", PreviewLimit+1) + strings.Repeat("C", tail)
	got, art := m.CompactInput(input, PreviewLimit)
	if !strings.Contains(got, "large user input") ||
		!strings.Contains(got, "\nHEAD:\n") || !strings.Contains(got, "\nTAIL:\n") {
		t.Fatalf("pointer/HEAD/TAIL missing: %.100q", got)
	}
	if strings.Contains(got, "B") {
		t.Fatal("middle leaked inline")
	}
	if art.Path == "" || art.Size != len(input) {
		t.Fatalf("artifact = %+v", art)
	}
	data, err := os.ReadFile(art.Path)
	if err != nil || string(data) != input {
		t.Fatalf("artifact content: %v", err)
	}
	if cat := m.Catalog(); !strings.Contains(cat, art.Path) {
		t.Fatalf("catalog missing input artifact: %q", cat)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd /home/hafiz/Desktop/Fender && go test ./internal/context/ -run TestCompactInput`
Expected: FAIL — build error `undefined: CompactInput`.

- [ ] **Step 3: Write the implementation** — append to `internal/context/context.go` (after `CompactOutput`, before `safeName`):

```go
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd /home/hafiz/Desktop/Fender && go test ./internal/context/`
Expected: PASS — all 8 tests.

- [ ] **Step 5: Commit**

```bash
cd /home/hafiz/Desktop/Fender
# CHANGELOG.md: add under [Unreleased] > Added:
#   - internal/context: CompactInput — HEAD/TAIL preservation for large user input (D31)
git add internal/context/context.go internal/context/context_test.go CHANGELOG.md
git commit -m "feat: context CompactInput — HEAD/TAIL preservation (D31)"
```

---

### Task 3: For() — budget arithmetic, turns truncation, artifact catalog (acceptance #3)

**Files:**
- Create: `internal/context/for.go`
- Create: `internal/context/for_test.go`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: `Manager`, `CompactInput`, `Catalog`, `record`, `chars()`, `turns()` from Tasks 1–2; `provider.Message` from `internal/provider`.
- Produces (Task 4 relies on this):
  - `func (m *Manager) For(system string, msgs []provider.Message) []provider.Message` — budget arithmetic (mino `ContextFor`): `available = chars() − len(system) − len(Catalog())`; `preview = min(PreviewLimit, max(512, available/4))`; compact every user message > preview via `CompactInput(msg.Content, preview)`; replace every non-user message > preview with `"[Large previous <role> message (<N> chars) is retained in the session artifact catalog.]"`; turns truncation: if `len(history) > turns()*2`, keep the last `turns()*2` messages and prepend `"[<K> earlier turns compacted; full content is not retained.]"` where `K = (len(history)-keep)/2`; hard budget drop: while `total > chars() − len(system) − len(Catalog())` and `len(history) > 2`, drop the oldest pair (the current user turn is never dropped); finally append `Catalog()` as an assistant message when non-empty.
  - Guarantees: `len(system) + Σ len(msg.Content) ≤ ContextChars` for the returned list (while system + current user turn fit within the budget alone).

- [ ] **Step 1: Study the reference.** Read `~/Desktop/mino/session.go` `ContextMessages` (lines ~180–220) and `ContextFor` (lines ~226–244) — the arithmetic this task ports verbatim, and `~/Desktop/mino/context_test.go` `TestContextForBoundsCurrentInputAndKeepsArtifactCatalog` + `TestContextMessagesKeepsLastNTurnsOnly`, the tests being ported.

- [ ] **Step 2: Write the failing tests (the load-bearing budget test first)** — `internal/context/for_test.go`:

```go
package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/provider"
)

// TestForBudgetBound is acceptance #3 — the load-bearing test: system +
// Σ messages ≤ ContextChars, with oversized user input compacted to
// HEAD/TAIL and the artifact catalog riding in context (port of mino
// TestContextForBoundsCurrentInputAndKeepsArtifactCatalog).
func TestForBudgetBound(t *testing.T) {
	m := newTestManager(t)
	m.ContextChars = 12000
	old := filepath.Join(m.Root, "old-result.txt")
	if err := os.MkdirAll(m.Root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("old result"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.record(Artifact{Label: "bash", Path: old, Size: 10})

	msgs := []provider.Message{
		{Role: "user", Content: "HEAD=orchid"},
		{Role: "assistant", Content: "ack"},
		{Role: "user", Content: strings.Repeat("x", 140000)},
		{Role: "assistant", Content: "middle"},
		{Role: "user", Content: strings.Repeat("u", 30000)}, // current task
	}
	system := strings.Repeat("s", 500)
	out := m.For(system, msgs)

	total := len(system)
	joined := ""
	for _, msg := range out {
		total += len(msg.Content)
		joined += msg.Content
	}
	if total > m.ContextChars {
		t.Fatalf("budget exceeded: %d > %d", total, m.ContextChars)
	}
	if !strings.Contains(joined, "large user input") || !strings.Contains(joined, old) {
		t.Fatalf("context lost input or catalog: %q", joined)
	}
}

// TestForKeepsLastTurnsOnly ports mino TestContextMessagesKeepsLastNTurnsOnly:
// only the last MaxHistoryTurns turns survive, with a compaction marker.
func TestForKeepsLastTurnsOnly(t *testing.T) {
	m := newTestManager(t)
	m.MaxHistoryTurns = 2
	msgs := []provider.Message{
		{Role: "user", Content: "turn1-q"}, {Role: "assistant", Content: "turn1-a"},
		{Role: "user", Content: "turn2-q"}, {Role: "assistant", Content: "turn2-a"},
		{Role: "user", Content: "turn3-q"}, {Role: "assistant", Content: "turn3-a"},
		{Role: "user", Content: "turn4-q"}, {Role: "assistant", Content: "turn4-a"},
		{Role: "user", Content: "turn5-q"}, {Role: "assistant", Content: "turn5-a"},
	}
	out := m.For("", msgs)
	joined := ""
	for _, msg := range out {
		joined += msg.Content
	}
	if !strings.Contains(joined, "turn4-q") || !strings.Contains(joined, "turn5-a") {
		t.Fatalf("last 2 turns missing: %q", joined)
	}
	if strings.Contains(joined, "turn1") || strings.Contains(joined, "turn2") || strings.Contains(joined, "turn3") {
		t.Fatalf("older turns leaked: %q", joined)
	}
	if !strings.Contains(joined, "3 earlier turns compacted") {
		t.Fatalf("compaction marker missing: %q", joined)
	}
}

// TestForCompactsOversizedHistory: non-user messages over the preview are
// replaced by a catalog note, never dropped silently (mino ContextMessages).
func TestForCompactsOversizedHistory(t *testing.T) {
	m := newTestManager(t)
	msgs := []provider.Message{
		{Role: "assistant", Content: strings.Repeat("z", PreviewLimit+1)},
		{Role: "user", Content: "current task"},
	}
	out := m.For("", msgs)
	if !strings.Contains(out[0].Content, "Large previous assistant message") {
		t.Fatalf("history not noted: %.80q", out[0].Content)
	}
	if out[1].Content != "current task" {
		t.Fatalf("current user message altered: %q", out[1].Content)
	}
}

// TestForCatalogRides: artifacts recorded before For() are listed in the
// returned messages (D31: select — the catalog rides in context).
func TestForCatalogRides(t *testing.T) {
	m := newTestManager(t)
	m.CompactOutput("shell", strings.Repeat("x", InlineLimit+1))
	out := m.For("", []provider.Message{{Role: "user", Content: "hi"}})
	found := false
	for _, msg := range out {
		if strings.Contains(msg.Content, "Live session artifacts") {
			found = true
		}
	}
	if !found {
		t.Fatal("catalog did not ride in context")
	}
}

// TestForBudgetDropsOldestPairs: when turns truncation alone cannot fit the
// budget, the oldest pairs are dropped — but the current user turn survives.
func TestForBudgetDropsOldestPairs(t *testing.T) {
	m := newTestManager(t)
	m.ContextChars = 9000
	m.MaxHistoryTurns = 100 // disable turns truncation; budget drop must do the work
	var msgs []provider.Message
	for i := 0; i < 12; i++ {
		msgs = append(msgs,
			provider.Message{Role: "user", Content: strings.Repeat("a", 7000) + fmt.Sprint(i)},
			provider.Message{Role: "assistant", Content: strings.Repeat("b", 7000)})
	}
	system := strings.Repeat("s", 1000)
	out := m.For(system, msgs)
	total := len(system)
	for _, msg := range out {
		total += len(msg.Content)
	}
	if total > m.ContextChars {
		t.Fatalf("budget exceeded: %d > %d", total, m.ContextChars)
	}
	// The current user turn survives the drops (the catalog is appended
	// after it, so scan for the last user message rather than out[len-1]).
	lastUser := ""
	for _, msg := range out {
		if msg.Role == "user" {
			lastUser = msg.Content
		}
	}
	if !strings.Contains(lastUser, "11") {
		t.Fatal("current user turn was dropped")
	}
}
```

- [ ] **Step 3: Run them to verify they fail**

Run: `cd /home/hafiz/Desktop/Fender && go test ./internal/context/ -run TestFor`
Expected: FAIL — build error `undefined: For`.

- [ ] **Step 4: Write the implementation** — `internal/context/for.go`:

```go
package context

import (
	"fmt"

	"github.com/H4fizWasabie/fender/internal/provider"
)

// For applies the budget arithmetic (mino ContextFor, D31): compacts
// oversized user messages to HEAD/TAIL, notes oversized history messages,
// truncates history to the last MaxHistoryTurns turns with a compaction
// marker, appends the artifact catalog, and guarantees
// len(system) + Σ message content ≤ ContextChars. The current user turn is
// never dropped (mino's rule).
func (m *Manager) For(system string, msgs []provider.Message) []provider.Message {
	available := m.chars() - len(system) - len(m.Catalog())
	preview := PreviewLimit
	if p := available / 4; p < preview {
		preview = p
	}
	if preview < 512 {
		preview = 512
	}

	// Compress (D31): compact oversized user messages (HEAD/TAIL), note
	// oversized history messages instead of dropping them silently.
	history := make([]provider.Message, 0, len(msgs))
	for _, msg := range msgs {
		if len(msg.Content) <= preview {
			history = append(history, msg)
			continue
		}
		if msg.Role == "user" {
			compacted, _ := m.CompactInput(msg.Content, preview)
			history = append(history, provider.Message{Role: "user", Content: compacted})
			continue
		}
		history = append(history, provider.Message{Role: msg.Role,
			Content: fmt.Sprintf("[Large previous %s message (%d chars) is retained in the session artifact catalog.]", msg.Role, len(msg.Content))})
	}

	// Turns truncation with a compaction marker (mino ContextMessages).
	keep := m.turns() * 2
	if len(history) > keep {
		marker := fmt.Sprintf("[%d earlier turns compacted; full content is not retained.]", (len(history)-keep)/2)
		history = append([]provider.Message{{Role: "assistant", Content: marker}}, history[len(history)-keep:]...)
	}

	// Hard budget bound: drop oldest pairs until the budget fits. The last
	// pair (marker + current user turn) is never dropped.
	budget := m.chars() - len(system) - len(m.Catalog())
	total := 0
	for _, msg := range history {
		total += len(msg.Content)
	}
	for total > budget && len(history) > 2 {
		total -= len(history[0].Content) + len(history[1].Content)
		history = history[2:]
	}

	// Artifact catalog rides in context (D31: select).
	if cat := m.Catalog(); cat != "" {
		history = append(history, provider.Message{Role: "assistant", Content: cat})
	}
	return history
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd /home/hafiz/Desktop/Fender && go test ./internal/context/`
Expected: PASS — all 13 tests. If `TestForBudgetBound` fails on `total > ContextChars`, the arithmetic drifted from this code — fix the implementation, not the test (the test is acceptance #3).

- [ ] **Step 6: Commit**

```bash
cd /home/hafiz/Desktop/Fender
# CHANGELOG.md: add under [Unreleased] > Added:
#   - internal/context: For() — budget arithmetic (system+Σmsgs ≤ ContextChars), turns truncation + marker, artifact catalog in context (D31)
git add internal/context/for.go internal/context/for_test.go CHANGELOG.md
git commit -m "feat: context For() — budget arithmetic, turns truncation, artifact catalog (D31)"
```

---

### Task 4: Agent loop wiring — ingress For(), tool-result compaction, dedup pointers

**Files:**
- Modify: `internal/agent/agent.go`
- Modify: `internal/agent/agent_test.go`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: `ctxpkg.Manager` (Task 1), `ctxpkg.SweepAge`, `ctxpkg` alias for `internal/context` (the stdlib `context` is already imported in agent.go — alias required).
- Produces: `Agent.Ctx *ctxpkg.Manager` field (nil = ticket-03 behavior); `Run` calls `a.Ctx.Cleanup(ctxpkg.SweepAge)` + `a.Ctx.For(a.System, msgs)` at ingress (before the system-prepend) and `a.Ctx.CompactOutput(tc.Function.Name, out)` on every successful tool result before the dedup store. Existing tests (nil Ctx) must pass unchanged.

- [ ] **Step 1: Write the failing integration tests** — append to `internal/agent/agent_test.go`:

```go
func TestRunCompactsLargeToolOutput(t *testing.T) {
	proj := t.TempDir()
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "shell", `{"cmd":"printf 'y%.0s' {1..9000}"}`),
		completeReply("complete", "done"),
	}}
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Yolo, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	a.Ctx = ctxpkg.New()
	a.Ctx.Root = filepath.Join(t.TempDir(), "run")
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "run it"}})
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	got := lastToolResult(t, f)
	if !strings.Contains(got, "[artifact:") {
		t.Fatalf("tool result not compacted: %.100q", got)
	}
	if strings.Contains(got, strings.Repeat("y", 9000)) {
		t.Fatal("raw 9K output leaked inline")
	}
}

func TestRunReadFileStaysInline(t *testing.T) {
	proj := t.TempDir()
	big := strings.Repeat("r", 10000)
	if err := os.WriteFile(filepath.Join(proj, "big.txt"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "read_file", `{"path":"big.txt"}`),
		completeReply("complete", "ok"),
	}}
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	a.Ctx = ctxpkg.New()
	a.Ctx.Root = filepath.Join(t.TempDir(), "run")
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, f); got != big {
		t.Fatalf("read_file result altered: %d chars", len(got))
	}
}

func TestRunDedupReplaysPointer(t *testing.T) {
	proj := t.TempDir()
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "shell", `{"cmd":"printf 'y%.0s' {1..9000}"}`),
		toolReply("call_2", "shell", `{"cmd":"printf 'y%.0s' {1..9000}"}`),
		completeReply("complete", "ok"),
	}}
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Yolo, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	a.Ctx = ctxpkg.New()
	a.Ctx.Root = filepath.Join(t.TempDir(), "run")
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	found := false
	for _, m := range f.last().Messages {
		if m.Role == "tool" && strings.HasPrefix(m.Content, "[already executed] [artifact:") {
			found = true
		}
	}
	if !found {
		t.Fatal("dedup replay lost the artifact pointer")
	}
}

func TestRunCompactsLargeUserInput(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{completeReply("complete", "ok")}}
	a, _ := newTestAgent(t, f)
	a.Ctx = ctxpkg.New()
	a.Ctx.Root = filepath.Join(t.TempDir(), "run")
	big := strings.Repeat("t", 30000)
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: big}})
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if msg := f.last().Messages[0]; !strings.Contains(msg.Content, "large user input") {
		t.Fatalf("task not compacted: %.100q", msg.Content)
	}
}
```

Add the import (the existing import block already has `"context"` — stdlib — so alias):

```go
	ctxpkg "github.com/H4fizWasabie/fender/internal/context"
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd /home/hafiz/Desktop/Fender && go test ./internal/agent/ -run "TestRunCompacts|TestRunReadFileStaysInline|TestRunDedupReplaysPointer|TestRunCompactsLargeUserInput"`
Expected: FAIL — build error `undefined: ctxpkg` (not yet imported/used).

- [ ] **Step 3: Write the implementation** — `internal/agent/agent.go`:

Import (alias — stdlib `context` is already imported):

```go
import (
	"context"
	"fmt"

	ctxpkg "github.com/H4fizWasabie/fender/internal/context"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/tools"
)
```

```go
// Agent is one loop: model + tools + discipline. The same type runs the
// parent and every subagent (D13).
type Agent struct {
	LLM        LLM
	Resolver   Resolver // subagent provider selection (D7); nil -> inherit parent LLM
	System     string
	MaxIter    int // 0 -> defaultMaxIter
	MaxSubIter int // 0 -> defaultMaxSubIter
	Ctx        *ctxpkg.Manager // D31 artifact layer; nil = ticket-03 behavior
	registry   *tools.Registry
}
```

In `Run`, replace the system-prepend block:

```go
	if a.Ctx != nil {
		a.Ctx.Cleanup(ctxpkg.SweepAge)
		msgs = a.Ctx.For(a.System, msgs)
	}
	if a.System != "" && (len(msgs) == 0 || msgs[0].Role != "system") {
		msgs = append([]provider.Message{{Role: "system", Content: a.System}}, msgs...)
	}
```

In the Act phase, after the successful execute and before the dedup store:

```go
			out, err := a.registry.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				errors++
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: "Error: " + err.Error()})
				continue
			}
			if a.Ctx != nil {
				out = a.Ctx.CompactOutput(tc.Function.Name, out)
			}
			dedup[key] = out
```

> The dedup cache stores the pointer — a repeated identical call replays `[already executed] [artifact: …]`, exactly what the model saw the first time. For() runs before the system-prepend, so the budget arithmetic reserves `len(system)` and the final list still satisfies acceptance #3.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd /home/hafiz/Desktop/Fender && go test ./internal/agent/`
Expected: PASS — all existing tests (nil Ctx unchanged) + 4 new integration tests.

- [ ] **Step 5: Commit**

```bash
cd /home/hafiz/Desktop/Fender
# CHANGELOG.md: add under [Unreleased] > Added:
#   - internal/agent: loop wired to context layer — For() at ingress, CompactOutput on tool results, dedup caches pointers (D31)
git add internal/agent/agent.go internal/agent/agent_test.go CHANGELOG.md
git commit -m "feat: agent loop wired to context layer (D31)"
```

---

### Task 5: Delegate child context — Child() wiring

**Files:**
- Modify: `internal/agent/delegate.go`
- Modify: `internal/agent/delegate_test.go`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: `Agent.Ctx`, `ctxpkg.Manager.Child()` (Task 1).
- Produces: child `Agent` gets `Ctx = a.Ctx.Child()` when the parent has one (nil-safe — a parent without Ctx still spawns ticket-03 children).

- [ ] **Step 1: Write the failing test** — append to `internal/agent/delegate_test.go`:

```go
func TestDelegateChildGetsOwnContext(t *testing.T) {
	child := &fakeLLM{steps: []*provider.Response{
		toolReply("call_c1", "shell", `{"cmd":"printf 'y%.0s' {1..9000}"}`),
		completeReply("complete", "child done"),
	}}
	parent := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"do the big thing","provider":"child"}`),
		completeReply("complete", "parent done"),
	}}
	a, _ := newTestAgent(t, parent)
	a.Resolver = func(name string) (LLM, error) { return child, nil }
	a.Ctx = ctxpkg.New()
	a.Ctx.Root = filepath.Join(t.TempDir(), "parent-run")
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	// The child loop compacted its big output into ITS run dir.
	pointer := ""
	for _, req := range child.all() {
		for _, m := range req.Messages {
			if strings.Contains(m.Content, "[artifact:") {
				pointer = m.Content
			}
		}
	}
	if pointer == "" {
		t.Fatal("child never compacted")
	}
	_, after, _ := strings.Cut(pointer, " at ")
	path, _, _ := strings.Cut(after, ";")
	if !strings.HasPrefix(path, filepath.Dir(a.Ctx.Root)+"/") {
		t.Fatalf("child artifact outside child root: %q", path)
	}
	if strings.HasPrefix(path, a.Ctx.Root+"/") {
		t.Fatal("child artifact leaked into parent root")
	}
	if strings.Contains(a.Ctx.Catalog(), path) {
		t.Fatal("child artifact recorded in parent catalog")
	}
}
```

Add imports to `delegate_test.go`: `"path/filepath"`, `ctxpkg "github.com/H4fizWasabie/fender/internal/context"`.

- [ ] **Step 2: Run it to verify it fails**

Run: `cd /home/hafiz/Desktop/Fender && go test ./internal/agent/ -run TestDelegateChildGetsOwnContext`
Expected: FAIL — the child's tool result stays inline (no pointer found → `child never compacted`), because the child's Ctx is nil.

- [ ] **Step 3: Write the implementation** — `internal/agent/delegate.go`:

Import (alias): `ctxpkg "github.com/H4fizWasabie/fender/internal/context"`.

In the delegate tool's Call, after the child Agent literal:

```go
			child := &Agent{
				LLM:        llm,
				Resolver:   a.Resolver,
				System:     subagentSystem,
				MaxIter:    a.subIter(),
				MaxSubIter: a.MaxSubIter,
				registry:   a.registry.Without("delegate"),
			}
			if a.Ctx != nil {
				child.Ctx = a.Ctx.Child() // D38: isolated artifacts + catalog
			}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd /home/hafiz/Desktop/Fender && go test ./internal/agent/`
Expected: PASS — all agent tests, including both delegate tests.

- [ ] **Step 5: Commit**

```bash
cd /home/hafiz/Desktop/Fender
# CHANGELOG.md: add under [Unreleased] > Added:
#   - internal/agent: delegate children get isolated context managers (D38)
git add internal/agent/delegate.go internal/agent/delegate_test.go CHANGELOG.md
git commit -m "feat: delegate child gets isolated context manager (D38)"
```

---

### Task 6: Tool caps — shell 8 MiB, read ceiling comment

**Files:**
- Modify: `internal/tools/shell.go`
- Modify: `internal/tools/read.go`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: nothing new (no signature changes — behavior only).
- Produces: `outputCap = 8 << 20` (8 MiB) in shell.go; corrected comment in read.go. The 64 KiB shell truncation was lossy — the artifact layer (Task 4) now carries full output; the cap is a memory ceiling. The 1 MiB read cap stays (read_file is never compacted, so it is the only bound on inline size).

- [ ] **Step 1: Make the changes**

`internal/tools/shell.go`:

```go
// outputCap bounds command output held in memory (8 MiB). Full output now
// reaches the model via the artifact layer (D31/D38): anything over
// InlineLimit becomes a pointer. This cap is a memory ceiling, not a
// content limit — ticket-03's 64 KiB truncation was lossy and is gone.
const outputCap = 8 << 20 // 8 MiB
```

`internal/tools/read.go`:

```go
// readCap is the inline safety ceiling for read_file output. read_file is
// never compacted (D31: its result is the explicit slice the model asked
// for), so this cap is the only bound on inline size — it stays.
const readCap = 1 << 20 // 1 MiB
```

- [ ] **Step 2: Verify the whole suite**

Run: `cd /home/hafiz/Desktop/Fender && go test ./... && go vet ./... && go build ./...`
Expected: PASS everywhere, vet clean, build succeeds. (No test asserts the old 64 KiB truncation — verified during planning.)

- [ ] **Step 3: Commit**

```bash
cd /home/hafiz/Desktop/Fender
# CHANGELOG.md: add under [Unreleased] > Changed:
#   - internal/tools: shell output cap 64 KiB → 8 MiB (memory ceiling; artifact layer carries full output); read cap comment corrected (D38)
git add internal/tools/shell.go internal/tools/read.go CHANGELOG.md
git commit -m "chore: shell cap 8MiB (artifacts carry full output), read cap comment (D38)"
```

---

### Task 7: Wrap-up — full verification, wayfinder ticket 04, frontier 05

**Files:**
- Modify: `.scratch/fender/issues/04-Context.md` (status → resolved, Answer filled)
- Modify: `.scratch/fender/map.md` (decisions line + unblocks)
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: everything from Tasks 1–6.

- [ ] **Step 1: Full verification**

Run:
```bash
cd /home/hafiz/Desktop/Fender
go test ./...
go vet ./...
go build ./...
git status
```
Expected: all green, vet clean, single binary builds, working tree clean (after commit).

- [ ] **Step 2: Resolve the wayfinder ticket** — `.scratch/fender/issues/04-Context.md`:

```markdown
# 04-Context

Type: task
Status: resolved
Blocked by: 03

## Question

Write + execute Plan 4: context artifact engineering (mino port, D31) — 8K inline limit, HEAD/TAIL, read_file never compacted, write-elsewhere, isolate. Port `context_test.go` techniques as Fender's context test suite (budget bounds, head/tail, compaction markers, artifact catalog).

## Answer

Plan 4 done (`docs/superpowers/plans/2026-08-04-fender-context.md`, 7 tasks):

- `internal/context`: Manager (root /tmp/fender/artifacts/<runID>, ContextChars 100K, MaxHistoryTurns 5) — CompactOutput (8K rule; read_file never compacted; 0700/0600 artifact write; per-call paths; write-failure marker), CompactInput (HEAD/TAIL), For() budget arithmetic (system + Σmsgs ≤ ContextChars, turns truncation + marker, artifact catalog ≤2K rides in context), Cleanup(24h) at Run start, Child() isolation.
- Agent loop wired: ingress For(), tool results compacted before history, dedup caches pointers, delegate children get Child().
- Tool caps: shell 64 KiB → 8 MiB (memory ceiling — artifacts carry full output); read 1 MiB stays (never compacted).
- 21 new tests including the load-bearing budget-bound test (acceptance #3); all ticket-03 tests pass unchanged (nil-safe Ctx).

Unblocks 05 (memory/ICM layers).
```

- [ ] **Step 3: Update the map** — `.scratch/fender/map.md`, under "Decisions so far" append:

```markdown
- [04-Context](issues/04-Context.md) — Plan 4 done: internal/context artifact engineering (8K rule, HEAD/TAIL, For() budget arithmetic ≤ ContextChars, artifact catalog, 24h sweep, Child() isolation); loop wired; shell cap 8 MiB; budget-bound test first-class (acceptance #3). Unblocks 05.
```

- [ ] **Step 4: Commit**

```bash
cd /home/hafiz/Desktop/Fender
# CHANGELOG.md: add under [Unreleased] > Added:
#   - docs: wayfinder ticket 04 resolved (context artifact engineering complete)
git add .scratch/fender/issues/04-Context.md .scratch/fender/map.md CHANGELOG.md
git commit -m "docs: resolve wayfinder ticket 04 (context done)"
```

- [ ] **Step 5: Confirm the frontier** — ticket 05 (Memory/ICM layers, D14/D17/D28) is the next plan. Report: state (test count, packages), the load-bearing budget test result, and the frontier to the user.

---

## Self-Review Notes (written before execution, verified during planning)

**Spec coverage** (spec `2026-08-04-fender-context-design.md` sections → tasks):
- §4 API: Manager/New/Child (T1), CompactOutput (T1), CompactInput (T2), For (T3), Catalog (T1), Cleanup (T1) — all present with exact signatures.
- §5 Agent wiring: Ctx field + ingress For + Cleanup at Run start + tool-result compaction + dedup pointers (T4); delegate Child() (T5).
- §6 Cap policy: shell 8 MiB, read comment (T6).
- §7 Test strategy: every row of the test table lands in T1/T3/T4/T5.
- §8 Acceptance: #1 suite green (T6/T7), #2 no >8K inline except read_file (T4 tests), #3 budget bound (T3 — load-bearing, user-required), #4 ticket-03 tests unchanged (T4 step 4), #5 CHANGELOG every commit (all tasks), #6 wayfinder resolution (T7).
- §9 Deferred: nothing built (seams only) — honored by omission.

**Type consistency check** (run across tasks): `CompactInput(input string, preview int) (string, Artifact)` — T2 defines, T3 calls with `(msg.Content, preview)`; `CompactOutput(tool, output string) string` — T1 defines, T4 calls with `(tc.Function.Name, out)`; `For(system string, msgs []provider.Message) []provider.Message` — T3 defines, T4 calls `a.Ctx.For(a.System, msgs)`; `Child()` — T1 defines, T5 calls `a.Ctx.Child()`; `Catalog()` — T1 defines, T3 renders, T5 asserts. `ctxpkg` alias used consistently in agent.go/agent_test.go/delegate_test.go. `Artifact{Label, Path, Size}` consistent across record/catalog/tests. No dangling names.

**Placeholder scan**: every step carries full code or exact commands; no "add error handling", no "similar to Task N", no TBD. The two `printf 'y%.0s' {1..9000}` shell invocations (T4, T5) were verified against the shell tool (runs `bash -c`; benign-verdict RUN under Yolo; 9000 chars > 8K so compaction triggers).
