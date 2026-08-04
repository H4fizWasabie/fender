# Fender — Context Artifact Engineering Design (Ticket 04)

**Date:** 2026-08-04
**Status:** Approved (brainstorm session, ticket 04)
**Supersedes:** spec §3.8's "thin interface" placeholder — D31 resolved the methodology; this spec is its Fender implementation.

Fender's conversation/tool-loop context methodology, ported from mino's artifact engineering (D31, D33): 8K inline limit, HEAD/TAIL, read_file never compacted, write-elsewhere, isolate. Port `context_test.go` techniques as Fender's context test suite.

---

## 1. Scope

Build `internal/context`: the artifact layer that keeps the model's context small and bounded.

- Tool output > 8K chars → one-line artifact pointer, full content written to an isolated file, fetched on demand via `read_file(offset, limit)`.
- `read_file` output is NEVER compacted (D31: "the one tool never compacted") — its result is the explicit slice the model asked for.
- Large user input → HEAD/TAIL split inline, full content artifacted.
- History → turns-based truncation with a compaction marker (mino `ContextMessages`).
- Budget arithmetic (`ContextFor`): system + artifact catalog + user context + history ≤ `ContextChars`.
- Artifact catalog rides in context, capped (2K chars), listing live artifacts so the model knows what it can fetch.
- Stale pruning: 24h maxAge sweep.
- Wire into the ONE agent loop (ticket 03) and the delegate tool (subagents get isolated context).

## 2. Non-goals (v1)

- **Session persistence** (D9): catalogs are in-memory per run. Artifact files persist on disk for the sweep window only.
- **Background consolidation via small model** (D32 layer 6): deferred with the memory module (ticket 05).
- **Provider prompt caching** (D34 layer 4): later ticket (system-prompt stability is already satisfied — dynamic content appends at the tail).
- **Response caching** (D35): never in v1.
- **Turns-truncation recovery**: the compaction marker says "not retained" honestly — there is no recall seam (memory is ticket 05).

## 3. Decisions

