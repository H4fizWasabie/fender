# Fender — Session Persistence Design (Ticket 10, D9 delivered)

## Scope

1. **Save** — after every REPL turn (and on /quit), write `state.history` to `.fender/sessions/<id>.json` (`{"id": ..., "started": ..., "messages": [...]}`). Id = timestamp-based (`20260804-235959`).
2. **Resume** — `fender` (no args) loads the latest session file if present (else starts fresh). `fender --new` forces a fresh session. `fender sessions` lists saved sessions (id, started, message count).
3. **Artifact compatibility** — free: history carries absolute artifact pointers; /tmp artifacts survive the 24h sweep, so resumed sessions fetch slices identically. Documented, zero code.
4. **Scope limit** — the context Manager's catalog is per-run (rebuilds from history pointers); no session-specific state beyond the message list. D9's "seam" honored.

## API

```go
// cmd/fender/session.go
type sessionFile struct {
    ID       string                 `json:"id"`
    Started  string                 `json:"started"`
    Messages []provider.Message     `json:"messages"`
}
func loadLatestSession() (*sessionFile, error)   // nil when none
func saveSession(s *sessionFile) error           // atomic-ish: write temp + rename
func listSessions() ([]sessionFile, error)
```

## Tests

| Test | Technique |
|------|-----------|
| save → load round-trip (messages preserved) | temp .fender dir |
| loadLatest picks the newest (by ID) | two files |
| none → nil, no error | empty dir |
| REPL: history loaded at boot, saved after turn (feed task via fake — save asserted on /quit) | repl integration |
| `fender sessions` lists | CLI |

## Acceptance

1. All prior tests unchanged (persistence is additive; `.fender/` gitignored already).
2. `fender` resumes; `--new` starts fresh; sessions list works.
3. CHANGELOG per commit; wayfinder ticket 10 resolved.
