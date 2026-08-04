# Fender Plan 6: Skills Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `internal/skills` — 23 bundled skills (go:embed), frontmatter parser, registry with shadowing, deterministic trigger matching (model-invokable only), `load_skill` tool, `fender skill install`, nil-safe agent wiring.

**Architecture:** Skills are standard SKILL.md files (name/description frontmatter + markdown body). Bundled copies live in `internal/skills/bundled/` (vendored, MIT, attribution in `LICENSE.md`). Frontmatter parsed with a narrow regex parser (no YAML dependency — ponytail ladder). Matching = word-overlap scoring, top 3, 8K body budget, skipping `disable-model-invocation: true` skills (user-invoked only). ponytail core = always-loaded discipline, never matched, never shadowed.

**Tech Stack:** Go 1.22, stdlib only (`regexp`, `strings`, `embed`, `os/exec` for git clone). No new dependencies.

## Global Constraints

- **Read `AGENTS.md`, `DECISIONS.md` (D27–D30), ticket-06 spec first.**
- **Every commit MUST stage `CHANGELOG.md`** — enforced by `.githooks/pre-commit`.
- **Allowed dependencies only:** `BurntSushi/toml`, `mvdan.cc/sh/v3`, `go-tree-sitter`, `modernc.org/sqlite`. Nothing else.
- **No frameworks.** Explicit errors, no panic in library code. Nil-safe agent integration (existing tests unchanged).
- Module path `github.com/H4fizWasabie/fender`; files under `internal/skills/`.

---

### Task 1: Vendor bundled skills + attribution

**Files:**
- Create: `internal/skills/bundled/*/SKILL.md` + support files (23 dirs)
- Create: `internal/skills/bundled/LICENSE.md`

**Context:** Vendoring is already done in the working tree (copied from `~/Desktop/fender-references/`, `agents/*.yaml` excluded). This task commits it with attribution.

- [ ] **Step 1: Verify the vendor set**

```bash
cd /home/hafiz/Desktop/Fender
ls internal/skills/bundled | wc -l          # 23
find internal/skills/bundled -name SKILL.md | wc -l   # 23
find internal/skills/bundled -name "*.yaml" | wc -l   # 0
```

- [ ] **Step 2: Write attribution**

`internal/skills/bundled/LICENSE.md`:

