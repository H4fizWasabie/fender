# Fender — Skills Design (Ticket 06)

**Date:** 2026-08-04
**Status:** Approved by decision log (D27–D30); this spec is the Fender implementation.
**Supersedes:** spec §3.5 placeholder.

23 skills bundled natively (17 engineering from mattpocock/skills + 6 ponytail, all MIT), zero custom skill format (standard SKILL.md), registry with lookup order, deterministic trigger matching, ponytail core = always-loaded discipline. Skills are instructions — they never bypass the guardrail (D29).

---

## 1. Scope

Build `internal/skills`:

1. **Bundled skills via `go:embed`** — skill directories copied into `internal/skills/bundled/` from `~/Desktop/fender-references/` (skills + ponytail repos, MIT — attribution in `bundled/LICENSE.md`). Only `SKILL.md` + support files (scripts/, references) — `agents/*.yaml` (Codex invocation policy) is dropped.
2. **SKILL.md parsing** — YAML frontmatter (`name`, `description` single-line or folded) + markdown body. Parse errors → skill skipped with a warning, never a hard error.
3. **Registry + lookup order** — project `.fender/skills/` → user `~/.fender/skills/` → bundled. Same name at a higher-precedence layer shadows lower layers.
4. **Always-loaded layer** — descriptions (23 × one line) + ponytail core body. Rides in the system prompt (D28: descriptions always, bodies selective).
5. **Trigger matching (harness-decided, D28)** — at Run start, the harness matches the user message against descriptions and loads matched bodies (top N, bounded). Deterministic, testable. Plus a `load_skill` tool so the model can fetch additional bodies on demand (model-invoked, D27).
6. **User-invoked** — `ByName(name)` for slash commands (`/tdd`, `/code-review`); the CLI surface is ticket 08.
7. **`fender skill install <repo|path>`** — copies skill directories from an external repo/path into the project `.fender/skills/`. Installed skills take precedence over bundled (lookup order).

## 2. Non-goals (v1)

