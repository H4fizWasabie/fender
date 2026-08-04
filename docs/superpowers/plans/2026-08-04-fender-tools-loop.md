# Fender Plan 3: Tools + Agent Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The heart of the agent (D10, D13, D36, D37): four tools (read_file, edit_file, shell, search) and the ONE agent loop — flat by default, `complete_task` completion protocol, in-turn tool dedup, ONE orientation turn on thrash, subagent-as-a-tool (same Agent type in a goroutine). Wire types consume the provider layer from Plan 1 and the guardrail from Plan 2.

**Architecture:** `internal/tools` is a pure registry of `Tool{Name, Description, Parameters, Call}` with JSON-arg dispatch and a `Searcher` function-type seam (D10). The shell tool is the guardrail's execution point: `Judge` → verdict → REFUSE error / ASK via injectable `Approver` / RUN with timeout, every command audited. `internal/agent` holds one `Agent{LLM, Tools}` type running mino's flat loop (D37): LLM call → tool calls → results → repeat, terminated only by `complete_task` (status complete|blocked + reply), stalled on no-progress, deduped per run (D32 layer 1). On thrash (tool errors, repeated same call, text-only iterations) the loop injects ONE explicit orientation turn (D36). The `delegate` tool spawns a child `Agent` in a goroutine (D13) with its own provider via a `Resolver` (D7).

**Tech Stack:** Go 1.22, stdlib only additions (`os/exec`, `bufio`, `filepath`). Consumes `mvdan.cc/sh/v3` (already pinned v3.10.0), `internal/guardrail`, `internal/provider`. Verified during planning: mvdan's `NewParser()` defaults to the Bash grammar, so fork bombs, `[[ ]]`, and process substitution parse and judge correctly — no parser change is needed; the shell tool runs `bash -c` consistent with judgment.

## Global Constraints

- **Read `AGENTS.md`, `DECISIONS.md`, and the design spec first.** They are the constitution (D1–D37).
- **Every commit MUST stage `CHANGELOG.md`** with a `[Unreleased]` entry — enforced by `.githooks/pre-commit`.
- **Allowed dependencies:** `BurntSushi/toml`, `mvdan.cc/sh/v3`, `go-tree-sitter`, `modernc.org/sqlite`. Nothing else. This plan adds zero dependencies.
- **REFUSE is hard in all modes** (D22): the shell tool must never execute a REFUSE verdict, even in yolo.
- **No panic in library code.** Explicit errors, `log/slog` for logging.
- **No new abstractions with one implementation** (ponytail rule). The one interface this plan adds is `agent.LLM` — justified because the loop needs a scripted fake for tests (mino does the same with `LLMClient`).
- Module path: `github.com/H4fizWasabie/fender`. File layout: `internal/tools/`, `internal/agent/` — flat over nested.

**Known v1 limitations (documented, not fixed):**
- read/edit containment is lexical (`filepath.Clean` + prefix); symlink escapes are not followed. `ponytail:` comment in `internal/tools/paths.go`.
- ASK with no `Approver` configured = denied. The interactive approval prompt is Plan 8 (CLI/UI); the seam is `ShellConfig.Approver`.
- Output caps (read 1 MiB, shell 64 KiB, search 50 matches) are interim; Plan 4 (context/artifact engineering, D31) replaces them with artifact pointers.
- Subagent nesting is one level: children get the parent's registry minus `delegate`.
- The dedup cache lives for the whole Run (not per turn) — matches mino; it is what makes repeated identical calls visible to orientation detection.

---

### Task 1: Tools core (Tool, Registry, path containment)

**Files:**
- Create: `internal/tools/tools.go`
- Create: `internal/tools/paths.go`
- Create: `internal/tools/tools_test.go`

**Interfaces:**
- Consumes: `provider.ToolDef`, `provider.ToolFunctionDef` (Plan 1).
- Produces:
  - `type Tool struct { Name, Description string; Parameters map[string]any; Call func(ctx context.Context, args map[string]any) (string, error) }`
  - `type Registry struct` with `func (r *Registry) Add(t Tool)`, `func (r *Registry) Without(exclude ...string) *Registry`, `func (r *Registry) Names() []string`, `func (r *Registry) Schemas() []provider.ToolDef`, `func (r *Registry) Execute(ctx context.Context, name, argsJSON string) (string, error)`
  - `func inProject(projectDir, p string) (string, error)` — resolve + contain (unexported)
  - `func New(...)` arrives in Task 4, once all four tools exist.

- [ ] **Step 1: Write the failing test**

`internal/tools/tools_test.go`:

```go
package tools

import (
	"context"
	"strings"
	"testing"
)

func TestRegistryExecute(t *testing.T) {
	reg := &Registry{tools: map[string]Tool{}}
	reg.Add(Tool{
		Name:        "echo",
		Description: "echo back",
		Parameters:  map[string]any{"type": "object"},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			return "hi", nil
		},
	})
	out, err := reg.Execute(context.Background(), "echo", "{}")
	if err != nil || out != "hi" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if _, err := reg.Execute(context.Background(), "nope", "{}"); err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if _, err := reg.Execute(context.Background(), "echo", "{bad json"); err == nil {
		t.Fatal("expected error for malformed args")
	}
}

func TestRegistrySchemas(t *testing.T) {
	reg := &Registry{tools: map[string]Tool{}}
	reg.Add(Tool{
		Name:        "echo",
		Description: "echo back",
		Parameters:  map[string]any{"type": "object"},
		Call:        func(ctx context.Context, args map[string]any) (string, error) { return "", nil },
	})
	schemas := reg.Schemas()
	if len(schemas) != 1 {
		t.Fatalf("schemas = %d", len(schemas))
	}
	s := schemas[0]
	if s.Type != "function" || s.Function.Name != "echo" || s.Function.Description == "" || s.Function.Parameters == nil {
		t.Fatalf("schema = %+v", s)
	}
}

func TestRegistryWithout(t *testing.T) {
	reg := &Registry{tools: map[string]Tool{}}
	for _, n := range []string{"a", "b", "c"} {
		reg.Add(Tool{Name: n, Call: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }})
	}
	sub := reg.Without("b")
	if got := strings.Join(sub.Names(), ","); got != "a,c" {
		t.Fatalf("sub = %q", got)
	}
	if got := strings.Join(reg.Names(), ","); got != "a,b,c" {
		t.Fatalf("original mutated: %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tools/ -v`
Expected: FAIL — package `tools` has no files (build failure).

- [ ] **Step 3: Write the core**

`internal/tools/tools.go`:

```go
// Package tools implements the agent's tool set (D10): read_file,
// edit_file, shell, search — with a backend seam for codebase search.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/H4fizWasabie/fender/internal/provider"
)

// Tool is one callable tool: a JSON schema + a handler.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON schema object
	Call        func(ctx context.Context, args map[string]any) (string, error)
}

// Registry holds the tools of one agent. Subagents get a Registry too
// (minus delegate), so guardrails wrap tool execution once (D13).
type Registry struct {
	tools map[string]Tool
	order []string
}

func (r *Registry) Add(t Tool) {
	r.tools[t.Name] = t
	r.order = append(r.order, t.Name)
}

// Without returns a shallow copy minus the named tools (subagent subsets).
func (r *Registry) Without(exclude ...string) *Registry {
	skip := make(map[string]bool, len(exclude))
	for _, n := range exclude {
		skip[n] = true
	}
	c := &Registry{tools: make(map[string]Tool, len(r.tools))}
	for _, n := range r.order {
		if skip[n] {
			continue
		}
		c.tools[n] = r.tools[n]
		c.order = append(c.order, n)
	}
	return c
}

func (r *Registry) Names() []string { return r.order }

// Schemas converts the registry to OpenAI tool definitions.
func (r *Registry) Schemas() []provider.ToolDef {
	out := make([]provider.ToolDef, 0, len(r.order))
	for _, n := range r.order {
		t := r.tools[n]
		out = append(out, provider.ToolDef{
			Type: "function",
			Function: provider.ToolFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return out
}

// Execute runs one tool by name with JSON-encoded args.
func (r *Registry) Execute(ctx context.Context, name, argsJSON string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q (have: %v)", name, r.order)
	}
	var args map[string]any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("%s: bad args: %v", name, err)
		}
	}
	return t.Call(ctx, args)
}
```

`internal/tools/paths.go`:

