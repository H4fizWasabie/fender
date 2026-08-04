package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
		fmt.Fprintln(out, "usage: fender [--config PATH] <command>")
		fmt.Fprintln(out, "  providers          list configured providers")
		fmt.Fprintln(out, "  skill install SRC  install skills from a local path or git URL")
		return nil
	}
	switch fs.Arg(0) {
	case "providers":
		return listProviders(out, *configPath)
	case "skill":
		return skillCommand(out, fs.Args()[1:])
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
