package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/codeintel"
	"github.com/H4fizWasabie/fender/internal/guardrail"
	"github.com/H4fizWasabie/fender/internal/memory"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/skills"
	"github.com/H4fizWasabie/fender/internal/tools"
)

const defaultSystem = `You are Fender, a coding agent working inside a repository. Work autonomously within your tools.

TOOL USE
- Prefer tools over guessing: read files before editing, run tests to verify, search before assuming.
- Use read_file with offset and limit to fetch only the slices you need.
- Use edit_file with a unique old_text; if it fails, read the file first.
- Use the shell tool for commands; keep them simple and purposeful.
- Delegate self-contained subtasks (research, investigation) to a child agent via delegate; the child's final text is its answer.

OUTPUT STYLE
- Be concise. Report meaningful milestones, decisions, failures, and when you need input — not a play-by-play.
- The user can see your tool activity; prose should add information, not commentary.

AMBIGUITY
- If you need information from the user, ask in prose. The turn ends with your question; they will answer.

COMPLETION
- When the work is done, stop. Your final text is the answer. Do not narrate a summary of steps already visible; state the outcome.`

// buildAgent wires every subsystem from fender.toml (ticket-08 spec §5).
// modeOverride nil → the config's mode; approver nil → ASK is denied.
func buildAgent(cfgPath string, modeOverride *guardrail.Mode, approver func(context.Context, string, string) (bool, error)) (*agent.Agent, error) {
	reg, err := provider.LoadSelected(cfgPath)
	if err != nil {
		return nil, err
	}
	primary, ok := reg.Default()
	if !ok {
		return nil, fmt.Errorf("no provider with default_model set (see fender.toml)")
	}
	llm, err := reg.WithFallback(primary)
	if err != nil {
		return nil, err
	}

	mode := configuredMode(cfgPath)
	if modeOverride != nil {
		mode = *modeOverride
	}
	// D54/D56: loop cap + context window from the canonical config
	var maxIterations, ctxWindow, reserveTokens int
	if cfg, err := provider.LoadConfig(cfgPath); err == nil {
		maxIterations = cfg.MaxIterations
		ctxWindow = cfg.ContextWindow
		reserveTokens = cfg.ReserveTokens
	}

	home, _ := os.UserHomeDir()
	os.MkdirAll(filepath.Join(home, ".fender"), 0700)
	auditF, err := os.OpenFile(filepath.Join(home, ".fender", "audit.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	audit := guardrail.NewAudit(auditF)

	var searcher tools.Searcher
	if _, err := os.Stat(filepath.Join(".fender", "codeintel", "graph.json")); err == nil {
		if store, err := codeintel.Open("."); err == nil {
			searcher = store.Searcher()
		}
	}
	if searcher == nil {
		searcher = tools.DefaultSearcher(".")
	}

	mem := memory.New(".")
	regTools := tools.New(".", tools.ShellConfig{
		Mode:       mode,
		ProjectDir: ".",
		Audit:      audit,
		Approver:   approver,
	}, searcher, mem.NestedRules) // D46: nested AGENTS.md on read/edit

	base, err := skills.Bundled()
	if err != nil {
		return nil, err
	}
	userSkills, _ := skills.Load(filepath.Join(home, ".fender", "skills"))
	projSkills, _ := skills.Load(filepath.Join(".fender", "skills"))
	regSkills := base.Merge(projSkills, userSkills)

	a := agent.NewAgent(llm, regTools)
	a.System = defaultSystem
	a.Mem = mem
	a.Skills = regSkills
	a.MaxIter = maxIterations // D54: configurable loop cap (0 = 30)
	a.Meter = &provider.Meter{Window: ctxWindow, Reserve: reserveTokens} // D56
	return a, nil
}

// configuredMode reports the mode from the provider package's canonical
// config selection, keeping the dashboard and tool guardrail aligned.
func configuredMode(cfgPath string) guardrail.Mode {
	cfg, err := provider.LoadConfig(cfgPath)
	if err == nil {
		if mode, parseErr := guardrail.ParseMode(cfg.Mode); parseErr == nil {
			return mode
		}
	}
	return guardrail.Balanced
}

// intelRefreshTool lets the agent refresh the code index when it senses
// drift (codeintel spec decision 8, delivered D45).
func intelRefreshTool(store *codeintel.Store) tools.Tool {
	return tools.Tool{
		Name:        "intel_refresh",
		Description: "Refresh the code-intelligence index (re-extract changed files, rebuild the symbol graph). Call when you suspect the index is stale or before searching for symbols.",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			n, err := store.Refresh()
			if err != nil {
				return "", err
			}
			if _, err := store.Rebuild(); err != nil {
				return "", err
			}
			return fmt.Sprintf("index refreshed (%d file(s) re-extracted)", n), nil
		},
	}
}
