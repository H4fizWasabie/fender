# Fender — Design Spec

**Date:** 2026-08-04; reconciled 2026-08-05 through D51
**Status:** Implemented through D51.

Fender is an open-source coding agent harness built from scratch in Go. It ships a terminal CLI and localhost browser dashboard in one binary. It exists for control + learning; the engineering methodology is Matt Pocock's skills + ponytail, native from day one.

---

## 1. Philosophy

- **The harness is the guardrail.** The LLM gets full freedom; safety lives in deterministic code (shell parsing, verdicts, timeouts), never in prompt instructions or the model's mood.
- **Full trust by default, configurable caution.** Default shipped mode is *balanced*; the author runs *yolo*.
- **One persistent actor.** The main agent owns the session and outcome. Delegation creates a synchronous, ephemeral child from the same `Agent` type; provider fallback is resilience, not another agent.
- **Prevention over compression.** Context is layered and loaded selectively (ICM); nothing enters context that isn't needed.

## 2. Non-goals — seams reserved, not built

- Planner, reflection phase, swarm, agent graph, or parallel tool dispatcher (D13, D50)
- TUI (D26) — the terminal remains a plain streaming transcript
- Response caching (D35, D44)

## 3. Architecture

```
fender/
├── cmd/fender/           # fender (interactive) · fender run "task" (autonomous) · fender init · fender skill install
├── internal/
│   ├── agent/            # D13/D50: ONE main loop; synchronous ephemeral child via delegate
│   ├── provider/         # shared provider shape + backup-key fallback + TOML registry
│   ├── tools/            # D10: read, edit, shell, search — seam for graphify/cce/codegraph as search backends
│   ├── guardrail/        # D21-24: strict/balanced/yolo modes; sh-parser verdicts (RUN/ASK/REFUSE); timeouts; audit log
│   ├── skills/           # D27-30: registry + trigger matching + loader (go:embed 23 skills; install external)
│   ├── memory/           # D14/17/28: ICM layers — PROJECT.md always; MAP.md/reference/skills loaded selectively
│   ├── codeintel/        # D16/19/20: tree-sitter extraction → graph build → cluster → query API
│   ├── context/          # D31/D38: artifact engineering and isolated child contexts
│   └── ui/               # terminal renderer + embedded localhost dashboard
├── .fender/              # per-project memory workspace: PROJECT.md, MAP.md, reference/, working/, skills/
└── fender.toml           # providers + permission mode + tool settings
```

### 3.1 Agent loop and child lifecycle (D13, D50)

One `Agent` type runs the canonical loop (prompt → LLM → tool call → execute → result → repeat). Exactly one persistent main agent owns the session and final outcome. `delegate` is a synchronous tool call that creates one ephemeral child running the same loop on a self-contained task. The child receives fresh messages, artifact context, and memory handle; bootstraps from the same canonical project memory; uses the same provider fallback chain and guardrailed tool registry minus `delegate`; returns one result; and is discarded without a session or consolidation. It cannot create grandchildren.

Tool calls execute sequentially in model order. There is no general parallel dispatcher. The removed `ask` one-shot model call is not a second form of child.

### 3.2 Provider layer and fallback (D6, D25, D42, D50)

The agent consumes one OpenAI-shaped internal request/response surface. OpenAI-compatible endpoints cover OpenRouter, Ollama, LM Studio, and vLLM; the shipped Anthropic adapter translates Messages API traffic to the same shape.

Each provider entry in `fender.toml` owns its key, base URL, models, and default model. Optional top-level `fallback = "provider-name"` references a second provider entry, normally the same service/model with a backup API key. A failed non-streaming model request retries once against that entry. A streaming request retries only if the primary failed before emitting text or thinking; cancellations and partial streams never retry. The main and child agents use the same chain. Fallback never creates an agent or changes delegation identity.

### 3.3 Tools (D10)