| # | Decision |
|---|----------|
| 1 | **Artifact root: `/tmp/fender/artifacts/<runID>/`** — NOT `.fender/working/`. Raw tool dumps must never pollute the project tree (accidental-commit risk; the constitution forbids it on principle). `.fender/working/` (ICM Layer 4) is for *durable deliberate notes* the agent chooses to write — that is ticket 05's job, not this layer's. |
| 2 | **Per-agent managers.** Parent and every subagent get their own `Manager` (fresh runID + catalog). Isolation per D31's isolate pillar; no shared mutable state across goroutines. The child's final reply reaches the parent as a normal tool result (compacted if >8K). |
| 3 | **Path shape: `<Root>/<n>/<tool>.txt`** — `n` is an internal per-manager counter, so repeated calls of the same tool never collide or overwrite (mino's per-turn dirs, without threading loop-iteration numbers through the API). |
| 4 | **Cap policy.** `shell.go` output cap 64 KiB → **8 MiB**: 64 KiB was lossy truncation — the artifact layer now carries full output to the model; 8 MiB is a memory ceiling for pathological output. `read.go` 1 MiB cap **stays** as the inline safety ceiling (read_file is never compacted, so the cap is the only bound on inline size). |
| 5 | **Defaults** (mino parity): `ContextChars = 100_000`, `MaxHistoryTurns = 5` (0 = default 5; unlimited not needed in v1 — budget arithmetic bounds anyway), `InlineLimit = 8000`, `PreviewLimit = 8000`, sweep `24h`. |
| 6 | **Seam (D9 migration):** when session persistence lands, migrating artifacts into `.fender/working/` is a path-only move — the pointer format already carries the full absolute path, so nothing in the format breaks. Recorded here so that future session knows the path exists. |

## 4. Module API — `internal/context`

Package imports `internal/provider` for `Message` (no new dependencies).

```go
const (
    InlineLimit    = 8000   // tool output / user input inline ceiling
    PreviewLimit   = 8000   // HEAD/TAIL preview budget for compacted user input
    DefaultChars   = 100_000
    DefaultTurns   = 5
)

type Artifact struct {
    Label string // tool name or "user input"
    Path  string
    Size  int
}

type Manager struct {
    Root            string // artifact root (default /tmp/fender/artifacts/<runID>)
    ContextChars    int    // 0 -> DefaultChars
    MaxHistoryTurns int    // 0 -> DefaultTurns
    runID           string
    turn            int    // internal counter -> path <Root>/<n>/<tool>.txt
    catalog         []Artifact
}

func New() *Manager                     // default root under /tmp/fender/artifacts/<random hex>
func (m *Manager) Child() *Manager      // same settings, fresh runID + catalog + turn
func (m *Manager) CompactOutput(tool, output string) string
    // read_file -> passthrough. len <= InlineLimit -> passthrough.
    // else write <Root>/<n>/<tool>.txt (0700 dir, 0600 file), record catalog,
    // return "[artifact: <tool> → <N> chars at <path>; use read_file with offset and limit]".
    // Write failure -> first InlineLimit chars + "\n[artifact write failed]" (never silent truncation).
func (m *Manager) CompactInput(input string) (string, Artifact)
    // len <= PreviewLimit -> (input, zero Artifact).
    // else write <Root>/<input-<nanos>>/user.txt, return
    // "[large user input: <N> chars at <path>; use read_file with offset and limit]\nHEAD:\n...\n...\nTAIL:\n..."
    // (head = PreviewLimit/2, tail = PreviewLimit - head), plus the Artifact for the caller to record.
func (m *Manager) For(system string, msgs []provider.Message) []provider.Message
    // Budget arithmetic (mino ContextFor):
    //   available  = ContextChars - len(system) - len(catalog)
    //   preview    = min(PreviewLimit, max(512, available/4))
    //   1. compact each user message > PreviewLimit via CompactInput, recording artifacts
    //   2. replace > PreviewLimit history messages with "[Large previous <role> message (<N> chars) retained in the session artifact catalog.]"
    //   3. turns truncation: keep last MaxHistoryTurns*2 messages, prepend "[<K> earlier turns compacted; full content is not retained.]"
    //   4. if catalog non-empty: append catalog (<= 2000 chars) as an assistant message
    //   5. append the (compacted) current user message last
    // Budget bound (ContextChars) is guaranteed for the returned list.
func (m *Manager) Cleanup(maxAge time.Duration)
    // remove Root entries older than maxAge (mino CleanupArtifacts)
func (m *Manager) Catalog() string     // capped (2000 chars) rendering for step 4
```

Notes:
- `Manager` is a struct, not an interface — one implementation (ponytail rule).
- `For` guarantees `len(system) + Σ len(msgs.Content) ≤ ContextChars`.
- Catalog cap 2000 chars (mino `SessionArtifacts(sessionID, 2000)`).
- Artifact recording dedupes by path (INSERT-OR-REPLACE analog in memory).
- Catalog timing: `For` renders the catalog snapshot at ingress. Tool-result artifacts created mid-run are recorded but not re-listed (their pointers already sit inline in the tool results); the catalog refreshes on the next `For` call (per user turn, Plan 8 interactive mode).

## 5. Agent loop wiring (`internal/agent`)

- `Agent` gains `Ctx *context.Manager` — **nil-safe**: nil = ticket-03 behavior (existing tests untouched).
- `Run` start: `if a.Ctx != nil { msgs = a.Ctx.For(a.System, msgs) }` before the existing system-prepend. This compacts the incoming task, truncates history, and appends the catalog.
- Tool results: `out = a.Ctx.CompactOutput(tc.Function.Name, out)` (nil-safe) before dedup-store + append. The dedup cache stores the *pointer* — a repeated call replays `[already executed] [artifact: …]`, which is exactly what the model saw the first time.
- `Run` start also calls `a.Ctx.Cleanup(24 * time.Hour)` (session-start sweep, mino app.go analog).
- `delegate.go`: child Agent gets `Ctx: a.Ctx.Child()` (nil-safe). Child artifacts are isolated in their own run dir; the child's final reply returns to the parent as a tool result and is compacted if >8K.

## 6. Tool cap changes (`internal/tools`)

- `shell.go`: `outputCap` 64 KiB → 8 MiB. Comment: memory ceiling, not a content limit — full output flows to the artifact layer.
- `read.go`: 1 MiB cap stays; fix the stale "Plan 4 replaces the cap" comment to state the ceiling stays (read_file is never compacted, D31).

## 7. Test strategy

Port mino's `context_test.go` techniques (D31: "mino's context_test.go = Fender's test suite blueprint"):

| Test | Technique |
|------|-----------|
| CompactOutput > 8K → pointer + file content matches + 0600 perms | compression round-trip |
| CompactOutput ≤ 8K inline; read_file passthrough at any size | 8K rule + select |
| CompactInput large → HEAD + TAIL present, middle absent, file written, artifact recorded | head/tail |
| `For` budget bound: system + all messages ≤ ContextChars | budget arithmetic (mino `TestContextForBoundsCurrentInputAndKeepsArtifactCatalog`) |
| `For` truncation: last N turns kept, marker `[K earlier turns compacted]`, older dropped | compaction markers (mino `TestContextMessagesKeepsLastNTurnsOnly`) |
| `For` catalog rides: artifact path present in returned messages | artifact catalog |
| Cleanup: stale removed, fresh kept | isolate sweep |
| Agent loop end-to-end: fake LLM + Manager — >8K shell output appears as pointer in history, read_file inline, dedup replay shows pointer | integration |
| Delegate child: child artifacts land in child root, parent catalog unaffected | isolation |

## 8. Acceptance criteria

1. `go test ./...` green, `go vet ./...` clean, single binary builds.
2. No tool result > 8K (except read_file) ever sits inline in the message history.
3. `For` output always within `ContextChars` (test-enforced).
4. Existing ticket-03 agent/tools tests pass unchanged (nil-safe Ctx).
5. `CHANGELOG.md` updated on every commit (hook-enforced).
6. Wayfinder ticket 04 resolved; frontier → 05 (Memory/ICM layers).

## 9. Deferred (with seams)

| Item | Decision | Seam |
|------|----------|------|
| Artifact persistence beyond sweep window | D9 | pointer format carries full path; `.fender/working/` migration is path-only |
| Consolidation via small model | D32 layer 6 | memory module (ticket 05) |
| Prompt caching (stable prefix) | D34 layer 4 | system-first ordering already satisfied |
