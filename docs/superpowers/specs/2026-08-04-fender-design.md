# Fender — Design Spec

**Date:** 2026-08-04
**Status:** Approved in discussion session. Implementation not started (decision log: `DECISIONS.md`).

Fender is a custom coding agent harness, built from scratch in Go, terminal CLI first with a desktop GUI later, open-sourced. Built for control + learning; the engineering methodology is Matt Pocock's skills + ponytail, native from day one.

---

## 1. Philosophy

- **The harness is the guardrail.** The LLM gets full freedom; safety lives in deterministic code (shell parsing, verdicts, timeouts), never in prompt instructions or the model's mood.
- **Full trust by default, configurable caution.** Default shipped mode is *balanced*; the author runs *yolo*.
- **One loop, everything is an agent.** Parent and subagents are the same `Agent` type; subagent spawning is just another tool.
- **Prevention over compression.** Context is layered and loaded selectively (ICM); nothing enters context that isn't needed.

## 2. Non-goals (v1) — seams reserved, not built

- Conversation/tool-loop context methodology (D15) — thin interface now
- Session persistence (D9)
- Anthropic-native adapter (D6) — OpenAI-compatible only in v1
- Desktop GUI (D2) — CLI first, UI stays a thin skin
- TUI polish (D26) — plain streaming transcript

## 3. Architecture

```
fender/
├── cmd/fender/           # fender (interactive) · fender run "task" (autonomous) · fender init · fender skill install
├── internal/
│   ├── agent/            # D13: ONE loop — Agent{Context, Model, Tools}; subagent = same type, own goroutine + provider
│   ├── provider/         # D6/D7: OpenAI-compatible client + provider registry from TOML
│   ├── tools/            # D10: read, edit, shell, search — seam for graphify/cce/codegraph as search backends
│   ├── guardrail/        # D21-24: strict/balanced/yolo modes; sh-parser verdicts (RUN/ASK/REFUSE); timeouts; audit log
│   ├── skills/           # D27-30: registry + trigger matching + loader (go:embed 23 skills; install external)
│   ├── memory/           # D14/17/28: ICM layers — PROJECT.md always; MAP.md/reference/skills loaded selectively
│   ├── codeintel/        # D16/19/20: tree-sitter extraction → graph build → cluster → query API
│   ├── context/          # D15: conversation-loop methodology — DEFERRED, thin interface now
│   └── ui/               # D26: plain streaming renderer
├── .fender/              # per-project memory workspace: PROJECT.md, MAP.md, reference/, working/, skills/
└── fender.toml           # providers + permission mode + tool settings
```

### 3.1 Agent loop (D13)

One `Agent` type: `Context + Model + Tools`, running the canonical loop (prompt → LLM → tool call → execute → result → repeat). Subagent-as-a-tool: the harness spawns a child `Agent` in a goroutine with its own provider/model from the config, runs the *same* loop, returns the result as a tool result to the parent. Parallel subagents = N goroutines + join. Guardrails wrap tool execution once — every agent passes through.

### 3.2 Provider layer (D6, D7, D25)

OpenAI-compatible API only (covers OpenRouter, Ollama, LM Studio, vLLM, local servers). Provider registry in `fender.toml` — per-provider key, base URL, model list. Subagents are told which provider/model to use. Interface stays clean for an Anthropic adapter later.

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
    sessions/           # (future) — seam for session persistence (D9).
```

Memory lives inside the project (portable, diffable). The harness controls loading order — prevention over compression.

### 3.7 Code intelligence (D16, D19, D20)

Built in Go from day one. Reference sources cloned to `~/Desktop/fender-references/` (code-context-engine, codegraph, graphify — all MIT; study + port, do not vendor).

- **Parser substrate:** tree-sitter C core + Go bindings (`go-tree-sitter`); reuse the grammar definitions codegraph ships. No parser rewriting.
- **Pipeline (graphify's architecture, one Go package per stage):** detect → extract → build_graph → cluster → analyze → report → export.
- **Schema:** nodes/edges with `EXTRACTED | INFERRED | AMBIGUOUS` confidence labels.
- **Query API:** symbol lookup, call paths, "where is this used" — codegraph's query ergonomics + cce's retrieval design.
- **MAP.md generation** — code-intel feeds the memory layer (D17: MAP.md is the front door, code-intel is the card catalog).

### 3.8 Context (D15)

Conversation/tool-loop context engineering uses a methodology decided later — separated from the ICM memory layer. Thin interface in v1.

### 3.9 UI (D26)

Plain streaming transcript: minimal ANSI color, visible tool-call lines, readable approval prompts, slash commands (`/model`, `/mode`, `/tool`, `/quit`). Renderer stays a thin skin — GUI replaces it later (D2).

### 3.10 Config (D25)

`fender.toml`, scaffolded by `fender init`. Providers (per-provider key/base URL/models), permission mode, tool settings.

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
| Conversation-loop context methodology | D15 | `internal/context` thin interface |
| Session persistence | D9 | `memory/sessions/` |
| Anthropic adapter | D6 | provider interface |
| Desktop GUI | D2 | `internal/ui` thin skin |
| graphify/cce/codegraph as external search backends | D16 (internal from day one) | search tool backend seam |
| TUI | D26 | renderer interface |
