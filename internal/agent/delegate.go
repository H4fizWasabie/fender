package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/tools"
)

const subagentSystem = "You are an ephemeral subagent of the Fender coding agent. Work on exactly the task you are given, using the available tools. When the task is done, stop — your final text is the answer the parent receives. If you need something only the parent can provide, say so in your final text. Do not ask questions."

// delegateTool is D13/D50: child-agent-as-a-tool. The same Agent type runs
// synchronously with fresh context and returns its final reply as the tool
// result. Children get the parent's registry minus delegate (one level only).
func (a *Agent) delegateTool() tools.Tool {
	return tools.Tool{
		Name:        "delegate",
		Description: "Run an ephemeral child agent (the same loop, fresh context and working memory) on ONE self-contained subtask: research, investigation, or a bounded change. The child cannot delegate; its final text is the answer. Give it a complete, self-contained prompt — it has no memory of this conversation.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt": map[string]any{"type": "string", "description": "The full, self-contained task for the child agent."},
			},
			"required": []string{"prompt"},
		},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			prompt, _ := args["prompt"].(string)
			if prompt == "" {
				return "", errors.New("delegate: empty prompt")
			}
			child := &Agent{
				LLM:        a.LLM, // same provider chain; backup key is resilience, not identity
				System:     subagentSystem,
				MaxIter:    a.subIter(),
				MaxSubIter: a.MaxSubIter,
				registry:   a.registry.Without("delegate"), // one level; no grandchildren
				Mem:        a.Mem.Child(),                  // fresh handle over canonical project grounding
				Skills:     a.Skills,
			}
			// The child's live stream remains observable, but it is one
			// ephemeral child rather than a provider-addressed agent graph.
			if a.Observer != nil {
				child.Observer = func(e Event) {
					e.Source = "child"
					a.Observer(e)
				}
			}
			res := child.Run(ctx, []provider.Message{{Role: "user", Content: prompt}})
			if res.Status == "complete" || res.Status == "blocked" {
				return fmt.Sprintf("[delegate %s] %s", res.Status, res.Reply), nil
			}
			return "", fmt.Errorf("delegate %s: %s", res.Status, res.Reply)
		},
	}
}
