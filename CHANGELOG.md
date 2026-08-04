# Changelog

All notable changes to Fender. Every commit MUST update this file (enforced by `.githooks/pre-commit`).

## [Unreleased]

### Fixed
- `fender run --config X` now honors the config file instead of falling back to ~/.fender/fender.toml (runTask threads configPath through to buildAgent); TestRunCommand pinned to a throwaway config so it never calls a real provider

### Added
- Session persistence (ticket 10, D41): `fender sessions` lists saved sessions; `fender` resumes the latest session, `fender --new` starts fresh; history saved atomically after every REPL turn and on /quit to `.fender/sessions/<id>.json` (timestamp IDs)
- `cmd/fender/session.go`: sessionFile schema, saveSession (temp+rename), loadLatestSession, listSessions (newest first); artifact-pointer compatibility documented (zero code — history carries absolute /tmp pointers that survive the 24h sweep)
- Wayfinder: ticket 10 resolved — session persistence delivered (deferred post-v1 list: Anthropic adapter, GUI, TUI, response caching)

### Added
- Spec: `docs/superpowers/specs/2026-08-04-fender-context-design.md` — ticket 04 design: internal/context artifact engineering (D31 port) + D38 in DECISIONS.md (artifact root /tmp, per-agent managers, cap policy, D9 migration seam)
- Plan: `docs/superpowers/plans/2026-08-04-fender-context.md` — ticket 04 implementation plan (7 tasks, test-first; budget-bound test as first-class task, acceptance #3)
- `internal/context`: Manager core — 8K rule (CompactOutput), artifact write (0700/0600, per-call paths), catalog, 24h Cleanup sweep (D31/D38)
- `internal/context`: CompactInput — HEAD/TAIL preservation for large user input (D31)
- `internal/context`: For() — budget arithmetic (system+Σmsgs ≤ ContextChars), turns truncation + marker, artifact catalog in context (D31)
- docs: wayfinder ticket 04 resolved (context artifact engineering complete)
- `internal/agent`: loop wired to context layer — For() at ingress, CompactOutput on tool results, dedup caches pointers (D31)
- `internal/agent`: delegate children get isolated context managers (D38)

### Changed
- `internal/tools`: shell output cap 64 KiB → 8 MiB (memory ceiling; artifact layer carries full output); read cap comment corrected (D38)
- Design session (2026-08-04): full Fender design locked in `DECISIONS.md` (D1–D37) and `docs/superpowers/specs/2026-08-04-fender-design.md`
- `AGENTS.md` — project constitution; every agent session must follow the decision log
- `.githooks/pre-commit` — enforces the changelog rule; install with `git config core.hooksPath .githooks`

### Changed
- AGENTS.md: added `BurntSushi/toml` to the allowed dependency list (D25 config implementation)

### Added
- Plan 1 (foundation): `docs/superpowers/plans/2026-08-04-fender-foundation.md` — module, fender.toml config, OpenAI-compatible provider client, registry, `fender providers` CLI (6 tasks)

### Added
- Ticket 05 spec (draft): `docs/superpowers/specs/2026-08-04-fender-memory-design.md` — ICM layers, convention-file detection, working memory, MAP.md schema; memory graph + consolidation deferred pending user review
- Module skeleton: `cmd/fender` stub, `go.mod` (github.com/H4fizWasabie/fender), BurntSushi/toml dependency
- `internal/provider` config types: fender.toml providers schema (base_url, api_key, models, default_model)
- OpenAI-compatible client: Chat() with tool_calls parsing, Bearer auth, non-200 error wrapping
- Streaming client: Stream() with SSE parsing, content deltas via callback, tool-call accumulation
- Provider registry: fender.toml loading (./ then ~/.fender/), client lookup, default provider
- `fender providers` CLI: lists configured providers, models, defaults; `--config` flag
- Wayfinder: tickets 01-02 resolved (Foundation, Guardrail); 03 (Tools+Loop) is the frontier
- Guardrail core types: strict/balanced/yolo modes, RUN/ASK/REFUSE verdicts, category x severity table; `mode` in fender.toml (D21, D23)
- Guardrail parsing substrate: mvdan.cc/sh/v3 AST helpers (word literals, call names, arg extraction) — no regex (D23)
- Guardrail detectors: destructive fs (severity by target), privilege/system, irreversible git, pipe-to-shell, runaway (fork bomb, zero-fill, infinite loop), tty hangers, protected paths, path escape (D23)
- Guardrail Judge: category x mode verdicts, strict ASK-all, yolo ASK->RUN, hard REFUSE (D21, D22)
- Guardrail audit log (JSON lines: command, verdict, timestamp) + DefaultTimeout 60s (D24)
- Plan 2 (guardrail): `docs/superpowers/plans/2026-08-04-fender-guardrail.md` — modes, sh-parser verdicts, audit (5 tasks, mvdan.cc/sh/v3@v3.10.0 pinned for go 1.22)
- Wayfinder map: `.scratch/fender/` — 8 build-phase tickets, 01 Foundation claimed, 02–08 blocked in AGENTS.md build order
- Plan 3 (tools + loop): `docs/superpowers/plans/2026-08-04-fender-tools-loop.md` — read/edit/shell/search tools + ONE agent loop (complete_task, dedup, D36 orientation, delegate subagent) (8 tasks)
- Tools core: Tool/Registry (JSON-arg dispatch, OpenAI schemas, subagent subsets via Without) + project path containment (D10)
- read_file tool (1-based offset/limit line slices, project containment) + edit_file tool (unique exact-match replace, mode-preserving) (D10)
- shell tool: Judge verdict wiring (REFUSE hard in all modes, ASK via injectable approver, RUN), audit every command, default 60s timeout, 64 KiB output cap (D11, D12, D24)
- search tool: walk-based default backend (skips .git/vendor/binary, 50-match cap) behind the Searcher seam (D10)
- Agent loop core: Agent{LLM, Tools} flat loop, complete_task completion protocol (D37), in-run tool dedup (D32), no-progress stall, max-iter cap; provider client defaults the model when omitted
- Adaptive OODA: flat loop by default, ONE orientation turn on thrash (tool errors, repeated same call, text-only no-progress), stall after orientation fails (D36)
- delegate tool: subagent-as-a-tool, same Agent type in a goroutine, per-subagent provider via Resolver (D7, D8, D13)
- Wayfinder: ticket 03 resolved (tools + agent loop); 04 (Context) is the frontier

### Added
- Ticket 05 spec approved: memory/ICM design (D39) — memory graph + consolidation deferred to D9 era

### Added
- Plan 5 (memory): `docs/superpowers/plans/2026-08-04-fender-memory.md` — workspace scaffold, convention detection, System assembly, working prune, agent wiring (6 tasks)

### Added
- `internal/memory`: Ensure() scaffold — .fender/{memory/{PROJECT.md,MAP.md,reference/,working/,facts/},skills/}, idempotent, seeded templates

### Added
- Convention detection: Detect() — user ~/.fender/AGENTS.md → project AGENTS.md (CLAUDE.md fallback) → CONTEXT.md; README/.cursorrules never auto-loaded

### Added
- Bootstrap(): Ensure + Detect + layer reads; System() assembly with provenance markers, 8K cap (oldest-first truncation), unreadable files skipped

### Added
- Working memory: pruneWorking (7-day, patterns.md exempt) in Bootstrap; Working() catalog

### Added
- Agent wiring: nil-safe Mem — Bootstrap at Run start, memory system prepend, delegates share project memory

### Changed
- Wayfinder: ticket 05 resolved — memory/ICM layers delivered; frontier → 06 (Skills)

### Added
- Ticket 06 spec: skills design (D27-D30 implementation) — go:embed 23 skills, regex frontmatter parser, word-overlap matching, shadowing, load_skill tool, skill install

### Added
- Plan 6 (skills): `docs/superpowers/plans/2026-08-04-fender-skills.md` — vendor, parser, registry, matching, agent wiring, skill install (7 tasks)

### Added
- Vendored 23 bundled skills into internal/skills/bundled/ (MIT, attribution; Codex yaml excluded)

### Added
- Skills: frontmatter parser (single-line/quoted/folded, disable-model-invocation), Bundled() go:embed loader — all 23 parse

### Added
- Skills registry: Load (missing dir = empty, broken skipped), Merge (project > user > bundled), ByName, Descriptions (4K cap), PonytailCore

### Fixed
- skills registry test: z lookup asserts ok=false (was wrongly expecting bundled source)

### Added
- Trigger matching: Match() — significant-word overlap >=2, top 3, 8K body budget, user-invoked skills excluded

### Added
- Agent wiring: nil-safe Skills — ponytail core + descriptions + matched bodies in system; load_skill tool; delegates share

### Changed
- Skills catalog: model-invokable skills only (user-invoked are slash-command-only), cap 4000 -> 6000 (full 14-skill catalog is 4508 chars)

### Added
- `fender skill install <src>`: copies skill dirs from local path or git URL into .fender/skills/

### Changed
- Wayfinder: ticket 06 resolved — skills delivered; frontier → 07 (CodeIntel)

### Added
- Ticket 07 spec: codeintel design (D16/D19/D20/D34-3) — tree-sitter extraction (Go/Python/TS), JSON incremental store, graph build, query API, MAP.md generation

### Added
- Plan 7 (codeintel): `docs/superpowers/plans/2026-08-04-fender-codeintel.md` — deps pin, extractor, store, graph, query API, MAP generation, intel CLI (7 tasks)

### Added
- codeintel skeleton: graphify node/edge schema, langSpec tables (go/python/typescript/javascript), specFor dispatch
- Pinned tree-sitter deps: go-tree-sitter v0.24.0 + grammar pseudo-versions (go1.22 compatible, cgo)

### Added
- Extractor: ExtractFile — generic walker over langSpec tables, symbol nodes, calls (INFERRED) + imports (EXTRACTED) edges

### Fixed
- codeintel langSpec: Go symbol kind is "function_declaration" (tree-sitter), not "func_declaration" — plan doc corrected (execution-caught plan bug)

### Added
- Store: Open/Refresh (mtime+size stamps, dirty-only re-extract, skip dirs), extractions cache, graph.json persist

### Added
- Graph: Build() — node union, self-edge skip (post-resolution), call resolution to same-project symbols (EXTRACTED), name-only stays INFERRED
- Store: Open/Refresh (mtime+size stamps, dirty-only re-extract, skip dirs), extractions cache, graph.json persist

### Fixed
- codeintel store prune: paths were already absolute — Join doubled them, prune deleted every stamp (execution-caught)
- codeintel graph: self-edge check moved after call resolution (A→A label resolves to self)
- codeintel store test: extraction key is the absolute path, not relative

### Added
- Query API: Search (substring), Symbols(path), Callers/Callees; GenerateMap (ticket-05 schema)

### Fixed
- codeintel GenerateMap: sections are package dirs (internal/agent), not first path segment (plan code vs plan test inconsistency — resolved toward useful grouping)

### Added
- `fender intel refresh|search|map`: incremental index, symbol search, MAP.md generation; Searcher adapter for the D10 seam

### Fixed
- codeintel searcher: SourceFile already absolute — removed double Join (execution-caught)
- intel map: Ensure() memory workspace before writing MAP.md

### Changed
- Wayfinder: ticket 07 resolved — codeintel delivered; frontier → 08 (CLI + UI). "Not yet specified" cleared: incremental index mechanics resolved

### Added
- Pinned tree-sitter dependency versions in go.mod (go1.22-compatible set)

### Added
- Ticket 08 spec: CLI + UI design — observer/streamer loop support, composition root, REPL, fender run/init (final ticket)

### Added
- Plan 8 (CLI+UI): `docs/superpowers/plans/2026-08-04-fender-cliui.md` — observer/streamer, composition root, REPL, run/init (5 tasks, final)

### Added
- Agent observer events (delta/tool/done) + optional Streamer interface; provider.Client.StreamChat; nil-safe, prior tests unchanged

### Added
- buildAgent: full subsystem wiring from fender.toml — default provider LLM, guardrail mode + audit file (~/.fender/audit.log), codeintel searcher (fallback default), skills merge, memory, context, resolver

### Added
- REPL: fender interactive mode — observer rendering (streaming deltas, tool lines), /quit /model /mode /skills /help, in-memory history

### Added
- `fender run <task>`: autonomous one-shot (reply + exit code by status)
- `fender init`: memory workspace + fender.toml scaffold, idempotent
- `fender` (no args): interactive REPL

### Fixed
- main_test: removed obsolete TestNoArgsShowsUsage (no-args now launches the REPL, covered by TestReplQuit)

### Changed
- Wayfinder: ticket 08 resolved — **Fender v1 complete** (all 8 subsystems delivered); map: destination reached, post-v1 backlog recorded

### Changed
- .gitignore: fender.toml is user-specific config (like .env) — generated by `fender init`

### Added
- Ticket 09: thinking mode spec (D40) — pi-style levels, reasoning_effort, reasoning_content parsing, /thinking

### Added
- Plan 9 (thinking): `docs/superpowers/plans/2026-08-04-fender-thinking.md` (4 tasks)

### Added
- Thinking wire format: reasoning_content parsed (stream + non-stream), reasoning_effort per level, per-model model_configs (thinking/thinking_levels), Client.SetThinking
- Observer thinking events (Streamer onThinking, dimmed-ready)

### Added
- REPL: /thinking <off|low|medium|high> — reasoning_effort control + dimmed thinking rendering (hidden at off)

### Added
- Reply-on-done rendering: complete_task answers now display in the REPL (done event carries the reply; printed when no deltas streamed)
- Wayfinder: ticket 09 resolved — thinking mode delivered (frontier: post-v1 backlog)

### Added
- Ticket 10: session persistence spec (D41) — history save/resume, artifact-pointer compatible
