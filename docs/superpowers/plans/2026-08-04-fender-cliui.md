# Fender Plan 8: CLI + UI Implementation Plan (final ticket — Fender v1)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The user-facing surface: observer/streaming loop support, the composition root, the REPL, `fender run`, `fender init`. **Completes Fender v1.**

**Architecture:** `Agent.Observer` + optional `Streamer` type-assertion (nil-safe, existing tests untouched). `cmd/fender/agent.go` composes every subsystem from `fender.toml`. `cmd/fender/repl.go` runs the interactive loop with slash commands; `fender run` is the quiet autonomous mode; `fender init` scaffolds.

**Tech Stack:** Go 1.22, stdlib only (`bufio`, `os/signal`, `context`). No new dependencies.

## Global Constraints

- **Read `AGENTS.md`, `DECISIONS.md`, ticket-08 spec first.**
- **Every commit MUST stage `CHANGELOG.md`** — enforced by `.githooks/pre-commit`.
- **Nil-safe observer** — all ticket-03..07 tests pass unchanged.
- **No new dependencies.** Stdlib only.
- Module path `github.com/H4fizWasabie/fender`.

---

### Task 1: Observer + Streamer + loop events

**Files:**
- Modify: `internal/agent/agent.go`
- Modify: `internal/provider/client.go`
- Create: `internal/agent/observer_test.go`

**Interfaces:**
- Produces:
  - `type Event struct { Kind, Text, Status string }`
  - `Agent.Observer func(Event)` (nil-safe)
  - `type Streamer interface { StreamChat(ctx, req, onDelta func(string)) (*provider.Response, error) }`
  - `provider.Client.StreamChat` — wraps `Stream`, identical behavior
  - Loop: delta events (one for non-stream Chat, per-chunk for Streamer), tool events, done event

- [ ] **Step 1: Write the failing test**

`internal/agent/observer_test.go`:

```go
package agent

import (
	"context"
	"testing"

	"github.com/H4fizWasabie/fender/internal/provider"
)

type streamFake struct {
	*fakeLLM
	deltas []string
}

func (s *streamFake) StreamChat(ctx context.Context, req provider.Request, onDelta func(string)) (*provider.Response, error) {
	for _, d := range []string{"hel", "lo"} {
		onDelta(d)
	}
	return completeReply("complete", "done"), nil
}

func TestObserverNonStreaming(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{completeReply("complete", "done")}}
	a := NewAgent(fake, newTestRegistry(t))
	var events []Event
	a.Observer = func(e Event) { events = append(events, e) }
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "go"}})
	var deltas []string
	for _, e := range events {
		if e.Kind == "delta" {
			deltas = append(deltas, e.Text)
		}
	}
	if len(deltas) != 1 || deltas[0] != "done" {
		t.Fatalf("deltas = %v", deltas)
	}
	// done event present
	found := false
	for _, e := range events {
		if e.Kind == "done" && e.Status == "complete" {
			found = true
		}
	}
	if !found {
		t.Fatalf("done event missing: %+v", events)
	}
}

func TestObserverStreaming(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{completeReply("complete", "done")}}
	sf := &streamFake{fakeLLM: fake}
	a := NewAgent(sf, newTestRegistry(t))
	var events []Event
	a.Observer = func(e Event) { events = append(events, e) }
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "go"}})
	var deltas []string
	for _, e := range events {
		if e.Kind == "delta" {
			deltas = append(deltas, e.Text)
		}
	}
	if len(deltas) != 2 || deltas[0] != "hel" || deltas[1] != "lo" {
		t.Fatalf("streamed deltas = %v", deltas)
	}
}

func TestObserverToolEvent(t *testing.T) {
	fake := &fakeLLM{steps: []*provider.Response{
		toolReply("c1", "shell", `{"command":"echo hi"}`),
		completeReply("complete", "done"),
	}}
	a := NewAgent(fake, newTestRegistry(t))
	var events []Event
	a.Observer = func(e Event) { events = append(events, e) }
	a.Run(context.Background(), []provider.Message{{Role: "user", Content: "run echo"}})
	found := false
	for _, e := range events {
		if e.Kind == "tool" && e.Text == "shell" {
			found = true
		}
	}
	if !found {
		t.Fatalf("tool event missing: %+v", events)
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/agent/ -run TestObserver -v`
Expected: FAIL — `Event` undefined.