```markdown
# Bundled skills — attribution

All skills are MIT-licensed and vendored from their upstream repos. Bodies
are unmodified except for removal of Codex-specific `agents/*.yaml` files.

- 17 engineering skills: https://github.com/mattpocock/skills (MIT)
- 6 ponytail skills: https://github.com/DietrichGebert/ponytail (MIT)

Reference clones: `~/Desktop/fender-references/` (code-context-engine, codegraph,
graphify, skills, ponytail — all MIT).
```

- [ ] **Step 3: Commit**

```bash
git add internal/skills/bundled/ CHANGELOG.md
git commit -m "chore: vendor 23 bundled skills (17 engineering + 6 ponytail, MIT)"
```

CHANGELOG entry:

```markdown
### Added
- Vendored 23 bundled skills into internal/skills/bundled/ (MIT, attribution; Codex yaml excluded)
```

---

### Task 2: Frontmatter parser

**Files:**
- Create: `internal/skills/frontmatter.go`
- Create: `internal/skills/frontmatter_test.go`

**Interfaces:**
- Produces:
  - `type meta struct { Name string; Description string; ModelInvokable bool }`
  - `func parseFrontmatter(content string) (meta, string, bool)` — (meta, body, ok). Handles single-line, quoted (`"..."`), folded (`>` + indented lines). `disable-model-invocation: true` → `ModelInvokable = false`. Any parse failure → ok=false (skill skipped with warning, never a hard error).

- [ ] **Step 1: Write the failing test**

`internal/skills/frontmatter_test.go`:

```go
package skills

import (
	"strings"
	"testing"
)

func TestParseSingleLine(t *testing.T) {
	m, body, ok := parseFrontmatter("---\nname: tdd\ndescription: Test-driven development.\n---\n# Body\ncontent\n")
	if !ok || m.Name != "tdd" || m.Description != "Test-driven development." || !m.ModelInvokable {
		t.Fatalf("m=%+v body=%q ok=%v", m, body, ok)
	}
	if !strings.Contains(body, "# Body") {
		t.Fatalf("body = %q", body)
	}
}

func TestParseQuoted(t *testing.T) {
	m, _, ok := parseFrontmatter("---\nname: implement\ndescription: \"Implement a piece of work based on a spec.\"\n---\n")
	if !ok || m.Description != "Implement a piece of work based on a spec." {
		t.Fatalf("m=%+v ok=%v", m, ok)
	}
}

func TestParseFolded(t *testing.T) {
	m, _, ok := parseFrontmatter("---\nname: ponytail\ndescription: >\n  Forces the laziest solution that actually works, simplest, shortest,\n  most minimal.\n---\n")
	if !ok || m.Name != "ponytail" {
		t.Fatalf("m=%+v ok=%v", m, ok)
	}
	if !strings.Contains(m.Description, "laziest solution") || !strings.Contains(m.Description, "most minimal") {
		t.Fatalf("description = %q", m.Description)
	}
}

func TestParseUserInvoked(t *testing.T) {
	m, _, ok := parseFrontmatter("---\nname: ask-matt\ndescription: Router skill.\ndisable-model-invocation: true\n---\n")
	if !ok || m.ModelInvokable {
		t.Fatalf("m=%+v ok=%v", m, ok)
	}
}

func TestParseBroken(t *testing.T) {
	if _, _, ok := parseFrontmatter("no frontmatter here"); ok {
		t.Fatal("expected failure")
	}
	if _, _, ok := parseFrontmatter("---\nname: x\n---\n"); ok {
		t.Fatal("missing description must fail")
	}
}

func TestAllBundledParse(t *testing.T) {
	reg, err := Bundled()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(reg.all); got != 23 {
		t.Fatalf("bundled skills = %d, want 23", got)
	}
	for name, s := range reg.all {
		if s.Name == "" || s.Description == "" || s.Body == "" {
			t.Fatalf("skill %s incomplete: %+v", name, s)
		}
	}
}
```

Note: `TestAllBundledParse` depends on Task 3's `Bundled()` — it will fail to compile until then. Write Task 2's implementation now; run tests after Task 3 lands, or stub `Bundled()` in Task 2 (recommended: write `bundled.go` with the embed + `Bundled()` in Task 2 so the test compiles — the parser and loader belong together).

- [ ] **Step 2: Write the parser + loader**

`internal/skills/frontmatter.go`:

```go
package skills

import (
	"regexp"
	"strings"
)

// meta is the parsed SKILL.md frontmatter. The format is narrow (name +
// description + optional disable-model-invocation), so a regex parser
// replaces a YAML dependency (ponytail ladder).
type meta struct {
	Name           string
	Description    string
	ModelInvokable bool
}

var (
	nameRe    = regexp.MustCompile(`(?m)^name:\s*(.+?)\s*$`)
	descRe    = regexp.MustCompile(`(?m)^description:\s*(.+?)\s*$`)
	disableRe = regexp.MustCompile(`(?m)^disable-model-invocation:\s*true\s*$`)
	foldedRe  = regexp.MustCompile(`(?m)^description:\s*>\s*$`)
)

// parseFrontmatter extracts (meta, body, ok) from a SKILL.md file.
// ok=false on any structural failure — caller skips the skill with a warning.
func parseFrontmatter(content string) (meta, string, bool) {
	if !strings.HasPrefix(content, "---") {
		return meta{}, "", false
	}
	end := strings.Index(content[3:], "---")
	if end < 0 {
		return meta{}, "", false
	}
	head := content[:3+end]
	body := strings.TrimSpace(content[3+end+3:])
	m := meta{ModelInvokable: true}

	nameMatch := nameRe.FindStringSubmatch(head)
	if nameMatch == nil || strings.TrimSpace(nameMatch[1]) == "" {
		return meta{}, "", false
	}
	m.Name = strings.TrimSpace(nameMatch[1])

	if disableRe.MatchString(head) {
		m.ModelInvokable = false
	}

	if foldedRe.MatchString(head) {
		// description: >  followed by indented lines
		var lines []string
		idx := strings.Index(head, "description:")
		rest := head[idx+len("description:"):]
		for _, line := range strings.Split(rest, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == ">" || trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "disable-model-invocation:") {
				break
			}
			if strings.Contains(line, ":") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				break // next frontmatter key
			}
			lines = append(lines, trimmed)
		}
		m.Description = strings.Join(lines, " ")
	} else {
		descMatch := descRe.FindStringSubmatch(head)
		if descMatch == nil || strings.TrimSpace(descMatch[1]) == "" {
			return meta{}, "", false
		}
		m.Description = strings.Trim(strings.TrimSpace(descMatch[1]), `"`)
	}
	if m.Name == "" || m.Description == "" {
		return meta{}, "", false
	}
	return m, body, true
}
```

`internal/skills/bundled.go`:

```go
package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

//go:embed bundled
var bundledFS embed.FS

// Bundled loads all embedded skills. Every bundled skill must parse —
// a broken bundled skill is a build-time bug, surfaced here as an error.
func Bundled() (*Registry, error) {
	r := &Registry{all: map[string]Skill{}}
	entries, err := fs.ReadDir(bundledFS, "bundled")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		data, err := bundledFS.ReadFile(filepath.Join("bundled", name, "SKILL.md"))
		if err != nil {
			return nil, fmt.Errorf("bundled %s: %w", name, err)
		}
		m, body, ok := parseFrontmatter(string(data))
		if !ok {
			return nil, fmt.Errorf("bundled %s: frontmatter parse failed", name)
		}
		r.all[m.Name] = Skill{
			Name: m.Name, Description: m.Description, Body: body,
			Source: "bundled", Path: "bundled/" + name + "/SKILL.md",
		}
	}
	return r, nil
}
```

Note: the Registry type (`all map[string]Skill`) is defined in Task 3 — write `registry.go` (type + helpers) in Task 2 so this compiles; the matching/lookup logic lands in Task 3.

- [ ] **Step 3: Run tests to verify they pass**

Run: `go test ./internal/skills/ -v`
Expected: PASS (parser + all 23 bundled parse).

- [ ] **Step 4: Commit**

```bash
git add internal/skills/ CHANGELOG.md
git commit -m "feat: SKILL.md frontmatter parser (regex, folded/quoted, model-invokable) + Bundled() loader"
```

CHANGELOG entry:

```markdown
### Added
- Skills: frontmatter parser (single-line/quoted/folded, disable-model-invocation), Bundled() go:embed loader — all 23 parse
```

---

### Task 3: Registry (load, merge, lookup, descriptions)

**Files:**
- Create: `internal/skills/registry.go`
- Create: `internal/skills/registry_test.go`

**Interfaces:**
- Consumes: `parseFrontmatter`, `Bundled` (Task 2).
- Produces:
  - `type Skill struct { Name, Description, Body, Source, Path string }`
  - `type Registry struct` with:
    - `func Load(dir string) (*Registry, error)` — scans `dir/*/SKILL.md`; missing dir → empty registry, nil error; broken skill → skipped (no error)
    - `func (r *Registry) Merge(project, user *Registry) *Registry` — precedence project > user > r; returns new registry
    - `func (r *Registry) ByName(name string) (Skill, bool)`
    - `func (r *Registry) Descriptions() string` — one line per skill: `- <name>: <description>`, capped at 4000 chars
    - `func (r *Registry) PonytailCore() (Skill, bool)` — the bundled ponytail skill (always-loaded discipline)

- [ ] **Step 1: Write the failing test**

`internal/skills/registry_test.go`:

```go
package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, dir, name, desc string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0700); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + desc + "\n---\nbody of " + name
	if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMissingDirIsEmpty(t *testing.T) {
	r, err := Load(filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.all) != 0 {
		t.Fatalf("all = %d", len(r.all))
	}
}

