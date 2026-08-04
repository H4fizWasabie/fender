# Fender Plan 5: Memory / ICM Layers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `internal/memory` — the ICM memory workspace (`.fender/`), convention-file detection (AGENTS.md/CLAUDE.md/CONTEXT.md loaded directly), always-loaded system assembly (8K cap), working-memory pruning — wired nil-safely into the agent loop.

**Architecture:** One struct (`Memory`), one bootstrap path (`Ensure` → `Detect` → `Bootstrap`). Convention files load DIRECTLY (canonical sources — never copied into PROJECT.md). Layered precedence: user `~/.fender/AGENTS.md` → project `AGENTS.md` (fallback `CLAUDE.md`) → `CONTEXT.md` → `PROJECT.md`. Always-loaded layer capped at 8K chars so memory never triggers ticket-04 compaction by itself. Memory graph, consolidation, SQLite facts: deferred to D9 era (spec §2 — approved).

**Tech Stack:** Go 1.22, stdlib only (`os`, `path/filepath`, `time`, `strings`, `slices`). No new dependencies.

## Global Constraints

- **Read `AGENTS.md`, `DECISIONS.md` (D39), the ticket-05 spec, and ticket-04's spec first.** They are the law.
- **Every commit MUST stage `CHANGELOG.md`** — enforced by `.githooks/pre-commit`.
- **Allowed dependencies only:** `BurntSushi/toml`, `mvdan.cc/sh/v3`, `go-tree-sitter`, `modernc.org/sqlite`. Nothing else.
- **No frameworks. Stdlib only.** Explicit errors, `log/slog`, no panic in library code.
- **Nil-safe integration:** existing ticket-03/04 tests must pass unchanged.
- Module path `github.com/H4fizWasabie/fender`; files live under `internal/memory/`.

---

### Task 1: Memory core + workspace scaffold (Ensure)

**Files:**
- Create: `internal/memory/memory.go`
- Create: `internal/memory/memory_test.go`

**Interfaces:**
- Produces:
  - `const NotesMaxAge = 7 * 24 * time.Hour`
  - `const SystemCap = 8000`
  - `type Memory struct` (unexported `root string`) with:
    - `func New(root string) *Memory`
    - `func (m *Memory) Ensure() error` — idempotent scaffold: `.fender/memory/{PROJECT.md, MAP.md, reference/, working/, facts/}` + `.fender/skills/`

- [ ] **Step 1: Write the failing test**

`internal/memory/memory_test.go`:

```go
package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCreatesStructure(t *testing.T) {
	root := t.TempDir()
	m := New(root)
	if err := m.Ensure(); err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(root, ".fender", "memory", "PROJECT.md"),
		filepath.Join(root, ".fender", "memory", "MAP.md"),
		filepath.Join(root, ".fender", "memory", "reference"),
		filepath.Join(root, ".fender", "memory", "working"),
		filepath.Join(root, ".fender", "memory", "facts"),
		filepath.Join(root, ".fender", "skills"),
	}
	for _, p := range want {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
	}
}

func TestEnsureIdempotent(t *testing.T) {
	root := t.TempDir()
	m := New(root)
	if err := m.Ensure(); err != nil {
		t.Fatal(err)
	}
	// user edits PROJECT.md
	proj := filepath.Join(root, ".fender", "memory", "PROJECT.md")
	if err := os.WriteFile(proj, []byte("# user edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.Ensure(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(proj)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# user edit" {
		t.Fatalf("Ensure overwrote user edit: %q", got)
	}
}

func TestEnsureSeedsTemplates(t *testing.T) {
	root := t.TempDir()
	m := New(root)
	if err := m.Ensure(); err != nil {
		t.Fatal(err)
	}
	proj, err := os.ReadFile(filepath.Join(root, ".fender", "memory", "PROJECT.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(proj), "PROJECT.md") {
		t.Fatalf("PROJECT.md template missing marker: %q", proj)
	}
	mapmd, err := os.ReadFile(filepath.Join(root, ".fender", "memory", "MAP.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(mapmd), "ticket 07") {
		t.Fatalf("MAP.md placeholder missing code-intel note: %q", mapmd)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/memory/ -v`
Expected: FAIL — no Go files in `internal/memory`.

