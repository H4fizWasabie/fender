package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/H4fizWasabie/fender/internal/codeintel"
	"github.com/H4fizWasabie/fender/internal/memory"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/skills"
)

const version = "0.1.0"

func main() {
	os.Exit(run(os.Stdout, os.Args[1:]))
}

// run parses args and executes; returns process exit code.
func run(out io.Writer, args []string) int {
	if err := runCLI(out, args); err != nil {
		fmt.Fprintln(out, "error:", err)
		return 1
	}
	return 0
}

func runCLI(out io.Writer, args []string) error {
	fs := flag.NewFlagSet("fender", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to fender.toml (default: ./fender.toml, then ~/.fender/fender.toml)")
	fs.SetOutput(out)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		// interactive REPL (D26)
		return repl(out, out, bufio.NewReader(os.Stdin), *configPath)
	}
	switch fs.Arg(0) {
	case "providers":
		return listProviders(out, *configPath)
	case "run":
		if fs.NArg() < 2 {
			return fmt.Errorf("usage: fender run <task>")
		}
		return runTask(out, strings.Join(fs.Args()[1:], " "))
	case "init":
		return initProject(out)
	case "skill":
		return skillCommand(out, fs.Args()[1:])
	case "intel":
		return intelCommand(out, fs.Args()[1:])
	case "repl":
		return repl(out, out, bufio.NewReader(os.Stdin), *configPath)
	default:
		return fmt.Errorf("unknown command %q", fs.Arg(0))
	}
}

func skillCommand(out io.Writer, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: fender skill install <src>")
	}
	switch args[0] {
	case "install":
		if len(args) < 2 {
			return fmt.Errorf("usage: fender skill install <src>")
		}
		return installSkills(out, args[1])
	default:
		return fmt.Errorf("unknown skill command %q", args[0])
	}
}

func installSkills(out io.Writer, src string) error {
	dest := filepath.Join(".fender", "skills")
	names, err := skills.Install(src, dest)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "installed %d skill(s): %s\n", len(names), strings.Join(names, ", "))
	return nil
}

func intelCommand(out io.Writer, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: fender intel <refresh|search|map>")
	}
	s, err := codeintel.Open(".")
	if err != nil {
		return err
	}
	switch args[0] {
	case "refresh":
		n, err := s.Refresh()
		if err != nil {
			return err
		}
		if _, err := s.Rebuild(); err != nil {
			return err
		}
		fmt.Fprintf(out, "refreshed %d file(s)\n", n)
	case "search":
		if len(args) < 2 {
			return fmt.Errorf("usage: fender intel search <query>")
		}
		searcher := s.Searcher()
		res, err := searcher(strings.Join(args[1:], " "))
		if err != nil {
			return err
		}
		for _, r := range res {
			fmt.Fprintf(out, "%s:%d: %s\n", r.Path, r.Line, r.Text)
		}
	case "map":
		if _, err := s.Refresh(); err != nil {
			return err
		}
		g, err := s.Rebuild()
		if err != nil {
			return err
		}
		body := g.GenerateMap()
		mem := memory.New(".")
		if err := mem.Ensure(); err != nil {
			return err
		}
		mapPath := filepath.Join(".fender", "memory", "MAP.md")
		if err := os.WriteFile(mapPath, []byte(body), 0600); err != nil {
			return err
		}
		fmt.Fprintf(out, "wrote %s\n", mapPath)
	default:
		return fmt.Errorf("unknown intel command %q", args[0])
	}
	return nil
}

func listProviders(out io.Writer, configPath string) error {
	var (
		r   *provider.Registry
		err error
	)
	if configPath != "" {
		r, err = provider.Load(configPath)
	} else {
		r, err = provider.LoadDefault()
	}
	if err != nil {
		return err
	}
	for _, name := range r.Names() {
		c, _ := r.Client(name)
		fmt.Fprintf(out, "%-15s %-40s models=%s default=%s\n", name, c.BaseURL(), strings.Join(c.Models(), ", "), c.Model())
	}
	return nil
}

// runTask runs one autonomous task (D4): quiet, final reply only.
func runTask(out io.Writer, task string) error {
	a, err := buildAgent("", nil, nil)
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