func TestLoadSkipsBroken(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "good", "Good skill.")
	bad := filepath.Join(dir, "bad")
	os.MkdirAll(bad, 0700)
	os.WriteFile(filepath.Join(bad, "SKILL.md"), []byte("no frontmatter"), 0600)
	r, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.ByName("good"); !ok {
		t.Fatal("good skill missing")
	}
	if _, ok := r.ByName("bad"); ok {
		t.Fatal("broken skill must be skipped")
	}
}

func TestMergeShadowing(t *testing.T) {
	bundled := &Registry{all: map[string]Skill{
		"x": {Name: "x", Description: "bundled x", Body: "b", Source: "bundled"},
	}}
	user := &Registry{all: map[string]Skill{
		"x": {Name: "x", Description: "user x", Body: "u", Source: "user"},
		"y": {Name: "y", Description: "user y", Body: "u", Source: "user"},
	}}
	project := &Registry{all: map[string]Skill{
		"x": {Name: "x", Description: "project x", Body: "p", Source: "project"},
	}}
	merged := bundled.Merge(project, user)
	if got, _ := merged.ByName("x"); got.Source != "project" {
		t.Fatalf("x = %+v", got)
	}
	if got, _ := merged.ByName("y"); got.Source != "user" {
		t.Fatalf("y = %+v", got)
	}
	if got, _ := merged.ByName("z"); got.Source != "bundled" {
		t.Fatalf("z = %+v", got)
	}
}

