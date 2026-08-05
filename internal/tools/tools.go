// Package tools implements the agent's tool set (D10): read_file,
// edit_file, shell, search — with a backend seam for codebase search.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/H4fizWasabie/fender/internal/provider"
)

// Tool is one callable tool: a JSON schema + a handler.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON schema object
	Call        func(ctx context.Context, args map[string]any) (string, error)
}

// Registry holds the tools of one agent. Subagents get a Registry too
// (minus delegate), so guardrails wrap tool execution once (D13).
type Registry struct {
	tools map[string]Tool
	order []string
}

func (r *Registry) Add(t Tool) {
	r.tools[t.Name] = t
	r.order = append(r.order, t.Name)
}

// Without returns a shallow copy minus the named tools (subagent subsets).
func (r *Registry) Without(exclude ...string) *Registry {
	skip := make(map[string]bool, len(exclude))
	for _, n := range exclude {
		skip[n] = true
	}
	c := &Registry{tools: make(map[string]Tool, len(r.tools))}
	for _, n := range r.order {
		if skip[n] {
			continue
		}
		c.tools[n] = r.tools[n]
		c.order = append(c.order, n)
	}
	return c
}

func (r *Registry) Names() []string { return r.order }

// Schemas converts the registry to OpenAI tool definitions.
func (r *Registry) Schemas() []provider.ToolDef {
	out := make([]provider.ToolDef, 0, len(r.order))
	for _, n := range r.order {
		t := r.tools[n]
		out = append(out, provider.ToolDef{
			Type: "function",
			Function: provider.ToolFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return out
}

// New returns the standard v1 registry. ruleLoader (nil-safe) returns
// nested-AGENTS.md rules for a target directory (D46) — prepended to
// read/edit outputs so the model follows directory-scoped conventions.
func New(projectDir string, shell ShellConfig, searcher Searcher, ruleLoader ...func(dir string) string) *Registry {
	var loader func(dir string) string
	if len(ruleLoader) > 0 {
		loader = ruleLoader[0]
	}
	r := &Registry{tools: make(map[string]Tool)}
	r.Add(readTool(projectDir, loader))
	r.Add(editTool(projectDir, loader))
	r.Add(shellTool(shell))
	r.Add(searchTool(projectDir, searcher))
	return r
}

// Execute runs one tool by name with JSON-encoded args.
func (r *Registry) Execute(ctx context.Context, name, argsJSON string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q (have: %v)", name, r.order)
	}
	var args map[string]any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("%s: bad args: %v", name, err)
		}
	}
	return t.Call(ctx, args)
}