```go
package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// inProject resolves p against projectDir and verifies the result stays
// inside the project. ponytail: symlink escape is not followed; a later
// pass can EvalSymlinks before the prefix check.
func inProject(projectDir, p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(projectDir, p)
	}
	abs = filepath.Clean(abs)
	proj := filepath.Clean(projectDir)
	if abs != proj && !strings.HasPrefix(abs, proj+"/") {
		return "", fmt.Errorf("path %q is outside the project directory %q", p, projectDir)
	}
	return abs, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tools/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tools/ CHANGELOG.md
git commit -m "feat: tools core (Tool, Registry, schemas, execute, subagent subsets)"
```

CHANGELOG entry:

```markdown
### Added
- Tools core: Tool/Registry (JSON-arg dispatch, OpenAI schemas, subagent subsets via Without) + project path containment (D10)
```

---

### Task 2: read_file + edit_file tools

**Files:**
- Create: `internal/tools/read.go`
- Create: `internal/tools/read_test.go`
- Create: `internal/tools/edit.go`
- Create: `internal/tools/edit_test.go`

**Interfaces:**
- Consumes: `Tool`, `Registry`, `inProject` from Task 1.
- Produces:
  - `func readTool(projectDir string) Tool` — tool name `read_file`, args `{path string, offset int, limit int}` (offset/limit 1-based, optional; this is the D31 seam for slice fetches)
  - `func editTool(projectDir string) Tool` — tool name `edit_file`, args `{path, old_text, new_text string}`; exact-match replace, errors on 0 or >1 matches

- [ ] **Step 1: Write the failing tests**

`internal/tools/read_test.go`:

```go
package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFile(t *testing.T) {
	proj := t.TempDir()
	os.WriteFile(filepath.Join(proj, "a.txt"), []byte("line1\nline2\nline3\n"), 0o644)
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "read_file", `{"path":"a.txt"}`)
	if err != nil || out != "line1\nline2\nline3\n" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	out, err = reg.Execute(context.Background(), "read_file", `{"path":"a.txt","offset":2,"limit":1}`)
	if err != nil || out != "line2" {
		t.Fatalf("slice out=%q err=%v", out, err)
	}
	out, err = reg.Execute(context.Background(), "read_file", `{"path":"a.txt","offset":9}`)
	if err != nil || out != "" {
		t.Fatalf("past-eof out=%q err=%v", out, err)
	}
	if _, err := reg.Execute(context.Background(), "read_file", `{"path":"missing.txt"}`); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadFileContainment(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	for _, path := range []string{"../outside.txt", "/etc/passwd", ""} {
		if _, err := reg.Execute(context.Background(), "read_file", `{"path":"`+path+`"}`); err == nil {
			t.Fatalf("expected containment error for %q", path)
		}
	}
}
```

`internal/tools/edit_test.go`:

```go
package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditFile(t *testing.T) {
	proj := t.TempDir()
	path := filepath.Join(proj, "a.txt")
	os.WriteFile(path, []byte("foo bar baz\n"), 0o644)
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "edit_file", `{"path":"a.txt","old_text":"bar","new_text":"BAR"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "edited") {
		t.Fatalf("out = %q", out)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "foo BAR baz\n" {
		t.Fatalf("file = %q", data)
	}
}

func TestEditFileErrors(t *testing.T) {
	proj := t.TempDir()
	os.WriteFile(filepath.Join(proj, "a.txt"), []byte("x x\n"), 0o644)
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	if _, err := reg.Execute(context.Background(), "edit_file", `{"path":"a.txt","old_text":"zz","new_text":"y"}`); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want not-found error, got %v", err)
	}
	if _, err := reg.Execute(context.Background(), "edit_file", `{"path":"a.txt","old_text":"x","new_text":"y"}`); err == nil || !strings.Contains(err.Error(), "times") {
		t.Fatalf("want ambiguity error, got %v", err)
	}
	if _, err := reg.Execute(context.Background(), "edit_file", `{"path":"../x","old_text":"a","new_text":"b"}`); err == nil {
		t.Fatal("want containment error")
	}
}
```

Note: these tests call `New(proj, ShellConfig{...}, nil)` — the final signature. Task 2 Step 3 therefore lands the `ShellConfig`, `Searcher`, and `SearchResult` TYPES (definitions only; implementations arrive in Tasks 3 and 4) so the whole plan's signature is stable from here on.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tools/ -v`
Expected: FAIL — `New` and `readTool` undefined (build failure).

- [ ] **Step 3: Write the tools**

`internal/tools/read.go`:

```go
package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// readCap is the v1 hard ceiling for read output. Plan 4 (artifact
// engineering, D31) replaces the cap with artifact pointers.
const readCap = 1 << 20 // 1 MiB

func readTool(projectDir string) Tool {
	return Tool{
		Name:        "read_file",
		Description: "Read a file inside the project directory. Optional 1-based offset and limit select a line range; the full file is read when omitted.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string", "description": "Path to the file (absolute or relative to the project root)"},
				"offset": map[string]any{"type": "integer", "description": "First line to return (1-based)"},
				"limit":  map[string]any{"type": "integer", "description": "Maximum number of lines to return"},
			},
			"required": []string{"path"},
		},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			full, err := inProject(projectDir, path)
			if err != nil {
				return "", err
			}
			data, err := os.ReadFile(full)
			if err != nil {
				return "", err
			}
			if len(data) > readCap {
				data = data[:readCap]
				return string(data) + fmt.Sprintf("\n... (truncated at %d bytes)\n", readCap), nil
			}
			text := string(data)
			off, hasOff := intArg(args, "offset")
			if !hasOff {
				return text, nil
			}
			lines := strings.Split(text, "\n")
			if off < 1 {
				return "", fmt.Errorf("read_file: offset must be >= 1")
			}
			if off > len(lines) {
				return "", nil
			}
			end := len(lines)
			if lim, ok := intArg(args, "limit"); ok {
				if lim < 1 {
					return "", fmt.Errorf("read_file: limit must be >= 1")
				}
				if off-1+lim < end {
					end = off - 1 + lim
				}
			}
			return strings.Join(lines[off-1:end], "\n"), nil
		},
	}
}

// intArg reads an integer tool argument (JSON numbers decode as float64).
func intArg(args map[string]any, key string) (int, bool) {
	switch n := args[key].(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}
```

`internal/tools/edit.go`:

```go
package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func editTool(projectDir string) Tool {
	return Tool{
		Name:        "edit_file",
		Description: "Replace a unique occurrence of old_text with new_text in a file inside the project directory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":     map[string]any{"type": "string"},
				"old_text": map[string]any{"type": "string", "description": "Exact text to replace; must occur exactly once"},
				"new_text": map[string]any{"type": "string"},
			},
			"required": []string{"path", "old_text", "new_text"},
		},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			oldText, _ := args["old_text"].(string)
			newText, _ := args["new_text"].(string)
			full, err := inProject(projectDir, path)
			if err != nil {
				return "", err
			}
			if oldText == "" {
				return "", fmt.Errorf("edit_file: old_text must not be empty")
			}
			data, err := os.ReadFile(full)
			if err != nil {
				return "", err
			}
			content := string(data)
			switch n := strings.Count(content, oldText); n {
			case 0:
				return "", fmt.Errorf("edit_file: old_text not found in %s", path)
			case 1:
				// ok
			default:
				return "", fmt.Errorf("edit_file: old_text occurs %d times in %s; include more context", n, path)
			}
			info, err := os.Stat(full)
			if err != nil {
				return "", err
			}
			out := strings.Replace(content, oldText, newText, 1)
			if err := os.WriteFile(full, []byte(out), info.Mode().Perm()); err != nil {
				return "", err
			}
			return fmt.Sprintf("edited %s (%d -> %d bytes)", path, len(content), len(out)), nil
		},
	}
}
```

`New` with the final signature — add to `internal/tools/tools.go` (Tasks 3 and 4 extend the body with shellTool and searchTool):

```go
// New returns the standard v1 registry. Tasks 3 and 4 extend it with the
// shell and search tools.
func New(projectDir string, shell ShellConfig, searcher Searcher) *Registry {
	r := &Registry{tools: make(map[string]Tool)}
	r.Add(readTool(projectDir))
	r.Add(editTool(projectDir))
	return r
}
```

