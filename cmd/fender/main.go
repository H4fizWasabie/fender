package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/H4fizWasabie/fender/internal/provider"
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
		fmt.Fprintln(out, "usage: fender [--config PATH] providers")
		fmt.Fprintln(out, "  providers   list configured providers")
		return nil
	}
	switch fs.Arg(0) {
	case "providers":
		return listProviders(out, *configPath)
	default:
		return fmt.Errorf("unknown command %q", fs.Arg(0))
	}
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
