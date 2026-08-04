package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/tools"
)

// Resolver selects a provider for a subagent by fender.toml name (D7).
// Nil means subagents inherit the parent's LLM.
type Resolver func(name string) (LLM, error)

const subagentSystem = "You are an ephemeral subagent of the Fender coding agent. Work on exactly the task you are given, using the available tools. When done, call complete_task alone with status complete and the final answer. If you need something only the parent can provide, call complete_task with status blocked and the exact blocker. Do not ask questions."

// delegateTool is D13: subagent-as-a-tool — the same Agent type runs in a
// goroutine with its own LLM and returns its final reply as the tool result.
// Children get the parent's registry minus delegate (one level of nesting).
func (a *Agent) delegateTool() tools.Tool {
	return tools.Tool{
		Name:        "delegate",
		Description: "Run an isolated subagent (the same agent loop, fresh context) on a self-contained subtask: research, investigation, or a bounded change. Returns only the subagent's final reply.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt":   map[string]any{"type": "string", "description": "The full, self-contained task for the subagent."},
				"provider": map[string]any{"type": "string", "description": "Provider name from fender.toml to run the subagent on (D7). Empty = inherit the parent's model."},
			},
			"required": []string{"prompt"},
		},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			prompt, _ := args["prompt"].(string)
			if prompt == "" {
				return "", errors.New("delegate: empty prompt")
			}
			llm := a.LLM
			if name, _ := args["provider"].(string); name != "" {
				if a.Resolver == nil {
					return "", fmt.Errorf("delegate: provider %q requested but no resolver is configured", name)
				}
				child, err := a.Resolver(name)
				if err != nil {
					return "", fmt.Errorf("delegate: %v", err)
				}
				llm = child
			}
			child := &Agent{
				LLM:        llm,
				Resolver:   a.Resolver,
				System:     subagentSystem,
				MaxIter:    a.subIter(),
				MaxSubIter: a.MaxSubIter,
				registry:   a.registry.Without("delegate"),
				Mem:        a.Mem,    // D39: delegates share project memory (artifact context still isolated below)
				Skills:     a.Skills, // D27: delegates share the skill registry
			}
			if a.Ctx != nil {
				child.Ctx = a.Ctx.Child() // D38: isolated artifacts + catalog
			}
			ch := make(chan *Result, 1)
			go func() {
				ch <- child.Run(ctx, []provider.Message{{Role: "user", Content: prompt}})
			}()
			select {
			case res := <-ch:
				if res.Status == "complete" || res.Status == "blocked" {
					return fmt.Sprintf("[delegate %s] %s", res.Status, res.Reply), nil
				}
				return "", fmt.Errorf("delegate %s: %s", res.Status, res.Reply)
			case <-ctx.Done():
				return "", ctx.Err()
			}
		},
	}
}