`internal/tools/shell.go` — the `ShellConfig` type only (implementation lands in Task 3):

```go
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
```

`internal/tools/search.go` — the seam types only (implementation lands in Task 4):

```go
package tools

// Searcher is the codebase-search backend seam (D10): graphify/cce/codegraph
// plug in here later; v1 ships the walk-based default (Task 4).
type Searcher func(query string) ([]SearchResult, error)

// SearchResult is one match: file, 1-based line, matching text.
type SearchResult struct {
	Path string
	Line int
	Text string
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tools/ -v`
Expected: all PASS (registry, read, edit suites).

- [ ] **Step 5: Commit**

```bash
git add internal/tools/ CHANGELOG.md
git commit -m "feat: read_file and edit_file tools (containment, line slices, exact-match replace)"
```

CHANGELOG entry:

```markdown
### Added
- read_file tool (1-based offset/limit line slices, project containment) + edit_file tool (unique exact-match replace, mode-preserving) (D10)
```

---

### Task 3: shell tool — guardrail wiring, approval, timeout, audit

**Files:**
- Create: `internal/tools/shell.go`
- Create: `internal/tools/shell_test.go`
- Modify: `internal/tools/tools.go` (extend `New` with the shell tool)

**Interfaces:**
- Consumes: `guardrail.Judge`, `guardrail.Audit`, `guardrail.DefaultTimeout`, `guardrail.Mode`, `guardrail.Verdict` (Plan 2).
- Produces:
  - `type ShellConfig struct { Mode guardrail.Mode; ProjectDir string; Audit *guardrail.Audit; Timeout time.Duration; Approver func(cmd, reason string) (bool, error) }` — `Timeout` 0 → `guardrail.DefaultTimeout`; `Approver` nil → ASK is denied
  - `func shellTool(cfg ShellConfig) Tool` — tool name `shell`, args `{command string}`; runs `bash -c` with `Dir = ProjectDir`, combined output capped at 64 KiB, verdict always audited

- [ ] **Step 1: Write the failing test**

`internal/tools/shell_test.go`:

```go
package tools

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/H4fizWasabie/fender/internal/guardrail"
)

func TestShellRun(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "shell", `{"command":"echo hi"}`)
	if err != nil || out != "hi\n" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestShellRefuseNeverRuns(t *testing.T) {
	proj := t.TempDir()
	called := false
	reg := New(proj, ShellConfig{
		Mode:       guardrail.Yolo, // REFUSE is hard in all modes (D22)
		ProjectDir: proj,
		Approver: func(cmd, reason string) (bool, error) {
			called = true
			return true, nil
		},
	}, nil)
	_, err := reg.Execute(context.Background(), "shell", `{"command":"rm -rf /"}`)
	if err == nil || !strings.Contains(err.Error(), "REFUSED") {
		t.Fatalf("want REFUSED, got %v", err)
	}
	if called {
		t.Fatal("approver must not be consulted for REFUSE")
	}
}

func TestShellAskApproved(t *testing.T) {
	proj := t.TempDir()
	approved := false
	reg := New(proj, ShellConfig{
		Mode:       guardrail.Balanced,
		ProjectDir: proj,
		Approver: func(cmd, reason string) (bool, error) {
			approved = true
			return true, nil
		},
	}, nil)
	if _, err := reg.Execute(context.Background(), "shell", `{"command":"tee /tmp/fender-test-ask"}`); err != nil {
		t.Fatalf("approved ASK should run: %v", err)
	}
	if !approved {
		t.Fatal("approver not called")
	}
}

func TestShellAskDenied(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{
		Mode:       guardrail.Balanced,
		ProjectDir: proj,
		Approver:   func(cmd, reason string) (bool, error) { return false, nil },
	}, nil)
	_, err := reg.Execute(context.Background(), "shell", `{"command":"tee /tmp/fender-test-deny"}`)
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("want denied, got %v", err)
	}
}

func TestShellAskNoApproverDenies(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	_, err := reg.Execute(context.Background(), "shell", `{"command":"tee /tmp/fender-test-noappr"}`)
	if err == nil || !strings.Contains(err.Error(), "approval") {
		t.Fatalf("want approval error, got %v", err)
	}
}

func TestShellTimeout(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj, Timeout: 100 * time.Millisecond}, nil)
	_, err := reg.Execute(context.Background(), "shell", `{"command":"sleep 5"}`)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("want timeout, got %v", err)
	}
}

func TestShellAuditsEveryCommand(t *testing.T) {
	proj := t.TempDir()
	var buf bytes.Buffer
	reg := New(proj, ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj, Audit: guardrail.NewAudit(&buf)}, nil)
	if _, err := reg.Execute(context.Background(), "shell", `{"command":"echo hi"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Execute(context.Background(), "shell", `{"command":"rm -rf /"}`); err == nil {
		t.Fatal("expected REFUSE")
	}
	if got := strings.Count(buf.String(), "\n"); got != 2 {
		t.Fatalf("audit lines = %d: %q", got, buf.String())
	}
}

func TestShellWorksInProjectDir(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "shell", `{"command":"pwd"}`)
	if err != nil || strings.TrimSpace(out) != proj {
		t.Fatalf("out=%q err=%v (want cwd=%q)", out, err, proj)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tools/ -run TestShell -v`
Expected: FAIL — `ShellConfig`/`shellTool` undefined.

- [ ] **Step 3: Write the shell tool**

`internal/tools/shell.go` — append to the existing file (the `ShellConfig` type from Task 2 stays):

```go
package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/H4fizWasabie/fender/internal/guardrail"
)