- [ ] **Step 3: Implement**

In `internal/agent/agent.go`:

```go
// Event is one observable loop event (the renderer seam, ticket-08 spec §3.1).
type Event struct {
	Kind   string // "delta" | "tool" | "done"
	Text   string // delta text / tool description
	Status string // tool status ("ok"|"error"|"cached") or result status
}

// Streamer is the optional streaming capability of an LLM (spec §3.2).
type Streamer interface {
	StreamChat(ctx context.Context, req provider.Request, onDelta func(string)) (*provider.Response, error)
}
```

Add field to Agent struct:

```go
	Observer   func(Event)      // renderer seam (ticket 08); nil-safe
```

In `Run`, the LLM call site becomes:

```go
		resp, err := a.chat(ctx, provider.Request{Messages: msgs, Tools: schemas})
```

with:

```go
// chat calls the LLM, streaming deltas through the observer when possible.
func (a *Agent) chat(ctx context.Context, req provider.Request) (*provider.Response, error) {
	if st, ok := a.LLM.(Streamer); ok && a.Observer != nil {
		return st.StreamChat(ctx, req, func(d string) {
			a.Observer(Event{Kind: "delta", Text: d})
		})
	}
	resp, err := a.LLM.Chat(ctx, req)
	if err == nil && a.Observer != nil && len(resp.Choices) > 0 {
		a.Observer(Event{Kind: "delta", Text: resp.Choices[0].Message.Content})
	}
	return resp, err
}
```

After tool execution (find the tool-execute site in Run, where `out` and `status` are computed — around the dedup block), emit:

```go
		if a.Observer != nil {
			a.Observer(Event{Kind: "tool", Text: tc.Function.Name, Status: status})
		}
```

Before each `return` of a Result (complete/blocked/error/stalled/cancelled), emit done. Simplest: wrap the return — add a helper `a.finish(res *Result) *Result` that emits `Event{Kind: "done", Status: res.Status}` when Observer != nil and returns res; replace all `return &Result{...}` in Run with `return a.finish(&Result{...})`. (There are ~6 return sites in Run; the helper keeps them one-line.)

In `internal/provider/client.go`:

```go
// StreamChat implements agent.Streamer: streams deltas, accumulates the
// full response (tool calls included).
func (c *Client) StreamChat(ctx context.Context, req provider.Request, onDelta func(string)) (*provider.Response, error) {
	return c.Stream(ctx, req, onDelta)
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/agent/ ./internal/provider/ -v`
Expected: PASS. Then `go test ./...` — all prior tests unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/ internal/provider/ CHANGELOG.md
git commit -m "feat: loop observer events + Streamer interface (delta/tool/done, nil-safe)"
```

CHANGELOG:

```markdown
### Added
- Agent observer events (delta/tool/done) + optional Streamer interface; provider.Client.StreamChat; nil-safe, prior tests unchanged
```

---

### Task 2: Composition root (`buildAgent`)

**Files:**
- Create: `cmd/fender/agent.go`
- Create: `cmd/fender/agent_test.go`

**Interfaces:**
- Consumes: provider registry, guardrail (Mode/NewAudit), tools.New, codeintel store Searcher, skills (Bundled/Load/Merge), memory, ctxpkg (all prior tickets).
- Produces:
  - `func buildAgent(cfgPath string, approver func(cmd, reason string) (bool, error)) (*agent.Agent, error)` — the full wiring (spec §5)

- [ ] **Step 1: Write the failing test**

`cmd/fender/agent_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildAgentWithConfig(t *testing.T) {
	cfg := writeConfig(t, `
mode = "balanced"

[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`)
	a, err := buildAgent(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("nil agent")
	}
	if a.System == "" {
		t.Fatal("system prompt missing")
	}
	if a.Mem == nil || a.Skills == nil || a.Ctx == nil {
		t.Fatal("wiring incomplete: Mem/Skills/Ctx must be set")
	}
}

