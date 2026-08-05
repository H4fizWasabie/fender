# Changelog

All notable changes to Fender. Every commit MUST update this file (enforced by `.githooks/pre-commit`).

## [Unreleased]

### Added
- Centered Docket browser workbench (D51): new-session-first task surface, explicit resumable session index, live activity/evidence slips, responsive layout, accessible states, and truthful completion handoff.
- Dashboard state/session/approval APIs so one browser session keeps a stable ID, old sessions can be resumed explicitly, and strict/balanced shell holds can be approved or denied from the workbench.
- `PRODUCT.md` plus Impeccable surface brief, approved comp, prompts, and design evidence for the redesigned workbench.
- Marketing visual archive under `docs/marketing/fender-workbench/`, preserving all direction sketches, composition studies, and desktop/mobile render passes with truth-safe usage notes.

### Changed
- Dashboard runtime and browser code are split into focused state, session, approval, event, HTTP, docket, evidence, and drawer modules; config selection now has one provider-owned path resolver.
- Fallback thinking incompatibility is now an explicit warning instead of a discarded library error; governing D51 status is synchronized across the decision log and assembled spec.
- Fender now opens a fresh session by default in both browser and terminal; CLI resumption is explicit through `--resume <id|latest>` (D51 supersedes D41's auto-resume default).
- Agent model reconciled (D50): exactly one persistent main agent; `delegate` now runs one synchronous ephemeral child on the same provider fallback chain with fresh conversation, artifact context, and memory handle; child delegation and child session persistence are unavailable by construction.
- Tool calls execute sequentially in model order again; broad D48 parallel dispatch is superseded until measured evidence justifies a new decision.
- Governing docs and terminology now distinguish main agent, child agent, project memory, working state, and provider fallback.
- gofmt: reformatted 6 files that had drifted (internal/agent/agent.go, internal/agent/delegate.go, internal/codeintel/store_test.go, internal/guardrail/verdict.go, internal/provider/client.go, internal/provider/config.go)

### Added
- Provider fallback (D50): optional top-level `fallback = "provider-name"` retries a failed model request through a separately configured provider/API key; streaming retries only before output begins.

### Removed
- One-shot `ask` model calls, provider-selectable child agents, and provider-based child identity (D47–D49 superseded by D50).

### Fixed
- Dashboard context meter now shows pi-style cache hit and window usage, warns near the context limit, keeps inline tool output visually bounded, and hides injected skill-body messages from the conversation.
- Dashboard settings now loads its click controller and has a real drawer/scrim state; the desktop workbench is viewport-contained so growing evidence scrolls inside its lane instead of pulling the whole page.
- Centered Docket reload/resume now reconstructs completion only for explicit terminal states, atomically couples durable completion evidence to terminal session persistence, labels abandoned `working` sessions as editable interruptions, persists real tool/approval/completion evidence, exposes bounded tool-result previews behind disclosure, and reports session-write failures instead of silently claiming resumability.
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

### Added
- Session persistence (D41/ticket 10): save after every turn + quit, auto-resume latest, --new flag, fender sessions

### Added
- Ticket 11: Anthropic adapter spec (D42) — api = "anthropic", Messages API translation, SSE

### Added
- Anthropic adapter (D42/ticket 11): api = "anthropic" provider — Messages API translation, SSE streaming, tool_use/tool_result blocks, thinking_delta mapping

### Added
- Ticket 12: dashboard GUI (D2 pragmatic) — embedded static UI, SSE observer broadcast, HTTP message API

### Added
- Dashboard GUI (ticket 12): embedded single-file web UI, SSE observer broadcast, HTTP message API, shared sessions

### Fixed
- Streamed responses now set Role:"assistant" — empty-role message poisoned the second request (400 from strict gateways); regression test added

### Added
- Ticket 13: consolidation spec (D43) — session-end distillation to facts/ + episodes

### Added
- Consolidation (D43/ticket 13): session-end small-model distillation → facts (dedup .md files) + episodes; background at quit/EOF

### Added
- REPL polish (ticket 14): status colors (green/yellow/red), prompt shows mode + model (dimmed)

### Changed
- Wayfinder: tickets 13+14 resolved — consolidation + polish delivered; post-v1 review recorded (D44: response caching stays deferred with measured data)

### Fixed
- agent.Event JSON tags: dashboard SSE was marshaling uppercase field names — the browser switch (ev.kind) never matched, UI rendered nothing; regression test added

### Fixed
- Dashboard rendering: deltas accumulate into one element per stream (was one div per SSE chunk = word-per-line); thinking whitespace collapsed; show-thinking toggle

### Added
- Code-intel is automatic (D45): session-build incremental refresh, always-symbol-aware search, agent-callable intel_refresh tool, fender init builds index + MAP.md

### Added
- Ticket 15: repo gaps — nested AGENTS.md (read/edit prepend dir-scoped rules), dashboard port-conflict message, README, MIT LICENSE, GitHub Actions CI, v0.1.0 tag, repo made public

### Added
- Subagent provider routing completed (D47): thinking-level propagation to children, delegate result names the provider (via <provider> / via parent-model); live-verified parent→zen subagent

### Deferred
- Parallel subagent dispatch (D8): loop is sequential by design; N-goroutine join needs its own ticket

### Fixed
- context.Manager thread-safety: mutex on catalog/turn; fixed self-deadlock (record re-lock) and copylocks deadlock (Child() copied the locked mutex) — race detector clean across all packages

### Added
- Parallel subagent dispatch (D48): concurrent tool calls per turn (goroutines + join), context Manager thread-safe (deadlock fixes, race-clean)
- Main vs subagent identity: source-tagged events ([subagent:zen]) in REPL + dashboard, live subagent streaming
- Config `subagent = "provider"`: default provider for delegates without explicit provider

### Changed
- Wayfinder: ticket 16 resolved — parallel subagents live-verified with two different account keys (parent zen → subagents zen-2, SUB-A/SUB-B returned)

### Added
- `ask` tool (D49): one-shot call to another provider/key — the call IS the subagent (no nested loop/tools/memory); parallel asks in one response; default provider from config `subagent =`; live-verified cross-key (OPINION-A OPINION-B)

### Changed
- Wayfinder: ticket 17 resolved — ask tool delivered (D49)

### Added
- Dashboard settings (D51): ⚙ gear → settings drawer — providers (add key/edit/remove, reasoning toggle), guardrail mode, fallback provider; masked keys, blank-preserves-key, dangling-fallback auto-clear, live agent rebuild after save

### Added
- Provider `path` config (default "/v1"; OpenRouter needs "/api/v1") — fixes double-/v1 class of bugs at the config level
- OpenRouter `reasoning` field parsed alongside `reasoning_content` (thinking display works on OpenRouter deepseek)
- User config: OpenRouter primary (deepseek-v4-flash-0731), zen-2 fallback; verified live

### Chore
- Removed stray test log files from the repo + gitignored *.log

### Added
- Pre-push hook: blocks commits containing API-key patterns (sk-or-v1-, long sk- keys) + optional ~/.fender/blocked-keys.txt blocklist; GitHub secret scanning + push protection enabled

### Fixed
- Settings save preserves provider path (dropping it re-broke /api/v1 providers like OpenRouter); path field shown in the settings form

### Fixed
- Runaway-loop (D52): shell commands normalized for dedup (cosmetic variants collapse), failing commands dedup too — the 505× go test loop is dead; thrash detection catches re-runs after ~3 variants

### Fixed
- fender init no longer writes a placeholder config that shadows the global ~/.fender/fender.toml (the "re-add your key" trap); prints providers-from-global instead

### Fixed
- fender init respects the global config: no placeholder config that shadows ~/.fender/fender.toml; template updated (correct base_url/path, current model)

### Fixed
- init test: isolates HOME (no global config) so the template path is exercised; the global-config path covered by the new test

### Fixed
- Conversational turns (D53): prose accepted as the answer after two nags — the model asking you a question now ends the turn instead of locking the input
- Dashboard Stop button: abort the run anytime (AbortController cancels the agent context)

### Added
- max_iterations config (D54): loop cap per fender.toml, 0 = 30; safe after D52/D53 loop protections

### Changed
- Context knobs (D55): inline 8K→16K, history 5→20 turns, budget 100K→200K — minos Telegram constraints dropped for coding-agent needs; configurable via inline_limit/history_turns/context_chars

### Changed
- pi-style context (D56): artifact machinery removed (inline everything), real token meter (CH/usage/window), cache-correct skill injection, /compact + near-limit warning
