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
- Wayfinder map: `.scratch/fender/` — 8 build-phase tickets, 01 Foundation claimed, 02–08 blocked in AGENTS.md build order