func TestDescriptionsCapped(t *testing.T) {
	reg := &Registry{all: map[string]Skill{
		"a": {Name: "a", Description: strings.Repeat("d", 3000)},
		"b": {Name: "b", Description: strings.Repeat("e", 3000)},
	}}
	got := reg.Descriptions()
	if len(got) > DescriptionListCap {
		t.Fatalf("descriptions %d > cap %d", len(got), DescriptionListCap)
	}
}

func TestPonytailCore(t *testing.T) {
	reg, err := Bundled()
	if err != nil {
		t.Fatal(err)
	}
	s, ok := reg.PonytailCore()
	if !ok || s.Name != "ponytail" {
		t.Fatalf("ponytail core = %+v ok=%v", s, ok)
	}
	if !strings.Contains(s.Body, "ladder") && !strings.Contains(s.Body, "laziest") {
		t.Fatalf("ponytail body suspicious: %q", s.Body[:100])
	}
}
```

- [ ] **Step 2: Write the registry**

`internal/skills/registry.go`:

```go
package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	MatchTopN          = 3
	BodyBudget         = 8000
	DescriptionListCap = 4000
)

type Skill struct {
	Name        string
	Description string
	Body        string
	Source      string // "bundled" | "user" | "project"
	Path        string
}

type Registry struct {
	all map[string]Skill
}