- [ ] **Step 3: Write the implementation**

`internal/memory/memory.go`:

```go
// Package memory implements Fender's ICM memory workspace (D14, D17, D39).
// Convention files load DIRECTLY — canonical sources, never copied into PROJECT.md.
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	NotesMaxAge = 7 * 24 * time.Hour // working/ prune window (mino working_memory.md rule)
	SystemCap   = 8000               // always-loaded layer ceiling (prevention over compression)
)

const projectTemplate = `# PROJECT.md — always-loaded memory (Layer 0)

What this project is, conventions, build commands. Keep small (<2K chars).
`

const mapTemplate = `# MAP.md — navigation (Layer 1)

_Not generated yet — code-intel (ticket 07) replaces this body.
Maintain by hand until then: one "## <area>" section per module, one line each.
_

## Areas

- (none recorded yet)
`

type Memory struct {
	root string
}

func New(root string) *Memory {
	return &Memory{root: root}
}

// Ensure creates the .fender/ workspace if missing. Idempotent: never
// overwrites existing files (user edits survive).
func (m *Memory) Ensure() error {
	dirs := []string{
		filepath.Join(m.root, ".fender", "memory", "reference"),
		filepath.Join(m.root, ".fender", "memory", "working"),
		filepath.Join(m.root, ".fender", "memory", "facts"), // reserved (D39)
		filepath.Join(m.root, ".fender", "skills"),          // ticket 06 seam
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0700); err != nil {
			return fmt.Errorf("memory ensure: %w", err)
		}
	}
	files := []struct {
		path    string
		content string
	}{
		{filepath.Join(m.root, ".fender", "memory", "PROJECT.md"), projectTemplate},
		{filepath.Join(m.root, ".fender", "memory", "MAP.md"), mapTemplate},
	}
	for _, f := range files {
		if _, err := os.Stat(f.path); os.IsNotExist(err) {
			if err := os.WriteFile(f.path, []byte(f.content), 0600); err != nil {
				return fmt.Errorf("memory ensure: %w", err)
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/memory/ -v`
Expected: all three PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/memory/ CHANGELOG.md
git commit -m "feat: memory workspace scaffold (Ensure, idempotent, templates)"
```

CHANGELOG entry:

```markdown
### Added
- `internal/memory`: Ensure() scaffold — .fender/{memory/{PROJECT.md,MAP.md,reference/,working/,facts/},skills/}, idempotent, seeded templates
```

---

### Task 2: Convention-file detection

**Files:**
- Create: `internal/memory/detect.go`
- Create: `internal/memory/detect_test.go`

**Interfaces:**
- Consumes: `Memory` from Task 1.
- Produces:
  - `type ConventionFile struct { Path string; Kind string; Layer string }` — Kind: `"AGENTS.md" | "CLAUDE.md" | "CONTEXT.md"`; Layer: `"user" | "project"`
  - `func (m *Memory) Detect(dir string) []ConventionFile` — order: user `~/.fender/AGENTS.md` (exists only), project `AGENTS.md` (fallback `CLAUDE.md`), project `CONTEXT.md` (exists only). `README.md`/`CONTRIBUTING.md`/`.cursorrules` never detected.

- [ ] **Step 1: Write the failing test**

`internal/memory/detect_test.go`:

```go
package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// user-level global rules
	userDir := filepath.Join(home, ".fender")
	if err := os.MkdirAll(userDir, 0700); err != nil {
		t.Fatal(err)
	}
	userAgents := filepath.Join(userDir, "AGENTS.md")
	os.WriteFile(userAgents, []byte("user rules"), 0600)

	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("project rules"), 0600)
	os.WriteFile(filepath.Join(root, "CONTEXT.md"), []byte("context"), 0600)
	// decoys — must never be detected
	os.WriteFile(filepath.Join(root, "README.md"), []byte("readme"), 0600)
	os.WriteFile(filepath.Join(root, ".cursorrules"), []byte("cursor"), 0600)

	m := New(root)
	got := m.Detect(root)
	if len(got) != 3 {
		t.Fatalf("detected %d files: %+v", len(got), got)
	}
	if got[0].Path != userAgents || got[0].Layer != "user" || got[0].Kind != "AGENTS.md" {
		t.Fatalf("got[0] = %+v", got[0])
	}
	if got[1].Kind != "AGENTS.md" || got[1].Layer != "project" {
		t.Fatalf("got[1] = %+v", got[1])
	}
	if got[2].Kind != "CONTEXT.md" {
		t.Fatalf("got[2] = %+v", got[2])
	}
}

