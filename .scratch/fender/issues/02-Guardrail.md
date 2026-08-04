# 02-Guardrail

Type: task
Status: resolved
Blocked by: 01

## Question

Write + execute Plan 2: guardrail package — strict/balanced/yolo modes from fender.toml, `mvdan.cc/sh/v3` AST shell-command verdicts (never regex), REFUSE hard in all modes, timeout, audit log. Table tests: every category × mode combination.

## Answer

Plan 2 executed — all 5 tasks, test-first, changelog'd commits (828e440…495c64d):

- `internal/guardrail`: Mode/Verdict/Category + verdict table, mvdan.cc/sh/v3 AST substrate, 8 category detectors (destructive fs w/ severity by target, privilege, irreversible git, pipe-to-shell, runaway, tty hangers, protected paths, path escape), `Judge` (strict ASK-all, yolo ASK→RUN, hard REFUSE incl. parse errors), JSON-lines audit log, DefaultTimeout 60s
- `fender.toml` gains top-level `mode`; `mvdan.cc/sh/v3@v3.10.0` pinned (newest go 1.22-compatible release)

4 plan bugs fixed during execution (tests were the spec):
1. `classifyWriteTarget` used `projectDir` without a parameter — added it
2. `git clean -fdx` missed — combined short-flag clusters now matched via `hasShortFlag`
3. `args[1:]` panicked on bare `python`/`cat` and skipped the first arg — indexing fixed
4. Verdict table's benign-in-strict entry said RUN; D21 says strict ASKs everything — fixed to ASK

27 tests, vet clean, single binary. Consumed by Plan 3 (tools + loop): shell tool wires Judge + Audit + DefaultTimeout.