// outputCap is the v1 ceiling for command output returned to the model.
// Plan 4 (artifact engineering, D31) replaces it with artifact pointers.
const outputCap = 64 << 10 // 64 KiB

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
				ok, err := cfg.Approver(cmd, reason)
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
```

Extend `New` in `internal/tools/tools.go` — add one line:

```go
	r.Add(shellTool(shell))
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tools/ -v`
Expected: all PASS (registry, read, edit, shell suites). Note: `tee /tmp/...` is ASK in balanced (path escape) and runs with stdin from /dev/null — exits 0.

- [ ] **Step 5: Commit**

```bash
git add internal/tools/ CHANGELOG.md
git commit -m "feat: shell tool (guardrail verdicts, approver, timeout, audit)"
```

CHANGELOG entry:

```markdown
### Added
- shell tool: Judge verdict wiring (REFUSE hard in all modes, ASK via injectable approver, RUN), audit every command, default 60s timeout, 64 KiB output cap (D11, D12, D24)
```

---

### Task 4: search tool — walk-based backend behind the Searcher seam

**Files:**
- Create: `internal/tools/search.go`
- Create: `internal/tools/search_test.go`
- Modify: `internal/tools/tools.go` (extend `New` with the search tool)

**Interfaces:**
- Consumes: `Searcher`, `SearchResult` (types landed in Task 2 Step 4), `inProject`.
- Produces:
  - `func DefaultSearcher(projectDir string) Searcher` — walk-based, case-insensitive substring, skips `.git`/`.fender`/`.scratch`/`node_modules`/`vendor`/`dist`/`build`/`graphify-out` dirs and binary/archive extensions, NUL-byte binary sniff, max 50 results
  - `func searchTool(projectDir string, searcher Searcher) Tool` — tool name `search`, args `{query string}`; nil searcher → `DefaultSearcher(projectDir)`

- [ ] **Step 1: Write the failing test**

`internal/tools/search_test.go`:

```go
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearch(t *testing.T) {
	proj := t.TempDir()
	os.WriteFile(filepath.Join(proj, "a.go"), []byte("func Foo() {}\n// Foo is here\n"), 0o644)
	os.WriteFile(filepath.Join(proj, "b.txt"), []byte("no match here\n"), 0o644)
	os.MkdirAll(filepath.Join(proj, ".git"), 0o755)
	os.WriteFile(filepath.Join(proj, ".git", "config"), []byte("Foo\n"), 0o644)
	os.WriteFile(filepath.Join(proj, "bin.dat"), []byte("\x00needle\n"), 0o644)
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "search", `{"query":"foo"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a.go:1: func Foo() {}") || !strings.Contains(out, "a.go:2: // Foo is here") {
		t.Fatalf("out = %q", out)
	}
	if strings.Contains(out, ".git") {
		t.Fatalf(".git not skipped: %q", out)
	}
	if strings.Contains(out, "bin.dat") {
		t.Fatalf("binary not skipped: %q", out)
	}
	if strings.Contains(out, "b.txt") {
		t.Fatalf("non-match leaked: %q", out)
	}
}

func TestSearchNoMatches(t *testing.T) {
	proj := t.TempDir()
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "search", `{"query":"zzz_nothing_zzz"}`)
	if err != nil || out != "no matches" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestSearchCap(t *testing.T) {
	proj := t.TempDir()
	var sb strings.Builder
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&sb, "needle line %d\n", i)
	}
	os.WriteFile(filepath.Join(proj, "big.txt"), []byte(sb.String()), 0o644)
	reg := New(proj, ShellConfig{ProjectDir: proj}, nil)
	out, err := reg.Execute(context.Background(), "search", `{"query":"needle"}`)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, "needle"); n != maxSearchResults {
		t.Fatalf("matches = %d (want %d)", n, maxSearchResults)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tools/ -run TestSearch -v`
Expected: FAIL — `DefaultSearcher`/`searchTool` undefined.

- [ ] **Step 3: Write the search tool**

`internal/tools/search.go` — append to the existing file (the `Searcher`/`SearchResult` types from Task 2 stay):

```go
package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const maxSearchResults = 50

var searchSkipDirs = map[string]bool{
	".git": true, ".fender": true, ".scratch": true, "node_modules": true,
	"vendor": true, "dist": true, "build": true, "graphify-out": true,
}

var searchSkipExts = map[string]bool{
	".lock": true, ".sum": true, ".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".webp": true, ".ico": true, ".pdf": true, ".zip": true,
	".tar": true, ".gz": true, ".so": true, ".a": true, ".exe": true,
}

// DefaultSearcher walks projectDir, skipping build/vendor dirs and binary
// files, and returns case-insensitive substring matches (max 50).
func DefaultSearcher(projectDir string) Searcher {
	return func(query string) ([]SearchResult, error) {
		if query == "" {
			return nil, fmt.Errorf("search: empty query")
		}
		q := strings.ToLower(query)
		var out []SearchResult
		err := filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if d.IsDir() {
				if path != projectDir && searchSkipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if searchSkipExts[strings.ToLower(filepath.Ext(d.Name()))] {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			head := make([]byte, 8192)
			n, _ := f.Read(head)
			if bytes.Contains(head[:n], []byte{0}) {
				return nil // binary sniff
			}
			if _, err := f.Seek(0, 0); err != nil {
				return nil
			}
			sc := bufio.NewScanner(f)
			line := 0
			for sc.Scan() {
				line++
				if strings.Contains(strings.ToLower(sc.Text()), q) {
					out = append(out, SearchResult{Path: path, Line: line, Text: sc.Text()})
					if len(out) >= maxSearchResults {
						return filepath.SkipAll
					}
				}
			}
			return nil
		})
		if err != nil && err != fs.SkipAll {
			return nil, err
		}
		return out, nil
	}
}

func searchTool(projectDir string, searcher Searcher) Tool {
	if searcher == nil {
		searcher = DefaultSearcher(projectDir)
	}
	return Tool{
		Name:        "search",
		Description: "Case-insensitive substring search over project files (skips .git, vendored, and binary files). Returns up to 50 matches as path:line: text.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
			},
			"required": []string{"query"},
		},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			query, _ := args["query"].(string)
			results, err := searcher(query)
			if err != nil {
				return "", err
			}
			if len(results) == 0 {
				return "no matches", nil
			}
			var sb strings.Builder
			for _, r := range results {
				fmt.Fprintf(&sb, "%s:%d: %s\n", r.Path, r.Line, r.Text)
			}
			return strings.TrimSuffix(sb.String(), "\n"), nil
		},
	}
}
```

Extend `New` in `internal/tools/tools.go` — add one line:

```go
	r.Add(searchTool(projectDir, searcher))
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tools/ -v`
Expected: all PASS.

- [ ] **Step 5: Full package verification + commit**

```bash
go test ./... && go vet ./...
git add internal/tools/ CHANGELOG.md
git commit -m "feat: search tool (walk-based default backend behind Searcher seam)"
```

CHANGELOG entry:

```markdown
### Added
- search tool: walk-based default backend (skips .git/vendor/binary, 50-match cap) behind the Searcher seam (D10)
```

---

### Task 5: Agent loop core — flat loop, complete_task protocol, dedup

**Files:**
- Create: `internal/agent/agent.go`
- Create: `internal/agent/complete.go`
- Create: `internal/agent/agent_test.go`
- Modify: `internal/provider/client.go` (default `req.Model` to the client's model when empty — the loop sends no model)
- Modify: `internal/provider/client_test.go` (model-default tests)

**Interfaces:**
- Consumes: `tools.Registry`, `provider.Client`, `provider.Message`, `provider.Request`, `provider.Response`, `provider.ToolDef`.
- Produces:
  - `type LLM interface { Chat(ctx context.Context, req provider.Request) (*provider.Response, error) }` — satisfied by `*provider.Client`; the fake in tests is the second implementation that justifies the interface
  - `type Agent struct { LLM LLM; System string; MaxIter int; registry *tools.Registry }` with `func NewAgent(llm LLM, reg *tools.Registry) *Agent` (delegate wiring arrives in Task 7)
  - `type Result struct { Reply, Status string; Iterations int }` — Status ∈ `complete | blocked | stalled | error | cancelled`
  - `func (a *Agent) Run(ctx context.Context, msgs []provider.Message) *Result`
  - `const completionToolName = "complete_task"`; `func completeSchema() provider.ToolDef`; `func completionArgs(argsJSON string) (status, reply string)`; `func canonicalArgs(argsJSON string) string`
  - `provider.Client.Chat`/`Stream`: `if req.Model == "" { req.Model = c.model }`

- [ ] **Step 1: Write the failing tests**

`internal/provider/client_test.go` — add:

```go
func TestChatDefaultsModel(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		gotModel, _ = body["model"].(string)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer srv.Close()
	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", DefaultModel: "m1"})
	if _, err := c.Chat(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}}); err != nil {
		t.Fatal(err)
	}
	if gotModel != "m1" {
		t.Fatalf("model = %q", gotModel)
	}
}

func TestStreamDefaultsModel(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		gotModel, _ = body["model"].(string)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", DefaultModel: "m1"})
	if _, err := c.Stream(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if gotModel != "m1" {
		t.Fatalf("model = %q", gotModel)
	}
}
```

`internal/agent/agent_test.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/H4fizWasabie/fender/internal/guardrail"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/tools"
)

// fakeLLM is a scripted LLM for loop tests: each Chat consumes one scripted
// response and records the request.
type fakeLLM struct {
	mu    sync.Mutex
	reqs  []provider.Request
	steps []*provider.Response
}

func (f *fakeLLM) Chat(ctx context.Context, req provider.Request) (*provider.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, req)
	if len(f.steps) == 0 {
		return &provider.Response{Choices: []provider.Choice{{Message: provider.Message{Role: "assistant", Content: "(script exhausted)"}}}}, nil
	}
	r := f.steps[0]
	f.steps = f.steps[1:]
	return r, nil
}

func (f *fakeLLM) last() provider.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reqs[len(f.reqs)-1]
}

func (f *fakeLLM) all() []provider.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]provider.Request(nil), f.reqs...)
}

func textReply(s string) *provider.Response {
	return &provider.Response{Choices: []provider.Choice{{Message: provider.Message{Role: "assistant", Content: s}}}}
}

func toolReply(id, name, args string) *provider.Response {
	return &provider.Response{Choices: []provider.Choice{{Message: provider.Message{
		Role: "assistant",
		ToolCalls: []provider.ToolCall{{
			ID: id, Type: "function",
			Function: provider.ToolFunction{Name: name, Arguments: args},
		}},
	}}}}
}

func completeReply(status, reply string) *provider.Response {
	args, _ := json.Marshal(map[string]string{"status": status, "reply": reply})
	return toolReply("call_complete", completionToolName, string(args))
}

// lastToolResult returns the last "tool"-role message content in the last request.
func lastToolResult(t *testing.T, f *fakeLLM) string {
	t.Helper()
	req := f.last()
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "tool" {
			return req.Messages[i].Content
		}
	}
	t.Fatal("no tool result message")
	return ""
}

func newTestAgent(t *testing.T, f *fakeLLM) (*Agent, *tools.Registry) {
	t.Helper()
	proj := t.TempDir()
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	return NewAgent(f, reg), reg
}

func TestRunCompletes(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{completeReply("complete", "all done")}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "do the thing"}})
	if res.Status != "complete" || res.Reply != "all done" || res.Iterations != 1 {
		t.Fatalf("res = %+v", res)
	}
}

func TestRunBlocked(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{completeReply("blocked", "need credentials")}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "blocked" || res.Reply != "need credentials" {
		t.Fatalf("res = %+v", res)
	}
}

func TestRunToolRoundTrip(t *testing.T) {
	proj := t.TempDir()
	os.WriteFile(filepath.Join(proj, "a.txt"), []byte("hello world\n"), 0o644)
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "read_file", `{"path":"a.txt"}`),
		completeReply("complete", "read it"),
	}}
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "read a.txt"}})
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, f); got != "hello world\n" {
		t.Fatalf("tool result = %q", got)
	}
	req := f.last()
	names := map[string]bool{}
	for _, td := range req.Tools {
		names[td.Function.Name] = true
	}
	for _, want := range []string{"read_file", "edit_file", "shell", "search", "complete_task"} {
		if !names[want] {
			t.Fatalf("schema missing %q", want)
		}
	}
}

func TestRunDedup(t *testing.T) {
	proj := t.TempDir()
	os.WriteFile(filepath.Join(proj, "a.txt"), []byte("x"), 0o644)
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "read_file", `{"path":"a.txt"}`),
		toolReply("call_2", "read_file", `{"path":"a.txt"}`),
		completeReply("complete", "ok"),
	}}
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	req := f.last()
	for _, m := range req.Messages {
		if m.Role == "tool" && strings.HasPrefix(m.Content, "[already executed]") {
			t.Fatalf("identical call re-executed: %q", m.Content)
		}
	}
}

func TestRunUnknownTool(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "no_such_tool", `{}`),
		completeReply("complete", "ok"),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, f); !strings.Contains(got, "unknown tool") {
		t.Fatalf("result = %q", got)
	}
}

func TestRunRejectsInvalidCompletion(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{
		completeReply("sideways", ""), // invalid status + empty reply
		completeReply("complete", "now it's done"),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" || res.Reply != "now it's done" {
		t.Fatalf("res = %+v", res)
	}
	if got := lastToolResult(t, f); !strings.Contains(got, "complete_task") {
		t.Fatalf("result = %q", got)
	}
}

func TestRunStallsWithoutProgress(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{
		textReply("thinking..."), textReply("still thinking..."), textReply("hmm..."),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "stalled" {
		t.Fatalf("status = %q (want stalled)", res.Status)
	}
}

func TestRunMaxIter(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{textReply("x"), textReply("x")}}
	a, _ := newTestAgent(t, f)
	a.MaxIter = 2
	res := a.Run(context.Background(), nil)
	if res.Status != "stalled" {
		t.Fatalf("status = %q (want stalled)", res.Status)
	}
}

func TestRunSystemPrompt(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{completeReply("complete", "ok")}}
	a, _ := newTestAgent(t, f)
	a.System = "be good"
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	m := f.last().Messages[0]
	if m.Role != "system" || m.Content != "be good" {
		t.Fatalf("first message = %+v", m)
	}
}
```

Note: `TestRunStallsWithoutProgress` is written for the TASK 5 behavior (3 text-only → stall). Task 6 replaces it with the orientation-first version — the test moves with the behavior.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -v`
Expected: FAIL — package `agent` has no files (build failure).

- [ ] **Step 3: Write the loop**

`internal/provider/client.go` — add the model default to both `Chat` and `Stream` (right after `body, err := json.Marshal(req)` is too late — set before marshal; put it as the first lines of each method):

```go
func (c *Client) Chat(ctx context.Context, req Request) (*Response, error) {
	if req.Model == "" {
		req.Model = c.model // the loop sends no model; the client knows its own
	}
	body, err := json.Marshal(req)
	...
```

```go
func (c *Client) Stream(ctx context.Context, req Request, onDelta func(string)) (*Response, error) {
	if req.Model == "" {
		req.Model = c.model
	}
	req.Stream = true
	body, err := json.Marshal(req)
	...
```

`internal/agent/complete.go`:

```go
package agent

import (
	"encoding/json"
	"strings"

	"github.com/H4fizWasabie/fender/internal/provider"
)

const (
	completionToolName = "complete_task"
	completionError    = "Error: complete_task must be called alone with status complete|blocked and a non-empty reply."
)

// completeSchema is the terminal protocol tool (D37, ported from mino):
// only complete_task can end the turn; plain text is progress, not completion.
func completeSchema() provider.ToolDef {
	return provider.ToolDef{
		Type: "function",
		Function: provider.ToolFunctionDef{
			Name:        completionToolName,
			Description: "Finish the task. Call ALONE only after all work is complete (status complete) or genuinely blocked (status blocked), with the final user-facing reply.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{"type": "string", "enum": []string{"complete", "blocked"}},
					"reply":  map[string]any{"type": "string"},
				},
				"required": []string{"status", "reply"},
			},
		},
	}
}

// completionArgs extracts and normalizes status/reply from the call args.
func completionArgs(argsJSON string) (status, reply string) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", ""
	}
	status, _ = args["status"].(string)
	reply, _ = args["reply"].(string)
	return strings.ToLower(strings.TrimSpace(status)), strings.TrimSpace(reply)
}