func TestDetectClaudeFallback(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("claude rules"), 0600)
	m := New(root)
	got := m.Detect(root)
	if len(got) != 1 || got[0].Kind != "CLAUDE.md" {
		t.Fatalf("got = %+v", got)
	}
}

func TestDetectNothing(t *testing.T) {
	m := New(t.TempDir())
	if got := m.Detect(t.TempDir()); len(got) != 0 {
		t.Fatalf("got = %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/memory/ -run TestDetect -v`
Expected: FAIL — `Detect` undefined.

- [ ] **Step 3: Write the implementation**

`internal/memory/detect.go`:

```go
package memory

import (
	"os"
	"path/filepath"
)

// ConventionFile is a detected project-rules file. Content is read at
// assembly time (System), never copied into PROJECT.md (canonical sources).
type ConventionFile struct {
	Path  string // absolute path
	Kind  string // "AGENTS.md" | "CLAUDE.md" | "CONTEXT.md"
	Layer string // "user" | "project"
}

// Detect finds convention files for dir, in precedence order:
// user ~/.fender/AGENTS.md → project AGENTS.md (fallback CLAUDE.md) → CONTEXT.md.
// README.md / CONTRIBUTING.md / .cursorrules are never auto-loaded.
// Dir-aware for the nested-AGENTS.md seam (v1 loads root level only).
func (m *Memory) Detect(dir string) []ConventionFile {
	var out []ConventionFile
	home, err := os.UserHomeDir()
	if err == nil {
		if p := filepath.Join(home, ".fender", "AGENTS.md"); exists(p) {
			out = append(out, ConventionFile{Path: p, Kind: "AGENTS.md", Layer: "user"})
		}
	}
	agents := filepath.Join(dir, "AGENTS.md")
	if exists(agents) {
		out = append(out, ConventionFile{Path: agents, Kind: "AGENTS.md", Layer: "project"})
	} else if claude := filepath.Join(dir, "CLAUDE.md"); exists(claude) {
		out = append(out, ConventionFile{Path: claude, Kind: "CLAUDE.md", Layer: "project"})
	}
	if ctx := filepath.Join(dir, "CONTEXT.md"); exists(ctx) {
		out = append(out, ConventionFile{Path: ctx, Kind: "CONTEXT.md", Layer: "project"})
	}
	return out
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/memory/ -run TestDetect -v`
Expected: all three PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/memory/ CHANGELOG.md
git commit -m "feat: convention-file detection (AGENTS.md/CLAUDE.md/CONTEXT.md, layered precedence)"
```

CHANGELOG entry:

```markdown
### Added
- Convention detection: Detect() — user ~/.fender/AGENTS.md → project AGENTS.md (CLAUDE.md fallback) → CONTEXT.md; README/.cursorrules never auto-loaded
```

---

### Task 3: Bootstrap + System assembly (8K cap)

**Files:**
- Create: `internal/memory/bootstrap.go`
- Create: `internal/memory/bootstrap_test.go`

**Interfaces:**
- Consumes: `Ensure` (Task 1), `Detect` (Task 2).
- Produces:
  - `type Bootstrap struct { Convention []ConventionFile; ProjectMD string; MAPMD string }`
  - `func (m *Memory) Bootstrap() (*Bootstrap, error)` — Ensure + Detect + prune (Task 4 adds pruning; here it's Ensure + Detect + read PROJECT.md/MAP.md; unreadable files are skipped, never an error)
  - `func (b *Bootstrap) System() string` — convention contents (in order) + PROJECT.md, each wrapped in provenance markers; total capped at SystemCap (8000), truncating oldest sections first with a marker.

- [ ] **Step 1: Write the failing test**

`internal/memory/bootstrap_test.go`:

```go
package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapReadsLayers(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("project rules"), 0600)
	os.WriteFile(filepath.Join(root, "CONTEXT.md"), []byte("context rules"), 0600)
	m := New(root)
	b, err := m.Bootstrap()
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Convention) != 2 {
		t.Fatalf("convention = %+v", b.Convention)
	}
	if !strings.Contains(b.ProjectMD, "PROJECT.md") {
		t.Fatalf("ProjectMD = %q", b.ProjectMD)
	}
	if !strings.Contains(b.MAPMD, "ticket 07") {
		t.Fatalf("MAPMD = %q", b.MAPMD)
	}
}

func TestSystemOrderAndCanonicalSources(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".fender"), 0700)
	os.WriteFile(filepath.Join(home, ".fender", "AGENTS.md"), []byte("USER-RULES"), 0600)

	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("PROJECT-RULES"), 0600)
	os.WriteFile(filepath.Join(root, "CONTEXT.md"), []byte("CONTEXT-RULES"), 0600)
	m := New(root)
	b, _ := m.Bootstrap()
	sys := b.System()
	// order: user first, then project AGENTS, then CONTEXT, then PROJECT.md
	ui := strings.Index(sys, "USER-RULES")
	pi := strings.Index(sys, "PROJECT-RULES")
	ci := strings.Index(sys, "CONTEXT-RULES")
	mi := strings.Index(sys, "PROJECT.md")
	if !(ui < pi && pi < ci && ci < mi) {
		t.Fatalf("order wrong: user=%d project=%d context=%d projectmd=%d\n%s", ui, pi, ci, mi, sys)
	}
	// canonical sources: PROJECT.md never contains copied convention content
	if strings.Contains(b.ProjectMD, "PROJECT-RULES") || strings.Contains(b.ProjectMD, "USER-RULES") {
		t.Fatal("PROJECT.md contains copied convention content")
	}
}

