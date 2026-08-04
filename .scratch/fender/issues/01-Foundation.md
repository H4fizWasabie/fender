# 01-Foundation

Type: task
Status: resolved
Blocked by:

## Question

Execute Plan 1 (`docs/superpowers/plans/2026-08-04-fender-foundation.md`): Go module, fender.toml config types, OpenAI-compatible client (non-streaming + SSE), registry, `fender providers` CLI. 6 tasks, each test-first, ends with a changelog'd commit. Repo now lives at `github.com/H4fizWasabie/fender` (private).

## Answer

Plan 1 executed — all 6 tasks, test-first, each a changelog'd commit (6b7d744…a26c912, pushed to origin/master):

- Go module `github.com/H4fizWasabie/fender` + BurntSushi/toml v1.6.0, `cmd/fender` stub
- `internal/provider`: Config/Provider types (fender.toml schema), OpenAI-compatible client (Chat + SSE Stream, tool_calls, Bearer, error wrapping), registry (Load/LoadDefault/Client/Names/Default)
- `fender providers` CLI with `--config` flag

Three plan corrections (plan code contradicted its own tests; tests are the spec):
1. Stream() panicked on first chunk (`choices[0]` before len check) — fixed ordering
2. Stream() appended tool-call fragments as separate calls — now merged by index per OpenAI streaming semantics
3. `Model()` falls back to first model; `Default()` only counts explicit `default_model` (with any-model fallback)

11 tests, `go vet` clean, single binary. Unblocks ticket 02 (Guardrail).