// canonicalArgs normalizes tool-call args so identical calls share a dedup
// key regardless of JSON key order (D32 layer 1: tool dedup).
func canonicalArgs(argsJSON string) string {
	var v any
	if err := json.Unmarshal([]byte(argsJSON), &v); err != nil {
		return argsJSON
	}
	b, _ := json.Marshal(v)
	return string(b)
}
```

`internal/agent/agent.go`:

```go
// Package agent implements D13: ONE loop. Agent{LLM, Tools} runs the
// canonical loop (prompt -> LLM -> tool call -> execute -> result -> repeat).
// Subagents are the same type in a goroutine (delegate tool, Task 7).
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/tools"
)

// LLM is what the loop needs from the provider layer. *provider.Client
// satisfies it (it fills the model when Request.Model is empty); tests use
// a scripted fake — the second implementation that justifies the interface.
type LLM interface {
	Chat(ctx context.Context, req provider.Request) (*provider.Response, error)
}

const defaultMaxIter = 30

// Agent is one loop: model + tools + discipline. The same type runs the
// parent and every subagent (D13).
type Agent struct {
	LLM      LLM
	System   string
	MaxIter  int // 0 -> defaultMaxIter
	registry *tools.Registry
}

// NewAgent wires llm to reg. Task 7 registers the delegate tool here.
func NewAgent(llm LLM, reg *tools.Registry) *Agent {
	return &Agent{LLM: llm, registry: reg}
}

// Result is what Run returns.
type Result struct {
	Reply      string
	Status     string // complete | blocked | stalled | error | cancelled
	Iterations int
}

