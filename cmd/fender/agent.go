package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/codeintel"
	ctxpkg "github.com/H4fizWasabie/fender/internal/context"
	"github.com/H4fizWasabie/fender/internal/guardrail"
	"github.com/H4fizWasabie/fender/internal/memory"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/skills"
	"github.com/H4fizWasabie/fender/internal/tools"
)

const defaultSystem = `You are Fender, a coding agent. Work autonomously within your tools. When the task is done, call complete_task with the final reply.`

// buildAgent wires every subsystem from fender.toml (ticket-08 spec §5).
// modeOverride nil → the config's mode; approver nil → ASK is denied.
func buildAgent(cfgPath string, modeOverride *guardrail.Mode, approver func(context.Context, string, string) (bool, error)) (*agent.Agent, error) {
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
	a.Ctx = ctxpkg.New()
	return a, nil
}

// configuredMode mirrors provider.LoadDefault's path order so the guardrail
// and dashboard report the same effective mode even when --config is omitted.
func configuredMode(cfgPath string) guardrail.Mode {
	paths := []string{cfgPath}
	if cfgPath == "" {
		paths = []string{"fender.toml"}
		if home, err := os.UserHomeDir(); err == nil {
			paths = append(paths, filepath.Join(home, ".fender", "fender.toml"))
		}
	}
	for _, path := range paths {
		if path == "" {
			continue
		}
		var cfg provider.Config
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			continue
		}
		if mode, err := guardrail.ParseMode(cfg.Mode); err == nil {
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
