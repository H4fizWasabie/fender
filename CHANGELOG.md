# Changelog

All notable changes to Fender. Every commit MUST update this file (enforced by `.githooks/pre-commit`).

## [Unreleased]

### Added
- Spec: `docs/superpowers/specs/2026-08-04-fender-context-design.md` — ticket 04 design: internal/context artifact engineering (D31 port) + D38 in DECISIONS.md (artifact root /tmp, per-agent managers, cap policy, D9 migration seam)
- Plan: `docs/superpowers/plans/2026-08-04-fender-context.md` — ticket 04 implementation plan (7 tasks, test-first; budget-bound test as first-class task, acceptance #3)
- `internal/context`: Manager core — 8K rule (CompactOutput), artifact write (0700/0600, per-call paths), catalog, 24h Cleanup sweep (D31/D38)
- `internal/context`: CompactInput — HEAD/TAIL preservation for large user input (D31)
- `internal/context`: For() — budget arithmetic (system+Σmsgs ≤ ContextChars), turns truncation + marker, artifact catalog in context (D31)
- `internal/agent`: loop wired to context layer — For() at ingress, CompactOutput on tool results, dedup caches pointers (D31)
- `internal/agent`: delegate children get isolated context managers (D38)
- Design session (2026-08-04): full Fender design locked in `DECISIONS.md` (D1–D37) and `docs/superpowers/specs/2026-08-04-fender-design.md`
- `AGENTS.md` — project constitution; every agent session must follow the decision log
- `.githooks/pre-commit` — enforces the changelog rule; install with `git config core.hooksPath .githooks`

### Changed
- AGENTS.md: added `BurntSushi/toml` to the allowed dependency list (D25 config implementation)

### Added
- Plan 1 (foundation): `docs/superpowers/plans/2026-08-04-fender-foundation.md` — module, fender.toml config, OpenAI-compatible provider client, registry, `fender providers` CLI (6 tasks)
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