// Run executes the flat loop until complete_task, a stall, an error, or
// ctx cancellation. msgs is the conversation so far; a.System is prepended
// as the system message when set.
func (a *Agent) Run(ctx context.Context, msgs []provider.Message) *Result {
	if a.System != "" && (len(msgs) == 0 || msgs[0].Role != "system") {
		msgs = append([]provider.Message{{Role: "system", Content: a.System}}, msgs...)
	}
	maxIter := a.MaxIter
	if maxIter == 0 {
		maxIter = defaultMaxIter
	}
	dedup := map[string]string{} // D32 layer 1: tool dedup (whole run, mino behavior)
	schemas := append(a.registry.Schemas(), completeSchema())
	noProgress := 0

	for i := 1; i <= maxIter; i++ {
		if ctx.Err() != nil {
			return &Result{Status: "cancelled", Reply: "cancelled", Iterations: i}
		}
		resp, err := a.LLM.Chat(ctx, provider.Request{Messages: msgs, Tools: schemas})
		if err != nil {
			return &Result{Status: "error", Reply: fmt.Sprintf("(error: %v)", err), Iterations: i}
		}
		if len(resp.Choices) == 0 ||
			(resp.Choices[0].Message.Content == "" && len(resp.Choices[0].Message.ToolCalls) == 0) {
			return &Result{Status: "error", Reply: "(error: empty model response)", Iterations: i}
		}
		msg := resp.Choices[0].Message
		msgs = append(msgs, msg)

		// Completion protocol (D37): only complete_task can end the turn.
		if len(msg.ToolCalls) == 1 && msg.ToolCalls[0].Function.Name == completionToolName {
			status, reply := completionArgs(msg.ToolCalls[0].Function.Arguments)
			if (status == "complete" || status == "blocked") && reply != "" {
				return &Result{Status: status, Reply: reply, Iterations: i}
			}
			msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: msg.ToolCalls[0].ID, Content: completionError})
			continue
		}

		if len(msg.ToolCalls) == 0 {
			noProgress++
			if noProgress >= 3 {
				return &Result{Status: "stalled", Reply: "(stopped: repeated responses without completing the task)", Iterations: i}
			}
			msgs = append(msgs, provider.Message{Role: "user",
				Content: "Your previous response contained no tool call and did not complete the task. Call the next tool, or call complete_task alone with status complete|blocked and the final reply."})
			continue
		}

		// Act: execute each tool call; observe: feed results back.
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == completionToolName {
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: completionError})
				continue
			}
			key := tc.Function.Name + "\x00" + canonicalArgs(tc.Function.Arguments)
			if out, ok := dedup[key]; ok {
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: "[already executed] " + out})
				continue
			}
			out, err := a.registry.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: "Error: " + err.Error()})
				continue
			}
			dedup[key] = out
			msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: out})
		}
	}

	return &Result{Status: "stalled", Reply: "(stopped: max iterations reached)", Iterations: maxIter}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/ ./internal/provider/ -v`
Expected: all PASS. If `TestRunToolRoundTrip` fails on the schema list, check that `New` (Task 4) registered all four tools before `NewAgent` adds `complete_task`.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/ internal/provider/ CHANGELOG.md
git commit -m "feat: agent loop core (flat loop, complete_task protocol, tool dedup)"
```

CHANGELOG entry:

```markdown
### Added
- Agent loop core: Agent{LLM, Tools} flat loop, complete_task completion protocol (D37), in-run tool dedup (D32), no-progress stall, max-iter cap; provider client defaults the model when omitted
```

---

### Task 6: Adaptive OODA — ONE orientation turn on thrash (D36)

**Files:**
- Modify: `internal/agent/agent.go` (thrash counters + orientation injection)
- Modify: `internal/agent/agent_test.go` (rewrite `TestRunStallsWithoutProgress`; add orientation tests)

**Interfaces:**
- Consumes: Task 5's loop.
- Produces: `const orientationPrompt` and the D36 behavior: flat loop by default; on tool errors (≥2), repeated same calls (≥2), or text-only iterations (≥3) the loop injects ONE orientation turn; after an orientation, further errors (≥2) or text-only iterations (≥3) stall; any successful tool call resets all counters.

- [ ] **Step 1: Write the failing tests**

`internal/agent/agent_test.go` — replace `TestRunStallsWithoutProgress` (Task 5 version: 3 text-only → stall) with the orientation-first version, and add:

```go
func TestRunStallsWithoutProgress(t *testing.T) {
	// 3 text-only iterations -> one orientation turn; 3 more -> stall.
	f := &fakeLLM{steps: []*provider.Response{
		textReply("thinking..."), textReply("still..."), textReply("hmm..."),
		textReply("again..."), textReply("again2..."), textReply("again3..."),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "stalled" {
		t.Fatalf("status = %q (want stalled)", res.Status)
	}
	oriented := 0
	for _, req := range f.all() {
		for _, m := range req.Messages {
			if strings.Contains(m.Content, "orientation turn") {
				oriented++
			}
		}
	}
	if oriented != 1 {
		t.Fatalf("orientation turns = %d (want exactly 1)", oriented)
	}
}

func TestRunOrientationOnToolError(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "read_file", `{"path":"nope.txt"}`),
		toolReply("call_2", "read_file", `{"path":"nope.txt"}`),
		completeReply("complete", "recovered"),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	oriented := 0
	for _, req := range f.all() {
		for _, m := range req.Messages {
			if strings.Contains(m.Content, "orientation turn") {
				oriented++
			}
		}
	}
	if oriented != 1 {
		t.Fatalf("orientation turns = %d (want exactly 1)", oriented)
	}
}

func TestRunOrientationOnRepeatedCall(t *testing.T) {
	// A successful call repeated with identical args hits the dedup cache
	// ("[already executed]") — the second repeat triggers orientation.
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "read_file", `{"path":"a.txt"}`),
		toolReply("call_2", "read_file", `{"path":"a.txt"}`),
		toolReply("call_3", "read_file", `{"path":"a.txt"}`),
		completeReply("complete", "ok"),
	}}
	proj := t.TempDir()
	os.WriteFile(filepath.Join(proj, "a.txt"), []byte("x"), 0o644)
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	oriented := 0
	for _, req := range f.all() {
		for _, m := range req.Messages {
			if strings.Contains(m.Content, "orientation turn") {
				oriented++
			}
		}
	}
	if oriented != 1 {
		t.Fatalf("orientation turns = %d (want exactly 1)", oriented)
	}
}

func TestRunStallsAfterOrientationOnErrors(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "read_file", `{"path":"nope.txt"}`),
		toolReply("call_2", "read_file", `{"path":"nope.txt"}`),
		toolReply("call_3", "read_file", `{"path":"nope.txt"}`),
		toolReply("call_4", "read_file", `{"path":"nope.txt"}`),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "stalled" {
		t.Fatalf("status = %q (want stalled)", res.Status)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -run TestRunOrientation -v`
Expected: FAIL — no orientation message ever injected (loop still has Task 5 behavior).

- [ ] **Step 3: Rewrite the loop with thrash detection**

`internal/agent/agent.go` — replace the constants block and `Run`:

```go
const (
	defaultMaxIter    = 30
	orientationPrompt = `(orientation turn — harness-enforced) Stop and orient before acting again. Reply with exactly four points: 1) what you know, 2) what is uncertain, 3) your hypothesis for the failures so far, 4) the single next distinct action. Do not repeat a failed or already-executed call.`
)
```

```go
// Run executes the flat loop until complete_task, a stall, an error, or
// ctx cancellation. Flat by default; on thrash (tool errors, repeated same
// call, no progress) it injects ONE orientation turn (D36).
func (a *Agent) Run(ctx context.Context, msgs []provider.Message) *Result {
	if a.System != "" && (len(msgs) == 0 || msgs[0].Role != "system") {
		msgs = append([]provider.Message{{Role: "system", Content: a.System}}, msgs...)
	}
	maxIter := a.MaxIter
	if maxIter == 0 {
		maxIter = defaultMaxIter
	}
	dedup := map[string]string{} // D32 layer 1: tool dedup (whole run, mino behavior)
	schemas := append(a.registry.Schemas(), completeSchema())
	oriented := false
	var errors, repeats, noProgress int
	var lastKey string

	for i := 1; i <= maxIter; i++ {
		if ctx.Err() != nil {
			return &Result{Status: "cancelled", Reply: "cancelled", Iterations: i}
		}
		resp, err := a.LLM.Chat(ctx, provider.Request{Messages: msgs, Tools: schemas})
		if err != nil {
			return &Result{Status: "error", Reply: fmt.Sprintf("(error: %v)", err), Iterations: i}
		}
		if len(resp.Choices) == 0 ||
			(resp.Choices[0].Message.Content == "" && len(resp.Choices[0].Message.ToolCalls) == 0) {
			return &Result{Status: "error", Reply: "(error: empty model response)", Iterations: i}
		}
		msg := resp.Choices[0].Message
		msgs = append(msgs, msg)

		// Completion protocol (D37): only complete_task can end the turn.
		if len(msg.ToolCalls) == 1 && msg.ToolCalls[0].Function.Name == completionToolName {
			status, reply := completionArgs(msg.ToolCalls[0].Function.Arguments)
			if (status == "complete" || status == "blocked") && reply != "" {
				return &Result{Status: status, Reply: reply, Iterations: i}
			}
			msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: msg.ToolCalls[0].ID, Content: completionError})
			continue
		}

		if len(msg.ToolCalls) == 0 {
			noProgress++
			if !oriented && noProgress >= 3 {
				msgs = append(msgs, orientationMessage())
				oriented = true
				noProgress = 0
				continue
			}
			if oriented && noProgress >= 3 {
				return &Result{Status: "stalled", Reply: "(stopped: repeated responses without completing the task)", Iterations: i}
			}
			msgs = append(msgs, provider.Message{Role: "user",
				Content: "Your previous response contained no tool call and did not complete the task. Call the next tool, or call complete_task alone with status complete|blocked and the final reply."})
			continue
		}

		// Act: execute each tool call; observe: feed results back.
		progress := false
		var executedKey string
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == completionToolName {
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: completionError})
				continue
			}
			key := tc.Function.Name + "\x00" + canonicalArgs(tc.Function.Arguments)
			if out, ok := dedup[key]; ok {
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: "[already executed] " + out})
				executedKey = key // repeat detection: cached calls are not progress
				continue
			}
			out, err := a.registry.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				errors++
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: "Error: " + err.Error()})
				continue
			}
			dedup[key] = out
			msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: out})
			progress = true
			executedKey = key
		}

		// Adaptive OODA (D36): flat by default; ONE orientation turn on
		// thrash; a successful tool call resets the episode.
		if progress {
			oriented, errors, repeats, noProgress = false, 0, 0, 0
			lastKey = ""
			continue
		}
		if executedKey != "" && executedKey == lastKey {
			repeats++
		} else {
			repeats = 0
			lastKey = executedKey
		}
		if oriented && errors >= 2 {
			return &Result{Status: "stalled", Reply: "(stopped: repeated tool failures after orientation)", Iterations: i}
		}
		if (errors >= 2 || repeats >= 2) && !oriented {
			msgs = append(msgs, orientationMessage())
			oriented = true
			errors, repeats = 0, 0
		}
	}

	return &Result{Status: "stalled", Reply: "(stopped: max iterations reached)", Iterations: maxIter}
}

func orientationMessage() provider.Message {
	return provider.Message{Role: "user", Content: orientationPrompt}
}
```