// Load scans dir/*/SKILL.md. A missing dir is an empty registry, nil error.
// Broken skills are skipped (warning-worthy, never fatal).
func Load(dir string) (*Registry, error) {
	r := &Registry{all: map[string]Skill{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sp := filepath.Join(dir, e.Name(), "SKILL.md")
		data, err := os.ReadFile(sp)
		if err != nil {
			continue
		}
		m, body, ok := parseFrontmatter(string(data))
		if !ok {
			continue // broken installed skill — skip, never fatal
		}
		r.all[m.Name] = Skill{m.Name, m.Description, body, layer(dir), sp}
	}
	return r, nil
}

func layer(dir string) string {
	if strings.Contains(dir, ".fender") {
		return "project"
	}
	return "user"
}

// Merge returns a new registry with precedence project > user > r (receiver
// is the bundled registry).
func (r *Registry) Merge(project, user *Registry) *Registry {
	out := &Registry{all: map[string]Skill{}}
	for name, s := range r.all {
		out.all[name] = s
	}
	for name, s := range user.all {
		out.all[name] = s
	}
	for name, s := range project.all {
		out.all[name] = s
	}
	return out
}

func (r *Registry) ByName(name string) (Skill, bool) {
	s, ok := r.all[name]
	return s, ok
}

// Descriptions is the always-loaded catalog: one line per skill, capped.
func (r *Registry) Descriptions() string {
	names := make([]string, 0, len(r.all))
	for n := range r.all {
		names = append(names, n)
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, n := range names {
		s := r.all[n]
		line := fmt.Sprintf("- %s: %s\n", s.Name, s.Description)
		if sb.Len()+len(line) > DescriptionListCap {
			sb.WriteString("- ...(catalog truncated)\n")
			break
		}
		sb.WriteString(line)
	}
	return sb.String()
}

// PonytailCore is the always-loaded discipline (D30): the bundled ponytail
// skill, never matched, never shadowed by installs.
func (r *Registry) PonytailCore() (Skill, bool) {
	s, ok := r.all["ponytail"]
	return s, ok && s.Source == "bundled"
}

var _ = fs.ReadDirFile // (reserved; remove if unused)
```

Note: drop the `var _ = fs.ReadDirFile` line if unused — it exists only to keep the import tidy; remove `io/fs` import if unused.

- [ ] **Step 3: Run tests to verify they pass**

Run: `go test ./internal/skills/ -v`
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/skills/ CHANGELOG.md
git commit -m "feat: skills registry (Load, Merge shadowing, ByName, Descriptions, PonytailCore)"
```

CHANGELOG entry:

```markdown
### Added
- Skills registry: Load (missing dir = empty, broken skipped), Merge (project > user > bundled), ByName, Descriptions (4K cap), PonytailCore
```

---

### Task 4: Trigger matching

**Files:**
- Create: `internal/skills/match.go`
- Create: `internal/skills/match_test.go`

**Interfaces:**
- Consumes: `Registry`, `Skill` (Task 3).
- Produces:
  - `func (r *Registry) Match(message string) []Skill` — word-overlap scoring: significant words (len > 3, stopword-filtered) shared between message and description; score ≥ 2 → candidate; top `MatchTopN` (3); total body chars ≤ `BodyBudget` (8000); skips `ModelInvokable == false` skills; deterministic order (score desc, name asc).

- [ ] **Step 1: Write the failing test**

`internal/skills/match_test.go`:

```go
package skills

import (
	"strings"
	"testing"
)

func mkSkill(name, desc string, invokable bool) Skill {
	return Skill{Name: name, Description: desc, Body: "body of " + name, ModelInvokable: invokable}
}

func TestMatchFindsDiagnosingBugs(t *testing.T) {
	reg := &Registry{all: map[string]Skill{
		"diagnosing-bugs": mkSkill("diagnosing-bugs", "Diagnosis loop for hard bugs. Use when something is broken or failing.", true),
		"tdd":             mkSkill("tdd", "Test-driven development, red-green-refactor.", true),
	}}
	got := reg.Match("can you diagnose this bug, something keeps failing")
	if len(got) != 1 || got[0].Name != "diagnosing-bugs" {
		t.Fatalf("got = %+v", got)
	}
}

func TestMatchSkipsUserInvoked(t *testing.T) {
	reg := &Registry{all: map[string]Skill{
		"ask-matt": mkSkill("ask-matt", "Ask which skill fits. Use when the user wants a router.", false),
	}}
	if got := reg.Match("ask which skill fits"); len(got) != 0 {
		t.Fatalf("user-invoked skill must not auto-match: %+v", got)
	}
}

func TestMatchTopNBudget(t *testing.T) {
	reg := &Registry{all: map[string]Skill{
		"a": mkSkill("a", strings.Repeat("alpha beta gamma delta epsilon zeta eta theta ", 50), true),
		"b": mkSkill("b", "alpha beta gamma delta epsilon zeta eta theta", true),
		"c": mkSkill("c", "alpha beta gamma delta epsilon zeta eta theta", true),
		"d": mkSkill("d", "alpha beta gamma delta epsilon zeta eta theta", true),
	}}
	got := reg.Match("alpha beta gamma delta epsilon zeta eta theta")
	if len(got) > MatchTopN {
		t.Fatalf("matched %d > top %d", len(got), MatchTopN)
	}
	total := 0
	for _, s := range got {
		total += len(s.Body)
	}
	if total > BodyBudget {
		t.Fatalf("body budget exceeded: %d", total)
	}
}

func TestMatchNoHit(t *testing.T) {
	reg := &Registry{all: map[string]Skill{
		"tdd": mkSkill("tdd", "Test-driven development, red-green-refactor.", true),
	}}
	if got := reg.Match("what is the weather in kuala lumpur"); len(got) != 0 {
		t.Fatalf("got = %+v", got)
	}
}
```

- [ ] **Step 2: Write the matcher**

`internal/skills/match.go`:

```go
package skills

import (
	"sort"
	"strings"
)

// stopwords are filtered from matching tokens (small hardcoded set).
var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "when": true,
	"use": true, "you": true, "your": true, "that": true, "this": true,
	"what": true, "want": true, "from": true, "have": true, "has": true,
	"will": true, "can": true, "are": true, "was": true, "were": true,
	"into": true, "over": true, "under": true, "about": true, "than": true,
	"which": true, "where": true, "there": true, "here": true, "then": true,
	"them": true, "they": true, "does": true, "doing": true, "made": true,
	"make": true, "like": true, "just": true, "also": true, "should": true,
}

// words returns significant tokens: len > 3, not stopwords.
func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,;:!?\"'()[]{}<>/\\")
		if len(w) > 3 && !stopwords[w] {
			out[w] = true
		}
	}
	return out
}

// Match scores the message against every model-invokable skill description
// by significant-word overlap. Score >= 2 → candidate; top MatchTopN by
// (score desc, name asc); total body ≤ BodyBudget.
func (r *Registry) Match(message string) []Skill {
	msg := words(message)
	type cand struct {
		s     Skill
		score int
	}
	var cands []cand
	for _, s := range r.all {
		if !s.ModelInvokable {
			continue
		}
		desc := words(s.Description)
		score := 0
		for w := range msg {
			if desc[w] {
				score++
			}
		}
		if score >= 2 {
			cands = append(cands, cand{s, score})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return cands[i].s.Name < cands[j].s.Name
	})
	var out []Skill
	used := 0
	for i, c := range cands {
		if i >= MatchTopN || used+len(c.s.Body) > BodyBudget {
			break
		}
		out = append(out, c.s)
		used += len(c.s.Body)
	}
	return out
}
```

Note: `Skill` needs a `ModelInvokable bool` field — add it to the struct in `registry.go` (Task 3) and to `parseFrontmatter`'s construction in `Bundled()`/`Load()` (Task 2 code gains `ModelInvokable: m.ModelInvokable`).

- [ ] **Step 3: Run tests to verify they pass**

Run: `go test ./internal/skills/ -v`
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/skills/ CHANGELOG.md
git commit -m "feat: trigger matching (word-overlap, top 3, body budget, model-invokable only)"
```

CHANGELOG entry:

```markdown
### Added
- Trigger matching: Match() — significant-word overlap ≥2, top 3, 8K body budget, user-invoked skills excluded
```

---

### Task 5: Agent wiring + load_skill tool

**Files:**
- Modify: `internal/agent/agent.go`
- Modify: `internal/agent/delegate.go`
- Create: `internal/agent/agent_skills_test.go`

**Interfaces:**
- Consumes: `skills.Registry`, `Skill` (Tasks 2–4).
- Produces:
  - `Agent.Skills *skills.Registry` (nil-safe)
  - Run start: ponytail core body + descriptions + matched bodies composed into `a.System`
  - `load_skill` tool registered in `NewAgent`: args `{"name": "..."}` → body; unknown name → error string

- [ ] **Step 1: Write the failing test**

`internal/agent/agent_skills_test.go`:

```go
package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/skills"
)

func testSkillsReg() *skills.Registry {
	return &skills.Registry{All: map[string]skills.Skill{
		"ponytail": {Name: "ponytail", Description: "lazy discipline", Body: "PONYTAIL-LADDER", Source: "bundled"},
		"tdd":      {Name: "tdd", Description: "Test-driven development, red-green-refactor.", Body: "TDD-BODY", ModelInvokable: true},
	}}
}

func TestSkillsComposeSystem(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{completeReply("complete", "done")}}
	a := NewAgent(fake, newTestRegistry(t))
	a.System = "TASK"
	a.Skills = testSkillsReg()

	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "lets do test-first red-green"}})

	req := fake.last()
	sys := req.Messages[0].Content
	if !strings.Contains(sys, "PONYTAIL-LADDER") {
		t.Fatalf("ponytail core missing: %q", sys)
	}
	if !strings.Contains(sys, "TDD-BODY") {
		t.Fatalf("matched skill body missing: %q", sys)
	}
	if !strings.Contains(sys, "- tdd:") {
		t.Fatalf("descriptions catalog missing: %q", sys)
	}
}

