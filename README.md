# Fender

A from-scratch Go coding agent — one loop, subagents as a tool, guardrailed shell, native skills, and code intelligence built in. Terminal REPL, autonomous mode, and a localhost web dashboard — all in a single binary.

> Design DNA: **the harness is the guardrail.** The LLM gets full freedom; safety lives in deterministic code.

## Quickstart

```bash
go install github.com/H4fizWasabie/fender/cmd/fender@latest   # or: go build ./cmd/fender
fender init                # scaffolds .fender/ workspace + fender.toml
# edit fender.toml — add your provider key
fender                     # interactive REPL
fender run "fix this bug"  # autonomous mode
fender dashboard           # web GUI at http://127.0.0.1:8787
```

### Config (`fender.toml`, or `~/.fender/fender.toml`)

```toml
mode = "balanced" # strict | balanced | yolo

[providers.zen]
base_url = "https://opencode.ai/zen"
api_key = "sk-..."
models = ["deepseek-v4-flash-free"]
default_model = "deepseek-v4-flash-free"

[providers.zen.model_configs.deepseek-v4-flash-free]
thinking = true
thinking_levels = { low = "low", medium = "medium", high = "high" }
```

Any OpenAI-compatible endpoint works (OpenRouter, Ollama, LM Studio, vLLM). Anthropic-native via `api = "anthropic"`.

## What it does

- **One loop, everything is an agent** (D13) — parent and subagents are the same type; subagent spawning is just a tool; subagents can pick different providers
- **Adaptive OODA** (D36) — flat loop by default, one explicit orientation turn on thrash detection
- **Guardrail in code, not prompts** (D21–24) — `mvdan.cc/sh` AST verdicts (RUN/ASK/REFUSE), strict/balanced/yolo modes, REFUSE is hard in all modes, command timeouts, audit log
- **Context = artifact engineering** (D31) — >8K output becomes pointers, HEAD/TAIL input preview, read_file never compacted, session isolation, 24h sweep
- **23 bundled skills** (D27–30) — 17 engineering skills (mattpocock/skills) + 6 ponytail, embedded in the binary; ponytail is the always-loaded discipline; `fender skill install <repo|path>` for more
- **ICM memory** (D14) — PROJECT.md always loaded, MAP.md navigation, AGENTS.md/CLAUDE.md/CONTEXT.md detected and loaded directly (nested AGENTS.md per directory), working notes with pruning
- **Code intelligence** (D16–20) — tree-sitter extraction (Go/Python/TS/JS) → symbol graph with EXTRACTED/INFERRED confidence → callers/callees/search; incremental index refreshed every session; `fender intel refresh|search|map`
- **Thinking mode** (D40) — pi-style levels (`/thinking off|low|medium|high`), reasoning streamed dimmed
- **Session persistence** (D41) — resume, `--new`, `fender sessions`; background consolidation distills sessions into facts + episodes (D43)

## Commands

```
fender                    REPL (resumes last session; --new for fresh)
fender run TASK           autonomous one-shot
fender dashboard          web GUI (localhost:8787)
fender init               scaffold workspace + config
fender sessions           list saved sessions
fender providers          list configured providers
fender intel refresh|search|map
fender skill install SRC  local path or git URL
```

REPL slash commands: `/quit` `/model <provider>` `/mode <strict|balanced|yolo>` `/thinking <off|low|medium|high>` `/skills` `/help`

## Design docs

- `DECISIONS.md` — the full decision log (D1–D45)
- `docs/superpowers/specs/` — one spec per subsystem
- `docs/superpowers/plans/` — implementation plans
- `.scratch/fender/map.md` — the wayfinder map (all tickets + backlog)

## Reference lineage

Fender's code-intel core is a Go reimplementation of the ideas in [graphify](https://github.com/Graphify-Labs/graphify), [codegraph](https://github.com/colbymchenry/codegraph), and [code-context-engine](https://github.com/elara-labs/code-context-engine) (all MIT). Skills are vendored from [mattpocock/skills](https://github.com/mattpocock/skills) and [ponytail](https://github.com/DietrichGebert/ponytail) (MIT).

## License

MIT — see [LICENSE](LICENSE).