func TestSystemCapTruncatesOldestFirst(t *testing.T) {
	// make the always-loaded layer exceed SystemCap via a huge user AGENTS.md
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".fender"), 0700)
	os.WriteFile(filepath.Join(home, ".fender", "AGENTS.md"), []byte(strings.Repeat("x", SystemCap+1000)), 0600)
	root := t.TempDir()
	m := New(root)
	b, _ := m.Bootstrap()
	sys := b.System()
	if len(sys) > SystemCap {
		t.Fatalf("System() exceeded cap: %d > %d", len(sys), SystemCap)
	}
	if !strings.Contains(sys, "truncated") {
		t.Fatalf("missing truncation marker: %q", sys[:200])
	}
}

func TestBootstrapSkipsUnreadable(t *testing.T) {
	root := t.TempDir()
	bad := filepath.Join(root, "AGENTS.md")
	os.WriteFile(bad, []byte("x"), 0600)
	os.Chmod(bad, 0000)
	m := New(root)
	if _, err := m.Bootstrap(); err != nil {
		t.Fatalf("unreadable convention file must not error: %v", err)
	}
	os.Chmod(bad, 0600)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/memory/ -run TestBootstrap -v`
Expected: FAIL — `Bootstrap` undefined.

- [ ] **Step 3: Write the implementation**

`internal/memory/bootstrap.go`:

```go
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Bootstrap struct {
	Convention []ConventionFile
	ProjectMD  string // PROJECT.md content, "" if absent
	MAPMD      string // MAP.md content, "" if absent (navigation is on-demand, never always-loaded)
}

// Bootstrap = Ensure + Detect + read PROJECT.md/MAP.md.
// Unreadable convention files are skipped, never an error (a broken
// rules file must not kill the session).
func (m *Memory) Bootstrap() (*Bootstrap, error) {
	if err := m.Ensure(); err != nil {
		return nil, err
	}
	b := &Bootstrap{Convention: m.Detect(m.root)}
	b.ProjectMD = readQuiet(filepath.Join(m.root, ".fender", "memory", "PROJECT.md"))
	b.MAPMD = readQuiet(filepath.Join(m.root, ".fender", "memory", "MAP.md"))
	return b, nil
}

func readQuiet(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// System composes the always-loaded layer: convention files (in precedence
// order) + PROJECT.md, provenance-marked. Capped at SystemCap — excess is
// truncated oldest-first with a marker (prevention over compression, D14).
func (b *Bootstrap) System() string {
	var sb strings.Builder
	for _, cf := range b.Convention {
		content := readQuiet(cf.Path)
		if content == "" {
			continue
		}
		fmt.Fprintf(&sb, "<<%s (%s): %s>>\n%s\n", cf.Kind, cf.Layer, cf.Path, strings.TrimSpace(content))
	}
	if b.ProjectMD != "" {
		sb.WriteString("\n<<PROJECT.md>>\n")
		sb.WriteString(strings.TrimSpace(b.ProjectMD))
	}
	if sb.Len() <= SystemCap {
		return sb.String()
	}
	// truncate oldest sections first: keep only the tail that fits, with a marker
	drop := sb.Len() - SystemCap
	marker := "\n[memory: earlier layers truncated — cap 8K]\n"
	kept := sb.String()[drop+len(marker):]
	return marker + kept
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/memory/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/memory/ CHANGELOG.md
git commit -m "feat: bootstrap + always-loaded System assembly (8K cap, canonical sources)"
```

CHANGELOG entry:

```markdown
### Added
- Bootstrap(): Ensure + Detect + layer reads; System() assembly with provenance markers, 8K cap (oldest-first truncation), unreadable files skipped
```

---

### Task 4: Working memory (prune + catalog)

**Files:**
- Modify: `internal/memory/bootstrap.go`
- Create: `internal/memory/working_test.go`

**Interfaces:**
- Produces:
  - Pruning runs inside `Bootstrap()`: `.fender/memory/working/*.md` with mtime older than `NotesMaxAge` (7 days) is removed; `patterns.md` is exempt.
  - `func (m *Memory) Working() []string` — sorted list of `"<basename>: <path> (<age>)"` for surviving working files.

- [ ] **Step 1: Write the failing test**

`internal/memory/working_test.go`:

```go
package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBootstrapPrunesStaleNotes(t *testing.T) {
	root := t.TempDir()
	working := filepath.Join(root, ".fender", "memory", "working")
	m := New(root)
	if err := m.Ensure(); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(working, "fresh.md")
	stale := filepath.Join(working, "stale.md")
	patterns := filepath.Join(working, "patterns.md")
	os.WriteFile(fresh, []byte("f"), 0600)
	os.WriteFile(stale, []byte("s"), 0600)
	os.WriteFile(patterns, []byte("p"), 0600)
	old := time.Now().Add(-NotesMaxAge - time.Hour)
	os.Chtimes(stale, old, old)
	os.Chtimes(patterns, old, old)

	if _, err := m.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal("fresh note removed")
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("stale note not pruned")
	}
	if _, err := os.Stat(patterns); err != nil {
		t.Fatal("patterns.md must never be pruned")
	}
}

func TestWorkingCatalog(t *testing.T) {
	root := t.TempDir()
	m := New(root)
	m.Ensure()
	working := filepath.Join(root, ".fender", "memory", "working")
	os.WriteFile(filepath.Join(working, "alpha.md"), []byte("a"), 0600)
	os.WriteFile(filepath.Join(working, "beta.md"), []byte("b"), 0600)
	m.Bootstrap()
	got := m.Working()
	if len(got) != 2 {
		t.Fatalf("catalog = %v", got)
	}
	if !strings.Contains(got[0], "alpha.md") || !strings.Contains(got[1], "beta.md") {
		t.Fatalf("catalog order/content = %v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/memory/ -run "TestBootstrapPrunes|TestWorking" -v`
Expected: FAIL — stale note not pruned (pruning not implemented yet).

- [ ] **Step 3: Implement pruning + catalog**

Append to `internal/memory/bootstrap.go`:

```go
// pruneWorking removes working/*.md older than NotesMaxAge. patterns.md is
// exempt — durable operational knowledge (mino patterns.md rule).
func (m *Memory) pruneWorking() {
	dir := filepath.Join(m.root, ".fender", "memory", "working")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "patterns.md" {
			continue
		}
		info, err := e.Info()
		if err != nil || time.Since(info.ModTime()) > NotesMaxAge {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// Working lists surviving working files: "<basename>: <path> (<age>)".
func (m *Memory) Working() []string {
	dir := filepath.Join(m.root, ".fender", "memory", "working")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		age := time.Since(info.ModTime()).Round(time.Hour)
		out = append(out, fmt.Sprintf("%s: %s (%s)", e.Name(), filepath.Join(dir, e.Name()), age))
	}
	return out
}
```

And hook pruning into `Bootstrap()` — add one line after `m.Ensure()`:

```go
func (m *Memory) Bootstrap() (*Bootstrap, error) {
	if err := m.Ensure(); err != nil {
		return nil, err
	}
	m.pruneWorking()
	b := &Bootstrap{Convention: m.Detect(m.root)}
	...
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/memory/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/memory/ CHANGELOG.md
git commit -m "feat: working memory — 7-day prune (patterns.md exempt), catalog"
```

CHANGELOG entry:

```markdown
### Added
- Working memory: pruneWorking (7-day, patterns.md exempt) in Bootstrap; Working() catalog
```

---

### Task 5: Agent wiring (nil-safe Mem)

**Files:**
- Modify: `internal/agent/agent.go`
- Create: `internal/agent/agent_memory_test.go`

**Interfaces:**
- Consumes: `memory.New`, `Memory.Bootstrap`, `Bootstrap.System` from Tasks 1–3.
- Produces:
  - `Agent` gains field `Mem *memory.Memory` (nil-safe).
  - `Run` start: `if a.Mem != nil { if b, err := a.Mem.Bootstrap(); err == nil { a.System = b.System() + a.System } }` — memory content prepends (constitution first, then task-specific).
  - `delegate.go`: child agents share `Mem` (same project memory — only artifact context is isolated per D38).

- [ ] **Step 1: Read the current wiring**

Read `internal/agent/agent.go` lines 30–80 and `internal/agent/delegate.go` first. Note where `Ctx` is handled and how the child Agent is constructed.

- [ ] **Step 2: Write the failing test**

`internal/agent/agent_memory_test.go`:

```go
package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/memory"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/tools"
)

func TestMemPrependsSystem(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("CONSTITUTION-RULES"), 0600)
	mem := memory.New(root)

	a := NewAgent(&stubLLM{reply: "done"}, tools.NewRegistry())
	a.System = "TASK-SPECIFIC"
	a.Mem = mem

	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "go"}})
	if res == nil {
		t.Fatal("nil result")
	}
	// stubLLM must have received the composed system prompt
	llm := a.llm.(*stubLLM)
	if !strings.Contains(llm.sentSystem, "CONSTITUTION-RULES") {
		t.Fatalf("system missing convention content: %q", llm.sentSystem)
	}
	if !strings.Contains(llm.sentSystem, "TASK-SPECIFIC") {
		t.Fatalf("system missing task-specific tail: %q", llm.sentSystem)
	}
	if !strings.HasPrefix(llm.sentSystem, "CONSTITUTION-RULES") {
		t.Fatalf("memory must prepend: %q", llm.sentSystem)
	}
}

func TestMemNilUnchanged(t *testing.T) {
	a := NewAgent(&stubLLM{reply: "done"}, tools.NewRegistry())
	a.System = "ONLY-THIS"
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "go"}})
	if got := a.llm.(*stubLLM).sentSystem; got != "ONLY-THIS" {
		t.Fatalf("nil Mem changed behavior: %q", got)
	}
}

// stubLLM records the system message it received.
type stubLLM struct {
	reply      string
	sentSystem string
}

func (s *stubLLM) Create(ctx context.Context, req provider.Request) (*provider.Response, error) {
	for _, m := range req.Messages {
		if m.Role == "system" {
			s.sentSystem = m.Content
		}
	}
	return &provider.Response{Choices: []provider.Choice{{Message: provider.Message{Role: "assistant", Content: s.reply}}}}, nil
}
```

Note: check how existing agent tests fake the LLM — if a `stubLLM` already exists in `agent_test.go`, reuse its name/pattern instead of defining a duplicate (adjust the test above to match the existing fake). The essential assertions: memory content prepends, nil Mem = unchanged behavior.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestMem -v`
Expected: FAIL — `Mem` field doesn't exist.

- [ ] **Step 4: Wire Mem into the agent**

In `internal/agent/agent.go`:

```go
type Agent struct {
	// ...existing fields...
	System string
	Ctx    *context.Manager // nil-safe (ticket 04)
	Mem    *memory.Memory   // nil-safe (ticket 05) — project memory workspace
}
```

In `Run`, immediately before the existing Ctx handling (or after — but before the system-prepend):

```go
	if a.Mem != nil {
		if b, err := a.Mem.Bootstrap(); err == nil {
			a.System = b.System() + a.System // constitution first, then task-specific
		}
	}
```

In `internal/agent/delegate.go`, child construction: copy the parent's Mem (shared project memory — delegates work the same codebase):

```go
// inside the child Agent literal:
Mem: a.Mem,
```

- [ ] **Step 4b: Run the full suite**

Run: `go test ./...`
Expected: all PASS — including all existing agent/context tests unchanged (nil-safe).

- [ ] **Step 5: Commit**

```bash
git add internal/agent/ CHANGELOG.md
git commit -m "feat: agent memory wiring (nil-safe Mem, system prepend, delegates share)"
```

CHANGELOG entry:

```markdown
### Added
- Agent wiring: nil-safe Mem — Bootstrap at Run start, memory system prepend, delegates share project memory
```

---

### Task 6: Wayfinder resolve + frontier

**Files:**
- Modify: `.scratch/fender/issues/05-Memory.md`
- Modify: `.scratch/fender/map.md`

- [ ] **Step 1: Full verification**

```bash
go build ./... && go vet ./... && go test ./...
```

Expected: build clean, vet clean, all tests PASS.

- [ ] **Step 2: Resolve the ticket**

Update `.scratch/fender/issues/05-Memory.md`:

```markdown
# 05-Memory

Type: task
Status: resolved
Blocked by: 04
Resolved: <date>

## Question

Write + execute Plan 5: memory = ICM layers (D14, D17) — PROJECT.md always, MAP.md from code-intel, reference/ + working/ selective. Convention files (AGENTS.md/CLAUDE.md/CONTEXT.md) load DIRECTLY, never copied into PROJECT.md. Memory graph.

## Answer

Plan 5 done: internal/memory — Ensure scaffold, Detect (user→project AGENTS.md/CLAUDE.md→CONTEXT.md), Bootstrap + System (8K cap, canonical sources), working prune (7d, patterns.md exempt). Agent Mem wired nil-safe; delegates share. Memory graph + consolidation + SQLite facts deferred to D9 era (spec approved 2026-08-04). Unblocks 06.
```

- [ ] **Step 3: Update the map's decisions index**

In `.scratch/fender/map.md`, under "Decisions so far", append:

```markdown
- [05-Memory](issues/05-Memory.md) — Plan 5 done: .fender/ workspace, convention detection + direct load, always-loaded System (8K), working prune; memory graph/consolidation/facts deferred to D9 era (user-approved). Unblocks 06.
```

- [ ] **Step 4: Commit**

```bash
git add .scratch/fender/ CHANGELOG.md
git commit -m "docs: resolve wayfinder ticket 05 (memory done, frontier 06)"
```

CHANGELOG entry:

```markdown
### Changed
- Wayfinder: ticket 05 resolved — memory/ICM layers delivered; frontier → 06 (Skills)
```

---

## Self-Review Notes

- **Spec coverage:** §1 scope (1–5) → Tasks 1–5; §3 decisions 1–6 → Tasks 1–3 + agent wiring; §4 API → Tasks 1–4; §5 wiring → Task 5; §6 test table → each task's tests by name; §7 acceptance criteria → Task 6 verification. Deferred items (§2, §8) are explicitly NOT built.
- **Placeholders:** none — every code step contains full source. Task 5 Step 1 tells the implementer to read the actual wiring first and adjust the stub name to the existing test fake (only legitimate adaptation point — existing tests own the name).
- **Type consistency:** `Memory.New`/`Bootstrap`/`System`/`Working` signatures match across tasks; `ConventionFile{Path,Kind,Layer}` defined once in Task 2, consumed in Task 3. `Agent.Mem *memory.Memory` added in Task 5, used in delegate.
- **CHANGELOG:** every task ends with an entry + commit (hook-enforced).
- **Deps:** none added — stdlib only.