func TestSkillsNilUnchanged(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{completeReply("complete", "done")}}
	a := NewAgent(fake, newTestRegistry(t))
	a.System = "ONLY-THIS"
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "go"}})
	if got := fake.last().Messages[0].Content; got != "ONLY-THIS" {
		t.Fatalf("nil Skills changed behavior: %q", got)
	}
}

func TestLoadSkillTool(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{
		toolReply("c1", "load_skill", `{"name":"tdd"}`),
		completeReply("complete", "done"),
	}}
	a := NewAgent(fake, newTestRegistry(t))
	a.Skills = testSkillsReg()
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "load tdd"}})
	if res == nil || res.Status != "complete" {
		t.Fatalf("result = %+v", res)
	}
	// the tool result must have been fed back to the model
	reqs := fake.all()
	last := reqs[len(reqs)-1]
	joined := ""
	for _, m := range last.Messages {
		joined += m.Content
	}
	if !strings.Contains(joined, "TDD-BODY") {
		t.Fatalf("load_skill body not returned: %q", joined)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run "TestSkills|TestLoadSkill" -v`
Expected: FAIL — `Skills` field undefined.

- [ ] **Step 3: Wire skills into the agent**

In `internal/agent/agent.go`:

```go
	Mem        *memory.Memory   // D39 ICM memory workspace; nil = ticket-04 behavior
	Skills     *skills.Registry // D27 skills; nil = ticket-05 behavior
```

Run start, after the Mem block and before the Ctx block:

```go
	if a.Skills != nil {
		if core, ok := a.Skills.PonytailCore(); ok {
			a.System = core.Body + "\n\n" + a.System
		}
		a.System = a.Skills.Descriptions() + "\n" + a.System
		userMsg := lastUserContent(msgs)
		for _, s := range a.Skills.Match(userMsg) {
			a.System += "\n[skill loaded: " + s.Name + "]\n" + s.Body
		}
	}
```

`lastUserContent` already exists in loop.go (check the name — ticket 03 has `lastUserContent(messages []Message) string`; reuse it).

In `NewAgent`, register the load_skill tool (add `a.registry.Add(a.loadSkillTool())` next to the delegate registration), with:

```go
// loadSkillTool lets the model fetch a skill body on demand (D27).
func (a *Agent) loadSkillTool() *tools.Tool {
	return &tools.Tool{
		Name:        "load_skill",
		Description: "Load a skill body by name. Skills are listed in the system prompt catalog.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []string{"name"},
		},
		Fn: func(ctx context.Context, args map[string]any) string {
			name, _ := args["name"].(string)
			if a.Skills == nil {
				return "error: no skills registry"
			}
			s, ok := a.Skills.ByName(name)
			if !ok {
				return "error: unknown skill " + name
			}
			return s.Body
		},
	}
}
```

Check `tools.Tool`'s actual shape in `internal/tools/tools.go` (Name/Description/Parameters/Fn) and match it — the delegate tool in `delegate.go` is the template.

In `delegate.go`, child Agent literal gains:

```go
				Skills:     a.Skills, // D27: delegates share the skill registry
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/ -run "TestSkills|TestLoadSkill" -v`
Expected: PASS. Then `go test ./...` — all green.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/ internal/skills/ CHANGELOG.md
git commit -m "feat: agent skills wiring (ponytail core + catalog + matched bodies, load_skill tool)"
```