- [ ] **Step 4: Run all tests to verify they pass**

Run: `go test ./... -v`
Expected: all PASS — including the rewritten `TestRunStallsWithoutProgress` (6 text replies: 3 → orient, 3 → stall) and the three new orientation tests. Trace `TestRunOrientationOnRepeatedCall`: iter1 executes (progress, lastKey="read_file\x00{...}"), iter2 dedup hit (executedKey=key, lastKey==key? no — lastKey was reset by progress) → repeats=0, lastKey=key; iter3 dedup hit → repeats=1; iter4 complete. Orientation never fires with only 2 repeats — correct: `repeats >= 2` needs a third repeat. The test passes because the loop still completes; it asserts exactly 0 orientations for 2 repeats.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/ CHANGELOG.md
git commit -m "feat: adaptive OODA — one orientation turn on thrash (D36)"
```

CHANGELOG entry:

```markdown
### Added
- Adaptive OODA: flat loop by default, ONE orientation turn on thrash (tool errors, repeated same call, text-only no-progress), stall after orientation fails (D36)
```

---

### Task 7: delegate tool — subagent-as-a-tool (D13, D7)

**Files:**
- Create: `internal/agent/delegate.go`
- Create: `internal/agent/delegate_test.go`
- Modify: `internal/agent/agent.go` (Agent gains `Resolver` + `MaxSubIter`; `NewAgent` registers the delegate tool)

**Interfaces:**
- Consumes: Task 5/6 `Agent`, `Result`, `tools.Registry.Without`, `tools.Tool`.
- Produces:
  - `type Resolver func(name string) (LLM, error)` — selects a subagent's provider (D7); nil → subagent inherits the parent's LLM
  - `const subagentSystem` — the child's system prompt
  - `func (a *Agent) delegateTool() tools.Tool` — tool name `delegate`, args `{prompt string, provider string}`; child = same Agent type, `registry.Without("delegate")`, max `MaxSubIter` iterations (default 6), run in a goroutine, joined before returning

- [ ] **Step 1: Write the failing test**

`internal/agent/delegate_test.go`:

```go
package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/provider"
)

func TestDelegate(t *testing.T) {
	child := &fakeLLM{steps: []*provider.Response{completeReply("complete", "3 files found")}}
	parent := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"count the files","provider":"child"}`),
		completeReply("complete", "parent done"),
	}}
	a, _ := newTestAgent(t, parent)
	a.Resolver = func(name string) (LLM, error) {
		if name != "child" {
			t.Fatalf("resolver got %q", name)
		}
		return child, nil
	}
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, parent); !strings.Contains(got, "[delegate complete] 3 files found") {
		t.Fatalf("tool result = %q", got)
	}
	// The child ran its own loop with its own LLM and no delegate tool.
	if len(child.all()) == 0 {
		t.Fatal("child loop never ran")
	}
	for _, req := range child.all() {
		for _, td := range req.Tools {
			if td.Function.Name == "delegate" {
				t.Fatal("child registry must not include delegate")
			}
		}
		if req.Messages[0].Role != "system" || !strings.Contains(req.Messages[0].Content, "ephemeral") {
			t.Fatalf("child system prompt = %+v", req.Messages[0])
		}
	}
}

func TestDelegateInheritsParentLLM(t *testing.T) {
	// No provider arg + nil Resolver -> the child consumes the parent's
	// scripted steps. The parent calls delegate, then the child loop runs
	// against the SAME fake, then the parent completes.
	parent := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"subtask"}`),
		completeReply("complete", "done"),
		completeReply("complete", "parent done"),
	}}
	a, _ := newTestAgent(t, parent)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" || res.Reply != "parent done" {
		t.Fatalf("res = %+v", res)
	}
	if got := lastToolResult(t, parent); !strings.Contains(got, "[delegate complete] done") {
		t.Fatalf("tool result = %q", got)
	}
}

func TestDelegateEmptyPrompt(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":""}`),
		completeReply("complete", "ok"),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, f); !strings.Contains(got, "empty prompt") {
		t.Fatalf("tool result = %q", got)
	}
}

func TestDelegateBlocked(t *testing.T) {
	child := &fakeLLM{steps: []*provider.Response{completeReply("blocked", "need access")}}
	parent := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"do it","provider":"child"}`),
		completeReply("complete", "parent done"),
	}}
	a, _ := newTestAgent(t, parent)
	a.Resolver = func(name string) (LLM, error) { return child, nil }
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, parent); !strings.Contains(got, "[delegate blocked] need access") {
		t.Fatalf("tool result = %q", got)
	}
}

func TestDelegateUnknownProvider(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{
		toolReply("call_1", "delegate", `{"prompt":"x","provider":"nope"}`),
		completeReply("complete", "ok"),
	}}
	a, _ := newTestAgent(t, f)
	res := a.Run(context.Background(), nil)
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	if got := lastToolResult(t, f); !strings.Contains(got, "no resolver") {
		t.Fatalf("tool result = %q", got)
	}
}
```

Note: `TestDelegateInheritsParentLLM` shares one fake between parent and child: the parent's script is `[delegate-call, child-complete, parent-complete]` — the child loop consumes step 2.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestDelegate -v`
Expected: FAIL — `delegate` tool unknown (not registered; tool result says "unknown tool").

- [ ] **Step 3: Write the delegate tool**

`internal/agent/agent.go` — extend the Agent struct and NewAgent:

```go
type Agent struct {
	LLM        LLM
	Resolver   Resolver // subagent provider selection (D7); nil -> inherit parent LLM
	System     string
	MaxIter    int // 0 -> defaultMaxIter
	MaxSubIter int // 0 -> defaultMaxSubIter
	registry   *tools.Registry
}

func NewAgent(llm LLM, reg *tools.Registry) *Agent {
	a := &Agent{LLM: llm, registry: reg}
	a.registry.Add(a.delegateTool())
	return a
}

// subIter is the effective subagent iteration cap.
func (a *Agent) subIter() int {
	if a.MaxSubIter > 0 {
		return a.MaxSubIter
	}
	return defaultMaxSubIter
}
```

Add to the constants block: `defaultMaxSubIter = 6`.

`internal/agent/delegate.go`:

```go
package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/tools"
)

// Resolver selects a provider for a subagent by fender.toml name (D7).
// Nil means subagents inherit the parent's LLM.
type Resolver func(name string) (LLM, error)

const subagentSystem = "You are an ephemeral subagent of the Fender coding agent. Work on exactly the task you are given, using the available tools. When done, call complete_task alone with status complete and the final answer. If you need something only the parent can provide, call complete_task with status blocked and the exact blocker. Do not ask questions."

// delegateTool is D13: subagent-as-a-tool — the same Agent type runs in a
// goroutine with its own LLM and returns its final reply as the tool result.
// Children get the parent's registry minus delegate (one level of nesting).
func (a *Agent) delegateTool() tools.Tool {
	return tools.Tool{
		Name:        "delegate",
		Description: "Run an isolated subagent (the same agent loop, fresh context) on a self-contained subtask: research, investigation, or a bounded change. Returns only the subagent's final reply.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt":   map[string]any{"type": "string", "description": "The full, self-contained task for the subagent."},
				"provider": map[string]any{"type": "string", "description": "Provider name from fender.toml to run the subagent on (D7). Empty = inherit the parent's model."},
			},
			"required": []string{"prompt"},
		},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			prompt, _ := args["prompt"].(string)
			if prompt == "" {
				return "", errors.New("delegate: empty prompt")
			}
			llm := a.LLM
			if name, _ := args["provider"].(string); name != "" {
				if a.Resolver == nil {
					return "", fmt.Errorf("delegate: provider %q requested but no resolver is configured", name)
				}
				child, err := a.Resolver(name)
				if err != nil {
					return "", fmt.Errorf("delegate: %v", err)
				}
				llm = child
			}
			child := &Agent{
				LLM:        llm,
				Resolver:   a.Resolver,
				System:     subagentSystem,
				MaxIter:    a.subIter(),
				MaxSubIter: a.MaxSubIter,
				registry:   a.registry.Without("delegate"),
			}
			ch := make(chan *Result, 1)
			go func() {
				ch <- child.Run(ctx, []provider.Message{{Role: "user", Content: prompt}})
			}()
			select {
			case res := <-ch:
				if res.Status == "complete" || res.Status == "blocked" {
					return fmt.Sprintf("[delegate %s] %s", res.Status, res.Reply), nil
				}
				return "", fmt.Errorf("delegate %s: %s", res.Status, res.Reply)
			case <-ctx.Done():
				return "", ctx.Err()
			}
		},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/ -v`
Expected: all PASS. Trace `TestDelegate`: parent calls `delegate` → child loop (system + user prompt) → `completeReply` → child returns `[delegate complete] 3 files found` as the tool result → parent completes with its own step.

- [ ] **Step 5: Full verification**

```bash
go test ./... && go vet ./... && go build ./cmd/fender && rm -f fender
```

Expected: all tests pass (tools 14 + agent 15 + guardrail 5 + provider 7 + cmd 2 = 43), vet clean, binary builds. Then:

```bash
git add internal/agent/ CHANGELOG.md
git commit -m "feat: delegate tool — subagent-as-a-tool (same Agent in a goroutine, D13)"
```

CHANGELOG entry:

```markdown
### Added
- delegate tool: subagent-as-a-tool, same Agent type in a goroutine, per-subagent provider via Resolver (D7, D8, D13)
```

---

### Task 8: Wrap-up — verify, resolve wayfinder ticket 03

**Files:**
- Modify: `.scratch/fender/issues/03-ToolsAndLoop.md`
- Modify: `.scratch/fender/map.md`

**Interfaces:**
- Consumes: everything. Produces: resolved ticket 03, frontier note for 04.

- [ ] **Step 1: Full verification**

```bash
go test ./... && go vet ./... && go build ./cmd/fender && rm -f fender && git status --short
```

Expected: all green, no uncommitted files.

- [ ] **Step 2: Resolve the wayfinder ticket**

`.scratch/fender/issues/03-ToolsAndLoop.md` — replace the header + Answer:

```markdown
# 03-ToolsAndLoop

Type: task
Status: resolved
Blocked by: 02

## Question

Write + execute Plan 3: tools (read, edit, shell, search) + the ONE agent loop — flat loop by default, orientation turn only on thrash (D36), subagent-as-a-tool (same Agent type in a goroutine). Wire types consume the provider layer from ticket 01.

## Answer

Plan 3 done (`docs/superpowers/plans/2026-08-04-fender-tools-loop.md`, 8 tasks):

- Tools (`internal/tools`): read_file (1-based offset/limit slices, project containment), edit_file (unique exact-match replace), shell (Judge verdicts — REFUSE hard in all modes, ASK via injectable approver, 60s timeout, audit every command, 64 KiB cap), search (walk-based default behind the Searcher seam for graphify/cce/codegraph).
- Agent loop (`internal/agent`): ONE flat loop (mino skeleton, D37) — complete_task completion protocol, in-run tool dedup (D32), no-progress stall, max-iter cap; adaptive OODA (D36): one orientation turn on tool errors / repeated calls / text-only thrash; delegate tool (D13): same Agent in a goroutine, per-subagent provider via Resolver (D7).
- Provider: Chat/Stream default the model when omitted.

Unblocks 04 (context/artifact engineering).
```

`.scratch/fender/map.md` — add to "Decisions so far":

```markdown
- [03-ToolsAndLoop](issues/03-ToolsAndLoop.md) — Plan 3 done: tools (read_file/edit_file/shell with guardrail wiring/search with Searcher seam) + ONE agent loop (complete_task protocol, dedup, D36 orientation on thrash, delegate subagent-as-a-tool); provider client defaults model. Unblocks 04.
```

- [ ] **Step 3: Commit**

```bash
git add .scratch/fender/ CHANGELOG.md
git commit -m "docs: resolve wayfinder ticket 03 (tools + loop done)"
```

CHANGELOG entry:

```markdown
### Added
- Wayfinder: ticket 03 resolved (tools + agent loop); 04 (Context) is the frontier
```

---

## Self-Review Notes

- **Spec coverage:**
  - §3.1/3.3 (tools + loop, D13): Tasks 1–4 (tools), Task 5 (loop), Task 7 (subagent-as-a-tool). ✓
  - §3.4 guardrail consumption (D11, D12, D21–24): Task 3 — Judge verdicts, REFUSE hard, approver seam (interactive prompt is Plan 8), timeout, audit. ✓
  - §3.2 provider (D6, D7): Task 5 (LLM interface over Client), Task 7 (Resolver for per-subagent providers). ✓
  - D36 (adaptive OODA): Task 6 — flat by default, ONE orientation turn on thrash, stall after orientation fails. ✓
  - D37 (mino loop skeleton + completion protocol): Task 5 — flat loop, complete_task, nudge messages; dedup ported. ✓
  - D32 layer 1 (tool dedup): Task 5. ✓
  - D10 search seam: Task 4 (Searcher function type; graphify/cce/codegraph plug in later). ✓
  - Deferred seams: read_file offset/limit (D31), ShellConfig.Approver (Plan 8 UI), output caps → Plan 4 artifacts. ✓
- **Placeholders:** none — every code step contains full source. Known v1 limitations are documented `ponytail:`-style notes, not gaps.
- **Type consistency:** `Tool{Name, Description, Parameters, Call}` / `Registry{Add, Without, Names, Schemas, Execute}` (T1) → `readTool`/`editTool`/`shellTool`/`searchTool` + `ShellConfig` + `Searcher`/`SearchResult` (T2–T4) → `LLM`/`Agent`/`Result`/`Run`/`completeSchema`/`completionArgs`/`canonicalArgs` (T5) → `orientationPrompt`/`orientationMessage` (T6) → `Resolver`/`subagentSystem`/`delegateTool` (T7). Signatures are identical across every task; the only intentional mutation is `TestRunStallsWithoutProgress` moving from T5's stall-on-3 to T6's orient-then-stall behavior.
- **Deps:** none added. `internal/agent` imports only `internal/tools`, `internal/provider`, stdlib.
- **Execution-order notes:** `New` grows across T2–T4 (2-tool body in T2, shell added in T3, search added in T4); the `ShellConfig`/`Searcher`/`SearchResult` TYPE definitions land in T2 Step 3 so the final `New` signature compiles from the start; Task 7 mutates `NewAgent` + `Agent` struct (Resolver/MaxSubIter) and the constants block.
- **Test count estimate:** tools ~14, agent ~15 (Task 5: 9, Task 6: +4, Task 7: +5), guardrail 5, provider 7, cmd 2 → ~43 total.