v1: read file, edit file, shell, codebase search. Search keeps a backend seam — graphify/cce/codegraph plug in later as adapters (in v1 they're internal modules per D16).

### 3.4 Guardrail (D11, D12, D21–24)

- **Permission modes** (per-user config): `strict` (ASK for every tool call) · `balanced` (verdict model) · `yolo` (no prompts). Shipped default: balanced. Author default: yolo.
- **Verdict model**, decided by deterministic code: `RUN` / `ASK` / `REFUSE`.
- **Parsing substrate:** `mvdan.cc/sh/v3` shell parser (AST, not regex).
- **Judged categories:** destructive filesystem (severity by target), privilege/system, irreversible git, pipe-to-shell, runaway/resource, TTY hangers, protected paths (secrets = ASK in balanced), path escape outside project dir.
- **REFUSE is hard in all modes** — fork bombs, mkfs, shutdown, curl-pipe-to-shell never run, even in yolo.
- **Autonomous mode + risky command:** run stops, partial results handed back, asks.
- **Harness-level:** timeout on every command (~60s default, configurable) + full audit log (command, verdict, timestamp).

### 3.5 Skills (D27–30)

- **Format:** standard `SKILL.md` (YAML frontmatter: name, description + markdown body) + optional support files. Compatible with pi / Claude Code skill repos.
- **Bundled (go:embed, all MIT):** 17 engineering skills from mattpocock/skills + all 6 ponytail skills. ~1,800 lines total.
- **Invocation:** model-invoked (descriptions ride in system prompt; body loads on trigger match) + user-invoked (slash commands: `/tdd`, `/code-review`, `/to-tickets`, `/wayfinder`...).
- **ponytail is special:** always-loaded Layer 0 behavioral discipline — governs all reasoning, not trigger-based.
- **Open system:** `fender skill install <repo|path>`. Lookup order: project `.fender/skills/` → user `~/.fender/skills/` → bundled.
- **Skills never bypass the guardrail.**

### 3.6 Memory (D14, D17, D28)

ICM is the memory architecture — layered files that record what's in the codebase (navigation), not just context strategy:

```
.fender/
  memory/
    PROJECT.md          # Layer 0 — always loaded. What this project is, conventions, build commands. Small.
    MAP.md              # Layer 1 — navigation. "What's where" — generated from code-intel.
    reference/          # Layer 2 — selectively loaded per topic.
    skills/             # Layer 3 — skill bodies, loaded on trigger.
    working/            # Layer 4 — session artifacts, scratch notes.
  sessions/             # persisted session history (D41)
```

Memory lives inside the project (portable, diffable). The harness controls loading order — prevention over compression.

### 3.7 Code intelligence (D16, D19, D20)

Built in Go from day one. Reference sources cloned to `~/Desktop/fender-references/` (code-context-engine, codegraph, graphify — all MIT; study + port, do not vendor).

- **Parser substrate:** tree-sitter C core + Go bindings (`go-tree-sitter`); reuse the grammar definitions codegraph ships. No parser rewriting.
- **Pipeline (graphify's architecture, one Go package per stage):** detect → extract → build_graph → cluster → analyze → report → export.
- **Schema:** nodes/edges with `EXTRACTED | INFERRED | AMBIGUOUS` confidence labels.
- **Query API:** symbol lookup, call paths, "where is this used" — codegraph's query ergonomics + cce's retrieval design.
- **MAP.md generation** — code-intel feeds the memory layer (D17: MAP.md is the front door, code-intel is the card catalog).

### 3.8 Context (D31 — resolved from D15)

Conversation/tool-loop context methodology = **mino's artifact engineering**, five techniques:

1. **Compress** — tool output > 8K chars → one-line artifact pointer, never inline
2. **Preserve head + tail** — large user input → HEAD/TAIL split inline, middle dropped
3. **Select** — model fetches slices via `read_file(offset,limit)` — the one tool never compacted; artifact catalog rides in context
4. **Write elsewhere** — full content at isolated path (0600), fetched on demand
5. **Isolate** — per-session/turn/tool dirs, stale pruning, maxAge sweep

Plus: turns-truncation with compaction markers, `ContextFor` budget arithmetic, completion reminders. `context_test.go` from mino = test blueprint.

**Token-caching stack (5 layers):** provider prompt caching · tool dedup within turn · input preview/artifact pointer · turns truncation + markers · background consolidation via small model.

### 3.9 UI (D2, D26)

The terminal remains a plain streaming transcript with minimal ANSI color, visible tool-call lines, readable approval prompts, and slash commands. It opens a fresh session by default; `--resume <id|latest>` restores an earlier session explicitly.

The embedded localhost browser is the **Centered Docket** workbench (D51), an Operate surface rather than a source editor:

- a compact left index contains New session, explicit Resume, repository, model, and permission context;
- one central docket owns the task composer and chronological user/main-agent record;
- an evidence lane is absent in a fresh session and attaches slips only for real observer events (tool, child, thinking, approval, error, completion);
- tool results plus tool, approval, and completion events persist with the dashboard session; transient text and thinking streams remain live-only;
- completion appears only when the HTTP snapshot marks an explicit terminal runtime status and carries the actual final reply, without inferring changed files, checks, or external proof;
- a dashboard session owns one stable persisted ID across turns; new and resumed sessions rebuild clean agent state;
- persistence failures surface through the HTTP state and request error instead of reporting an unsaved session as resumable;
- strict/balanced shell ASK verdicts become a pending browser hold with explicit approve/deny controls;
- responsive layouts preserve the same hierarchy by turning the session index into a drawer and placing evidence after the docket on narrow screens.

The UI is plain embedded HTML/CSS/JS with no framework. Its durable visual language is warm technical stock, graphite rules, safety orange for action/attention, verification green only for proven completion, clipped sheets, flat matte surfaces, and progressive disclosure.

### 3.10 Config (D25)

`fender.toml`, scaffolded by `fender init`. Providers (per-provider key/base URL/models), optional backup provider/key via `fallback`, permission mode, and tool settings.

## 4. Session walkthrough (validation)

`fender "fix the parser bug"` → PROJECT.md loads → description matching pulls `diagnosing-bugs` → MAP.md → code-intel symbol query → read/edit/test cycle under the guardrail → `/code-review` before commit.

## 5. Test strategy

- Port against upstream fixtures: graphify's `worked/` examples, codegraph's `__tests__/`.
- Guardrail verdict table: each category × mode has a test case.
- Skill loader: trigger matching + precedence tests.
- One runnable check per non-trivial module (assert-based demo or small test).

## 6. Deferred list (with seams)

| Item | Decision | Seam |
|------|----------|------|
| TUI | D26 | renderer interface |
| Response caching | D35/D44 | revisit only with paid-model measurements |
| General parallel execution | D50 | requires a new measured decision |
