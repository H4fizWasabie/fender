# Changelog

All notable changes to Fender. Every commit MUST update this file (enforced by `.githooks/pre-commit`).

## [Unreleased]

### Added
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
- Wayfinder: ticket 01 (Foundation) resolved — Plan 1 shipped; ticket 02 (Guardrail) claimed
- Guardrail core types: strict/balanced/yolo modes, RUN/ASK/REFUSE verdicts, category x severity table; `mode` in fender.toml (D21, D23)
- Guardrail parsing substrate: mvdan.cc/sh/v3 AST helpers (word literals, call names, arg extraction) — no regex (D23)
- Guardrail detectors: destructive fs (severity by target), privilege/system, irreversible git, pipe-to-shell, runaway (fork bomb, zero-fill, infinite loop), tty hangers, protected paths, path escape (D23)
- Guardrail Judge: category x mode verdicts, strict ASK-all, yolo ASK->RUN, hard REFUSE (D21, D22)
- Plan 2 (guardrail): `docs/superpowers/plans/2026-08-04-fender-guardrail.md` — modes, sh-parser verdicts, audit (5 tasks, mvdan.cc/sh/v3@v3.10.0 pinned for go 1.22)
- Wayfinder map: `.scratch/fender/` — 8 build-phase tickets, 01 Foundation claimed, 02–08 blocked in AGENTS.md build order
