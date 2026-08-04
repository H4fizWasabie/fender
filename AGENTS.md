# Fender — Agent Rules

> Every AI coding agent working on this project MUST follow these rules.
> Violations = rejected PR. No exceptions.

## First Steps (mandatory)

1. **Read `DECISIONS.md`** — the decision log (D1–D37). Every design rule below is a decision. If a decision is missing, ADD it to DECISIONS.md in the same commit — never build on an unrecorded decision.
2. **Read `docs/superpowers/specs/2026-08-04-fender-design.md`** — the assembled design spec.
3. **Reference repos live at `~/Desktop/fender-references/`** (code-context-engine, codegraph, graphify, skills, ponytail — all MIT). Read the reference BEFORE writing the port. Fender's own source is the last word — references are study material, not vendored code.
4. **Understand the philosophy**: Fender is a from-scratch Go coding agent. The harness is the guardrail. The LLM gets full freedom; safety is deterministic code. Ponytail (the ladder) is the default behavioral discipline — for Fender's own code too.

## Design Laws (from the decisions — do not violate)

- **No frameworks, no agent libraries.** Just LLM API calls + stdlib. (D1)
- **OpenAI-compatible provider layer only.** One client. Anthropic adapter NOT in v1. (D6)
- **One loop, one Agent type.** Subagent = same type in a goroutine, subagent-as-a-tool. No planner, no reflection phase, no parallel-execution machinery. (D13)
- **Adaptive OODA.** Flat loop by default; explicit orientation turn ONLY on thrash detection (tool error, repeated same call, no progress). Never add per-turn OODA ceremony. (D36)
- **Context = mino's artifact engineering.** 8K inline limit, HEAD/TAIL, read_file never compacted, write-elsewhere, isolate. Port `context_test.go` techniques as tests. (D31)
- **Caching layers 1–6 only.** Tool dedup, artifact rule, code-intel incremental index, stable-prefix prompt caching, embedding cache seam (only if embeddings ship), consolidation dedup. NO response caching (D35 — revisit only with real numbers). (D34)
- **Guardrail: modes + sh-parser verdicts.** strict/balanced/yolo from `fender.toml`; `mvdan.cc/sh/v3` AST, never regex; REFUSE is hard in ALL modes. (D21–24)
- **Skills: 23 bundled, zero custom skill format.** go:embed the 17 engineering skills + 6 ponytail skills. ponytail core = always-loaded discipline, NOT a trigger skill. Skills never bypass the guardrail. (D27–30)
- **Memory = ICM layers.** PROJECT.md always; MAP.md from code-intel; reference/ + working/ selective. Convention files (AGENTS.md/CLAUDE.md/CONTEXT.md) load DIRECTLY — never copy their content into PROJECT.md (canonical sources, no drift). (D14, D17)
- **Code-intel built in Go, from day one.** tree-sitter grammars (go-tree-sitter) + graphify pipeline (detect→extract→build→cluster→analyze→report→export). Incremental index — only changed files re-extract. (D16, D19, D20, D34)
- **mino is the PERSONAL-agent reference.** Port its context engineering and loop skeleton. Do NOT port its coding behavior — Fender's coding behavior comes from this design + the engineering skills. (D37)
- **Deferred list in the spec is a NO-BUILD list.** Session persistence, Anthropic adapter, GUI, TUI, response caching. Seams only.

## Project structure (core first, extensions last)

```
fender/
  cmd/fender/           # fender · fender run · fender init · fender skill install
  internal/
    agent/              # ONE loop + subagent-as-a-tool
    provider/           # OpenAI-compatible client + TOML registry
    tools/              # read, edit, shell, search
    guardrail/          # modes + sh-parser verdicts + timeout + audit
    skills/             # registry, trigger matching, go:embed bundles
    memory/             # ICM layers + memory graph
    codeintel/          # extract → graph → cluster → query
    context/            # artifact engineering (mino port)
    ui/                 # plain streaming renderer
  .fender/              # per-project memory workspace
  fender.toml           # providers + permission mode + tool settings
```
**Build order:** provider → guardrail → tools → agent loop → context → memory → skills → codeintel → CLI commands → UI. The loop must work with one provider, two tools, and a REPL before anything else.

## Code quality

- **Go stdlib first.** No external dependency without explicit discussion. Allowed so far: `mvdan.cc/sh/v3`, `go-tree-sitter`, `modernc.org/sqlite` (pure Go, no CGO). That's the whole list — anything else is a discussion.
- **Single binary.** Everything embedded via `embed.FS` (skills, grammars). One `go build`.
- **Readable in an afternoon.** Core modules ~100 lines each; if it's growing, split it.
- **No panic in library code.** Explicit errors, `log/slog` for logging.
- **Flat over nested** inside packages. No speculative abstractions — one implementation per interface (ponytail rule).

## Version control

- **Commit at every working milestone.** Subject says what, body says WHY.
- **CHANGELOG.md is mandatory on EVERY commit.** No changelog = no commit — enforced by `.githooks/pre-commit` (install: `git config core.hooksPath .githooks`; required after every clone). Format:
  ```
  ## [Unreleased]
  ### Added
  - Feature X (reason)
  ### Changed
  - Refactored Y (why)
  ```
- **Design changes update `DECISIONS.md` + spec in the SAME commit as the code.** No code drift from the decision log — that's the whole point of this file.
- **Branch naming:** `feat/short-description`, `fix/short-description`, `refactor/short-description`.

## Testing

- **`go test ./...` must pass before push.**
- **If you fix a bug, add a test for it.** No exceptions.
- **Port mino's `context_test.go` techniques** (budget bounds, head/tail, compaction markers, artifact catalog) — they become Fender's context test suite.
- **Guardrail verdict table tests** — every category × mode combination has a case.
- **Port against upstream fixtures:** graphify `worked/`, codegraph `__tests__/`.
- Table-driven tests, stdlib `testing`.

## Scope discipline

- **No feature creep.** Check the decision log + spec before proposing anything new.
- **No unrecorded decisions.** New design direction → add to DECISIONS.md first, then discuss, then build.
- **One task per commit.** If it takes more than an afternoon, split it.
- **When in doubt, ask the ladder:** does this need to exist at all? Is there a stdlib answer? Can it be one line? The laziest working solution is the right one — for Fender itself, always.