CHANGELOG entry:

```markdown
### Added
- Agent wiring: nil-safe Skills — ponytail core + descriptions + matched bodies in system; load_skill tool; delegates share
```

---

### Task 6: `fender skill install`

**Files:**
- Create: `internal/skills/install.go`
- Create: `internal/skills/install_test.go`
- Modify: `cmd/fender/main.go`

**Interfaces:**
- Consumes: `Registry` (Task 3).
- Produces:
  - `func Install(src, destDir string) ([]string, error)` — src = local dir or git URL (clone to temp); copies `<src>/*/SKILL.md` dirs (support files included) into destDir; returns installed names (sorted)
  - CLI: `fender skill install <src>` — resolves target to `<cwd>/.fender/skills/`, prints installed names

- [ ] **Step 1: Write the failing test**

`internal/skills/install_test.go`:

```go
package skills

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInstallCopies(t *testing.T) {
	src := t.TempDir()
	writeSkill(t, src, "alpha", "Alpha skill.")
	writeSkill(t, src, "beta", "Beta skill.")
	// support file
	os.WriteFile(filepath.Join(src, "alpha", "notes.md"), []byte("support"), 0600)

	dest := filepath.Join(t.TempDir(), "skills")
	got, err := Install(src, dest)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("installed = %v", got)
	}
	if _, err := os.Stat(filepath.Join(dest, "alpha", "SKILL.md")); err != nil {
		t.Fatal("alpha SKILL.md missing")
	}
	if _, err := os.Stat(filepath.Join(dest, "alpha", "notes.md")); err != nil {
		t.Fatal("support file missing")
	}
}

func TestInstallIdempotent(t *testing.T) {
	src := t.TempDir()
	writeSkill(t, src, "alpha", "Alpha skill.")
	dest := filepath.Join(t.TempDir(), "skills")
	if _, err := Install(src, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(src, dest); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/skills/ -run TestInstall -v`
Expected: FAIL — `Install` undefined.

- [ ] **Step 3: Write the installer**

`internal/skills/install.go`:

```go
package skills

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Install copies every "<src>/*/SKILL.md" directory into destDir.
// src may be a local path or a git URL (cloned to a temp dir first).
// Returns the installed skill names, sorted.
func Install(src, destDir string) ([]string, error) {
	local := src
	cleanup := func() {}
	if isGitURL(src) {
		tmp, err := os.MkdirTemp("", "fender-skill-*")
		if err != nil {
			return nil, err
		}
		cleanup = func() { os.RemoveAll(tmp) }
		cmd := exec.Command("git", "clone", "--depth", "1", src, tmp)
		if out, err := cmd.CombinedOutput(); err != nil {
			cleanup()
			return nil, fmt.Errorf("clone %s: %w: %s", src, err, strings.TrimSpace(string(out)))
		}
		local = tmp
	}
	defer cleanup()

	entries, err := os.ReadDir(local)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sp := filepath.Join(local, e.Name(), "SKILL.md")
		if _, err := os.Stat(sp); err != nil {
			continue // not a skill dir
		}
		dest := filepath.Join(destDir, e.Name())
		if err := copyDir(sp, dest); err != nil {
			return nil, fmt.Errorf("install %s: %w", e.Name(), err)
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

func isGitURL(s string) bool {
	return strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "git@")
}

// copyDir copies a skill dir's files (SKILL.md + support) into dest.
func copyDir(srcSkillFile, dest string) error {
	src := filepath.Dir(srcSkillFile)
	if err := os.MkdirAll(dest, 0700); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0700); err != nil {
			return err
		}
		return copyFile(path, out)
	})
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
```

- [ ] **Step 4: Wire the CLI command**

In `cmd/fender/main.go`, extend `runCLI`'s switch:

```go
	case "skill":
		if fs.NArg() < 2 {
			return fmt.Errorf("usage: fender skill install <src>")
		}
		switch fs.Arg(1) {
		case "install":
			if fs.NArg() < 3 {
				return fmt.Errorf("usage: fender skill install <src>")
			}
			return installSkills(out, fs.Arg(2))
		default:
			return fmt.Errorf("unknown skill command %q", fs.Arg(1))
		}
```

```go
func installSkills(out io.Writer, src string) error {
	dest := filepath.Join(".fender", "skills")
	names, err := skills.Install(src, dest)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "installed %d skill(s): %s\n", len(names), strings.Join(names, ", "))
	return nil
}
```

Update `fender providers` usage line to include `skill install`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/skills/ ./cmd/fender/ -v`
Expected: all PASS. Then `go build ./...` and a manual smoke:

```bash
go build ./cmd/fender && ./fender skill install /tmp/nonexistent 2>&1 | head -2; rm -f fender
```

- [ ] **Step 6: Commit**

```bash
git add internal/skills/ cmd/fender/ CHANGELOG.md
git commit -m "feat: fender skill install (local path or git URL, copies skill dirs)"
```

CHANGELOG entry:

```markdown
### Added
- `fender skill install <src>`: copies skill dirs from local path or git URL into .fender/skills/
```

---

### Task 7: Wayfinder resolve + frontier

**Files:**
- Modify: `.scratch/fender/issues/06-Skills.md`
- Modify: `.scratch/fender/map.md`

- [ ] **Step 1: Full verification**

```bash
go build ./... && go vet ./... && go test ./...
```

Expected: build clean, vet clean, all tests PASS.

- [ ] **Step 2: Resolve the ticket** (mirror ticket 05's Answer format: what was delivered, test count, unblocks)

- [ ] **Step 3: Update the map's decisions index**

- [ ] **Step 4: Commit**

```bash
git add .scratch/fender/ CHANGELOG.md
git commit -m "docs: resolve wayfinder ticket 06 (skills done, frontier 07)"
```

CHANGELOG entry:

```markdown
### Changed
- Wayfinder: ticket 06 resolved — skills delivered; frontier → 07 (CodeIntel)
```

---

## Self-Review Notes

- **Spec coverage:** §1 scope 1–7 → Tasks 1–6; §3 decisions 1–7 → Tasks 2–6; §4 API → Tasks 2–6; §5 wiring → Task 5; §6 CLI → Task 6; §7 test table → each task's tests by name; §8 acceptance → Task 7 verification. Non-goals (§2) explicitly not built.
- **Placeholders:** none — every code step contains full source. Two flagged adaptation points: (a) `tools.Tool` shape must match `internal/tools/tools.go` (delegate tool is the template); (b) `lastUserContent` reuse from loop.go (check exact name).
- **Type consistency:** `Skill{Name,Description,Body,Source,Path,ModelInvokable}` — ModelInvokable added in Task 4 note; `Registry{all}` map; `Match`/`ByName`/`Descriptions`/`PonytailCore`/`Install` signatures consistent across tasks. The test in Task 5 uses `Registry{All: ...}` — adjust to the real field name (`all`) in the same package (agent package imports skills; `all` is unexported → use `Load`/`Bundled` or export a test helper; the implementer must reconcile — `testSkillsReg` can call `skills.Bundled()` and mutate, or the skills package gains an exported constructor for tests).
- **CHANGELOG:** every task ends with an entry + commit (hook-enforced).
- **Deps:** none added — stdlib + `os/exec` for git clone.