- Skill authoring tooling (write SKILL.md by hand — the format is the tool)
- Skill marketplace/versioning/updates (install is a copy; git is the versioning)
- MCP tools (spec's MCP note: separate layer, not v1)
- Per-skill permission configuration — skills never bypass the guardrail (D29)
- Nested skill subdirectories (one level: `<dir>/SKILL.md`)
- Skill trigger phrases from frontmatter beyond `description` (description IS the trigger text)

## 3. Decisions

| # | Decision |
|---|----------|
| 1 | **Bundling = vendoring skill files into the repo** (MIT, attribution note). go:embed at `internal/skills/bundled/`. Single binary holds all 23. |
| 2 | **ponytail core is special** (D30): always-loaded in full, never trigger-matched, never shadowed by installs (it's the default discipline, not a library skill). Its description still appears in the always-loaded list (the model should know it's active). |
| 3 | **Matching = word-overlap scoring** (mino `MatchingSkills` parity): tokenize message + description, score shared significant words (len > 3, stopword-filtered), threshold ≥ 2 shared, top 3 loaded, total loaded body budget ≤ 8K chars. Deterministic — the harness decides what enters context (prevention over compression). |
| 4 | **`load_skill` tool** — model-invoked fallback: takes a skill name, returns the body (bounded 8K, artifact-pointer if larger). Registered with the tools registry at agent construction (like `delegate`). |
| 5 | **Shadowing** — project `.fender/skills/<name>/` shadows user and bundled; user shadows bundled. `ByName` and matching both resolve through the same lookup. |
| 6 | **`fender skill install <src>`** — src = local path or git repo URL; copies `<src>/*/SKILL.md` dirs into `.fender/skills/`. Idempotent; existing names are overwritten (install is explicit user intent). |
| 7 | **Parse resilience** — a broken installed skill is skipped with a warning; bundled skills are validated at build time by tests (all 23 must parse). |

## 4. Module API — `internal/skills`

```go
package skills

const (
    MatchTopN     = 3     // max bodies auto-loaded per message
    BodyBudget    = 8000  // max total matched-body chars in context
    DescriptionListCap = 4000
)

type Skill struct {
    Name        string
    Description string
    Body        string
    Source      string // "bundled" | "user" | "project"
    Path        string // where it was loaded from
}

type Registry struct { ... }

func Bundled() (*Registry, error)      // go:embed bundled/, validate all parse
func Load(dir string) (*Registry, error) // load .fender/skills or ~/.fender/skills dir (may be empty → empty registry, no error)
func (r *Registry) Merge(project, user *Registry) *Registry // project > user > r(bundled); ponytail core always bundled copy
func (r *Registry) ByName(name string) (Skill, bool)         // user-invoked (slash commands)
func (r *Registry) Match(message string) []Skill             // word-overlap scoring, MatchTopN, BodyBudget
func (r *Registry) Descriptions() string                     // always-loaded list, capped
func (r *Registry) PonytailCore() (Skill, bool)              // always-loaded discipline (D30)
func Install(src, destDir string) ([]string, error)          // copy <src>/*/SKILL.md dirs → destDir; returns installed names
```

Notes:
- `Load` on a non-existent dir returns an empty registry, nil error (missing `.fender/skills/` is normal).
- Frontmatter parsing: minimal YAML (name + description) via `BurntSushi/toml`? No — TOML ≠ YAML. Frontmatter is YAML. Allowed-deps list has no YAML lib. Two options: (a) regex-extract the two fields from frontmatter (frontmatter is tiny and regular: `name: x`, `description: y` possibly folded `>`), (b) add `gopkg.in/yaml.v3` to the allowed list. **Decision: (a) regex frontmatter parser** — the format is narrow (name + description, single-line or folded), and adding a YAML dep for two fields violates the ponytail ladder. If a skill's frontmatter doesn't parse, skip with warning (resilience). Tested against all 23 real frontmatters (acceptance).
- Trigger matching uses the same stopword list as… nothing exists yet; a small hardcoded stopword set lives in `match.go`.

## 5. Agent wiring (`internal/agent`)

- `Agent` gains `Skills *skills.Registry` (nil-safe, ticket-05 pattern).
- `Run` start (after Mem, before Ctx): 
  ```
  if a.Skills != nil {
      list := a.Skills.Descriptions()
      core, _ := a.Skills.PonytailCore()
      a.System = core.Body + "\n" + list + "\n" + a.System  // discipline + catalog first
      for _, s := range a.Skills.Match(lastUserMessage) {
          a.System += "\n[skill loaded: " + s.Name + "]\n" + s.Body  // bounded by BodyBudget
      }
  }
  ```
- `load_skill` tool registered in `NewAgent` (alongside `delegate`): name → body (8K inline, artifact pointer beyond — same `CompactOutput` path as other tools when `Ctx` is set).
- Delegates inherit `Skills` (same project).
- Always-loaded cap: ponytail core + descriptions + matched bodies are bounded (BodyBudget on matched; core+descriptions ~2K) — total well under memory's SystemCap interaction; matched bodies count against context via ticket-04's normal budget arithmetic (they ride in System, which For() budgets).

## 6. `fender skill install`

- `cmd/fender/main.go` gains `skill install <src>` (ticket-08-style minimal command now, per the ticket brief):
  - `<src>` local path or `https://github.com/…` (clone to temp, install from clone)
  - Install target: project `.fender/skills/` (Ensure() from memory already creates the dir — skills.Install creates it if missing)
  - Output: installed skill names
- Tests: install from a temp fixture dir; shadowing test (installed name beats bundled).

## 7. Test strategy

| Test | Technique |
|------|-----------|
| All 23 bundled skills parse (name + description non-empty) | build-time validation |
| Frontmatter folded description (`>`) parses (ponytail) | parser |
| Parse resilience: broken frontmatter → skipped, no error | resilience |
| Match: "fix this bug" → diagnosing-bugs; "test-first red-green" → tdd; no match → empty | trigger matching |
| Match budget: >3 matches → top 3, body total ≤ BodyBudget | bounding |
| Lookup shadowing: project name beats user beats bundled | precedence |
| ByName: slash-command lookup incl. shadowed resolution | user-invoked |
| Install: fixture dir → copied; names returned; idempotent re-install | install |
| Agent end-to-end: Skills set → system contains ponytail core + descriptions + matched body; nil → unchanged | integration, nil-safety |
| load_skill tool: request by name → body returned; unknown → error message | tool |

## 8. Acceptance criteria

1. `go test ./...` green, `go vet ./...` clean, single binary builds (embed verified).
2. All ticket-03/04/05 tests pass unchanged (nil-safe).
3. All 23 bundled skills parse (test-enforced); binary size includes the skills (embed verified via `go build`).
4. Trigger matching is deterministic and bounded; no skill body > BodyBudget enters context at Run start.
5. `fender skill install` works from a local path; installed skill shadows bundled.
6. `CHANGELOG.md` updated on every commit (hook-enforced).
7. Wayfinder ticket 06 resolved; frontier → 07 (CodeIntel).

## 9. Deferred (with seams)

| Item | Decision | Seam |
|------|----------|------|
| Skill marketplace / versioning | — | `Install(src)` takes any path/URL; git is versioning |
| Skill authoring tools | — | SKILL.md is the format; `fender skill install` accepts local dirs for dogfooding |
| MCP tools | spec note | separate layer, not v1 |
| Per-skill permissions | D29 (never) | guardrail is global by design |
| Slash-command UX | ticket 08 | `ByName(name)` is the seam |
