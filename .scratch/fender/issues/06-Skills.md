# 06-Skills

Type: task
Status: resolved
Blocked by: 05
Resolved: 2026-08-04

## Question

Write + execute Plan 6: skills — 23 bundled (17 engineering + 6 ponytail) via go:embed, zero custom skill format, registry + trigger matching. ponytail core = always-loaded discipline, NOT a trigger skill. Skills never bypass the guardrail. `fender skill install`.

## Answer

Plan 6 done: internal/skills — 23 skills vendored (MIT, attribution, Codex yaml excluded), regex frontmatter parser (single-line/quoted/folded, disable-model-invocation), Bundled() embed loader (all 23 validated), Registry (Load/Merge shadowing project>user>bundled/ByName/Descriptions/PonytailCore), Match (word-overlap ≥2, top 3, 8K body budget, model-invokable only), agent wiring (nil-safe Skills, ponytail core + catalog + matched bodies in system), load_skill tool, `fender skill install` (local path or git URL). 2 design fixes during execution: catalog is model-invokable-only (user-invoked = slash-command-only) with cap 4000→6000 (full 14-skill catalog is 4508 chars). 29 tests in skills+agent (123 total). Unblocks 07.
