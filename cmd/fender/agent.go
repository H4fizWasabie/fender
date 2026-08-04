package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
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
// modeOverride nil → the config's mode; approver nil → ASK is denied.
func buildAgent(cfgPath string, modeOverride *guardrail.Mode, approver func(cmd, reason string) (bool, error)) (*agent.Agent, error) {
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

	mode := guardrail.Balanced
	if cfgPath != "" {
		var cfg provider.Config
		if _, err := toml.DecodeFile(cfgPath, &cfg); err == nil && cfg.Mode != "" {
			mode = guardrail.Mode(cfg.Mode)
		}
	}
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

	regTools := tools.New(".", tools.ShellConfig{
		Mode:       mode,
		ProjectDir: ".",
		Audit:      audit,
		Approver:   approver,
	}, searcher)

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