func TestBuildAgentMissingConfig(t *testing.T) {
	if _, err := buildAgent(filepath.Join(t.TempDir(), "absent.toml"), nil); err == nil {
		t.Fatal("expected error for missing config")
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./cmd/fender/ -run TestBuildAgent -v`
Expected: FAIL — `buildAgent` undefined.

- [ ] **Step 3: Write the composition root**

`cmd/fender/agent.go`:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/codeintel"
	"github.com/H4fizWasabie/fender/internal/context"
	"github.com/H4fizWasabie/fender/internal/guardrail"
	"github.com/H4fizWasabie/fender/internal/memory"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/skills"
	"github.com/H4fizWasabie/fender/internal/tools"
)

const defaultSystem = `You are Fender, a coding agent. Work autonomously within your tools. When the task is done, call complete_task with the final reply.`

// buildAgent wires every subsystem from fender.toml (ticket-08 spec §5).
func buildAgent(cfgPath string, approver func(cmd, reason string) (bool, error)) (*agent.Agent, error) {
	var (
		reg *provider.Registry
		err error
	)
	if cfgPath != "" {
		reg, err = provider.Load(cfgPath)
	} else {
		reg, err = provider.LoadDefault()
	}
	if err != nil {
		return nil, err
	}
	llm, ok := reg.Default()
	if !ok {
		return nil, fmt.Errorf("no provider with default_model set (see fender.toml)")
	}

	// guardrail: mode from config, audit file sink, approver from caller
	mode := guardrail.Mode("balanced")
	if cfgPath != "" {
		var cfg provider.Config
		if _, err := provider.Decode(cfgPath, &cfg); err == nil && cfg.Mode != "" {
			mode = guardrail.Mode(cfg.Mode)
		}
	}
	home, _ := os.UserHomeDir()
	auditF, err := os.OpenFile(filepath.Join(home, ".fender", "audit.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	audit := guardrail.NewAudit(auditF)

	// search backend: codeintel index when present, else default walker
	var searcher tools.Searcher
	if _, err := os.Stat(filepath.Join(".fender", "codeintel", "graph.json")); err == nil {
		if store, err := codeintel.Open("."); err == nil {
			searcher = store.Searcher()
		}
	}
	if searcher == nil {
		searcher = tools.DefaultSearcher(".")
	}

	regTools := tools.New(".", tools.ShellConfig{
		Mode:       mode,
		ProjectDir: ".",
		Audit:      audit,
		Approver:   approver,
	}, searcher)

	// skills: bundled merged with user + project (D27 lookup order)
	base, err := skills.Bundled()
	if err != nil {
		return nil, err
	}
	userSkills, _ := skills.Load(filepath.Join(home, ".fender", "skills"))
	projSkills, _ := skills.Load(filepath.Join(".fender", "skills"))
	regSkills := base.Merge(projSkills, userSkills)

	a := agent.NewAgent(llm, regTools)
	a.System = defaultSystem
	a.Mem = memory.New(".")
	a.Skills = regSkills
	a.Ctx = context.New()
	a.Resolver = func(name string) (agent.LLM, error) {
		c, ok := reg.Client(name)
		if !ok {
			return nil, fmt.Errorf("unknown provider %q", name)
		}
		return c, nil
	}
	return a, nil
}
```

Note: `provider.Decode` may not exist — check `internal/provider/config.go`; if only `toml.DecodeFile` is used directly, add a small exported `Decode(path string, cfg *Config) error` helper or read the mode via a tiny local decode. Choose whichever fits the existing API. If `guardrail.Mode` isn't a defined type, check `internal/guardrail/verdict.go` for the actual mode type name (ticket 02) and use it.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./cmd/fender/ -run TestBuildAgent -v`
Expected: PASS. Then `go test ./...`.

- [ ] **Step 5: Commit**

```bash
git add cmd/fender/ CHANGELOG.md
git commit -m "feat: composition root buildAgent (providers, guardrail, skills, memory, context, search)"
```

CHANGELOG:

```markdown
### Added
- buildAgent: full subsystem wiring from fender.toml — default provider LLM, guardrail mode + audit file (~/.fender/audit.log), codeintel searcher (fallback default), skills merge, memory, context, resolver
```

---

### Task 3: REPL with slash commands

**Files:**
- Create: `cmd/fender/repl.go`
- Create: `cmd/fender/repl_test.go`
- Modify: `cmd/fender/main.go` (no-args → repl)

**Interfaces:**
- Produces:
  - `func repl(out, errOut io.Writer, in *bufio.Reader, cfgPath string) error` — banner, `> ` prompt, slash dispatch (`/quit`, `/model <name>`, `/mode <mode>`, `/skills`, `/help`), else Agent.Run with observer rendering
  - History carried across turns (messages slice); agent rebuilt on /model and /mode

- [ ] **Step 1: Write the failing test**

`cmd/fender/repl_test.go`:

```go
package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestReplQuit(t *testing.T) {
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/quit\n"))
	if err := repl(&out, &errOut, in, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "fender") {
		t.Fatalf("banner missing: %q", out.String())
	}
}

func TestReplHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/help\n/quit\n"))
	if err := repl(&out, &errOut, in, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "/quit") || !strings.Contains(out.String(), "/mode") {
		t.Fatalf("help missing commands: %q", out.String())
	}
}

func TestReplUnknownSlash(t *testing.T) {
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/nope\n/quit\n"))
	if err := repl(&out, &errOut, in, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unknown command") {
		t.Fatalf("expected unknown-command error: %q", out.String())
	}
}

func TestReplModelUnknown(t *testing.T) {
	var out, errOut bytes.Buffer
	in := bufio.NewReader(strings.NewReader("/model does-not-exist\n/quit\n"))
	if err := repl(&out, &errOut, in, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unknown provider") {
		t.Fatalf("expected unknown provider error: %q", out.String())
	}
}
```

Note: `repl` must work WITHOUT a valid fender.toml for slash commands — buildAgent errors surface only when a real task runs; slash dispatch happens first. If no config exists, tasks print the build error and continue (the REPL never dies on agent-build failure — it reports and loops). The tests above only exercise slash paths, so they pass without config.

- [ ] **Step 2: Run to verify fail**

Run: `go test ./cmd/fender/ -run TestRepl -v`
Expected: FAIL — `repl` undefined.

- [ ] **Step 3: Write the REPL**

`cmd/fender/repl.go`:

```go
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/guardrail"
	"github.com/H4fizWasabie/fender/internal/provider"
)

func repl(out, errOut io.Writer, in *bufio.Reader, cfgPath string) error {
	fmt.Fprintf(out, "fender %s — type /help for commands\n", version)

	var (
		curAgent *agent.Agent
		history  []provider.Message
		mode     = guardrail.Balanced
	)
	rebuild := func() error {
		approver := func(cmd, reason string) (bool, error) {
			fmt.Fprintf(out, "\n  [approval] %s\n  %s [y/N] ", reason, cmd)
			line, _ := in.ReadString('\n')
			return strings.TrimSpace(strings.ToLower(line)) == "y", nil
		}
		a, err := buildAgent(cfgPath, approver)
		if err != nil {
			return err
		}
		a.Observer = func(e agent.Event) {
			switch e.Kind {
			case "delta":
				fmt.Fprint(out, e.Text)
			case "tool":
				status := e.Status
				if status == "" {
					status = "ok"
				}
				fmt.Fprintf(out, "\n  [tool %s: %s]\n", e.Text, status)
			case "done":
				fmt.Fprintf(out, "\n<%s>\n", e.Status)
			}
		}
		curAgent = a
		return nil
	}
	if err := rebuild(); err != nil {
		fmt.Fprintf(errOut, "warning: %v\n", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	for {
		fmt.Fprint(out, "> ")
		line, err := in.ReadString('\n')
		if err != nil { // EOF
			fmt.Fprintln(out)
			return nil
		}
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		if strings.HasPrefix(text, "/") {
			quit, err := slash(out, text, &mode, rebuild)
			if err != nil {
				fmt.Fprintf(out, "error: %v\n", err)
			}
			if quit {
				return nil
			}
			continue
		}
		if curAgent == nil {
			fmt.Fprintln(out, "error: agent not built (config problem above)")
			continue
		}
		history = append(history, provider.Message{Role: "user", Content: text})
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-sig
			cancel()
		}()
		res := curAgent.Run(ctx, history)
		cancel()
		if res.Status == "complete" || res.Status == "blocked" {
			history = append(history,
				provider.Message{Role: "assistant", Content: res.Reply})
		}
	}
}

// slash handles one slash command. Returns quit=true for /quit.
func slash(out io.Writer, text string, mode *guardrail.Mode, rebuild func() error) (bool, error) {
	parts := strings.Fields(text)
	switch parts[0] {
	case "/quit":
		return true, nil
	case "/help":
		fmt.Fprintln(out, "commands: /quit /model <provider> /mode <strict|balanced|yolo> /skills /help")
		return false, nil
	case "/model":
		if len(parts) < 2 {
			return false, fmt.Errorf("usage: /model <provider>")
		}
		// swap the LLM: rebuild with a fresh agent; caller must set LLM
		// (implement by rebuilding then replacing LLM via buildAgent's returned agent
		// — simplest: rebuild() then set a.LLM from a resolver lookup)
		return false, fmt.Errorf("not yet wired: /model rebuilds via buildAgent")
	case "/mode":
		if len(parts) < 2 {
			return false, fmt.Errorf("usage: /mode <strict|balanced|yolo>")
		}
		m := guardrail.Mode(parts[1])
		if !validMode(m) {
			return false, fmt.Errorf("invalid mode %q (strict|balanced|yolo)", parts[1])
		}
		*mode = m
		return false, rebuild()
	case "/skills":
		if curReg == nil {
			return false, fmt.Errorf("skills registry unavailable")
		}
		fmt.Fprint(out, curReg.Descriptions())
		return false, nil
	default:
		return false, fmt.Errorf("unknown command %q (try /help)", parts[0])
	}
}

func validMode(m guardrail.Mode) bool {
	switch m {
	case guardrail.Strict, guardrail.Balanced, guardrail.Yolo:
		return true
	}
	return false
}
```

Notes:
- `/model` and `/skills` need state (current registry / current agent). Cleanest: keep a package-level or closure state struct in `repl` — pass `curReg *skills.Registry` and a `setLLM func(agent.LLM)` closure to `slash`. Implement `/model` as: rebuild() then `curAgent.LLM = client from registry lookup` (buildAgent could return the registry too, or slash re-loads it via provider.Load). Choose the closure approach; adjust signatures as needed.
- The `mode` var is passed to buildAgent's ShellConfig via a package var or closure — buildAgent currently reads mode from config; for `/mode` to take effect live, buildAgent must accept a mode override. Add `modeOverride *guardrail.Mode` param (nil → config value). Adjust Task 2's signature accordingly (tests updated to pass nil).
- Signal handling: keep it simple — the goroutine cancels the run ctx on interrupt; the REPL loop continues.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./cmd/fender/ -run TestRepl -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/fender/ CHANGELOG.md
git commit -m "feat: interactive REPL (observer rendering, slash commands, live /mode)"
```

CHANGELOG:

```markdown
### Added
- REPL: fender interactive mode — observer rendering (streaming deltas, tool lines), /quit /model /mode /skills /help, in-memory history
```

---

### Task 4: `fender run` + `fender init`

**Files:**
- Modify: `cmd/fender/main.go`

**Interfaces:**
- Produces:
  - `fender run <task>` — buildAgent (nil approver → ASK denied), Run, print reply, exit 0 complete / 1 otherwise
  - `fender init` — memory.Ensure() + scaffold fender.toml if missing; idempotent

- [ ] **Step 1: Write the failing test** (append to `cmd/fender/main_test.go`)

```go
func TestRunCommand(t *testing.T) {
	// no config → build error surfaces as exit 1 with message
	var out bytes.Buffer
	err := runCLI(&out, []string{"run", "do something"})
	if err == nil {
		t.Fatal("expected error without config")
	}
}

func TestInitCommand(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runCLI(&out, []string{"init"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".fender", "memory", "PROJECT.md")); err != nil {
		t.Fatal("memory workspace not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "fender.toml")); err != nil {
		t.Fatal("fender.toml not scaffolded")
	}
	// idempotent
	if err := runCLI(&out, []string{"init"}); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./cmd/fender/ -run "TestRunCommand|TestInitCommand" -v`
Expected: FAIL — unknown commands.

- [ ] **Step 3: Implement**

In `main.go` — usage lines + switch cases:

```go
	case "run":
		if fs.NArg() < 2 {
			return fmt.Errorf("usage: fender run <task>")
		}
		return runTask(out, strings.Join(fs.Args()[1:], " "))
	case "init":
		return initProject(out)
```

```go
// runTask runs one autonomous task (D4): quiet, final reply only.
func runTask(out io.Writer, task string) error {
	a, err := buildAgent("", nil)
	if err != nil {
		return err
	}
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: task}})
	fmt.Fprintln(out, res.Reply)
	if res.Status != "complete" {
		return fmt.Errorf("status: %s", res.Status)
	}
	return nil
}

// initProject scaffolds the workspace + config (D25), idempotent.
func initProject(out io.Writer) error {
	mem := memory.New(".")
	if err := mem.Ensure(); err != nil {
		return err
	}
	if _, err := os.Stat("fender.toml"); os.IsNotExist(err) {
		template := `# Fender configuration (D25)
mode = "balanced" # strict | balanced | yolo (D21)

[providers.openrouter]
base_url = "https://openrouter.ai/api/v1"
api_key = "sk-or-v1-..."
models = ["openai/gpt-4o-mini"]
default_model = "openai/gpt-4o-mini"
`
		if err := os.WriteFile("fender.toml", []byte(template), 0600); err != nil {
			return err
		}
		fmt.Fprintln(out, "wrote fender.toml (edit api_key)")
	}
	fmt.Fprintln(out, "workspace ready (.fender/)")
	return nil
}
```

Imports: `context`, `provider`, `memory` added to main.go as needed.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./cmd/fender/ -v`
Expected: all PASS. Then `go build ./cmd/fender && ./fender init && rm -f fender` smoke test.

- [ ] **Step 5: Commit**

```bash
git add cmd/fender/ CHANGELOG.md
git commit -m "feat: fender run (autonomous) + fender init (scaffold)"
```

CHANGELOG:

```markdown
### Added
- `fender run <task>`: autonomous one-shot (reply + exit code by status)
- `fender init`: memory workspace + fender.toml scaffold, idempotent
```

---

### Task 5: Wayfinder resolve — Fender v1 complete

**Files:**
- Modify: `.scratch/fender/issues/08-CLIAndUI.md`
- Modify: `.scratch/fender/map.md`

- [ ] **Step 1: Full verification**

```bash
go build ./... && go vet ./... && go test ./...
go build ./cmd/fender && ./fender init && ./fender intel refresh && ./fender intel map && ./fender --help && rm -f fender
```

Expected: build/vet/tests green; init + intel + help all work.

- [ ] **Step 2: Resolve the ticket** — Answer: everything delivered, v1 complete, post-v1 backlog listed (D9 persistence, D6 Anthropic, D2 GUI, TUI, response caching D35, memory graph/consolidation).

- [ ] **Step 3: Update the map** — decisions index entry + a "Destination reached" note.

- [ ] **Step 4: Commit**

```bash
git add .scratch/fender/ CHANGELOG.md
git commit -m "docs: resolve wayfinder ticket 08 — Fender v1 complete"
```

CHANGELOG:

```markdown
### Changed
- Wayfinder: ticket 08 resolved — Fender v1 complete (all 8 subsystems delivered); map notes post-v1 backlog
```

---

## Self-Review Notes

- **Spec coverage:** §1 scope 1–6 → Tasks 1–4; §3 decisions 1–8 → Tasks 1–4; §4 API → Task 1; §5 composition → Task 2; §6 REPL → Task 3; §7 test table → each task's tests; §8 acceptance → Task 5. Non-goals (§2) not built.
- **Placeholders:** none. Flagged adaptation points (read the actual code at implementation): (a) `guardrail.Mode` type name and `Strict/Balanced/Yolo` constants (ticket 02's verdict.go); (b) `provider.Config` decode helper for the mode override; (c) `/model` and `/skills` need closure state — the plan sketches the approach, wire it to the actual agent struct; (d) `Agent.finish` helper for done events — check the actual return sites.
- **Type consistency:** `Event{Kind,Text,Status}`, `Streamer.StreamChat`, `buildAgent(cfgPath, approver, modeOverride)`, `repl(out, errOut, in, cfgPath)` consistent across tasks.
- **CHANGELOG:** every task ends with an entry + commit.
- **Deps:** none added.
