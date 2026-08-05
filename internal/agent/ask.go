package agent

import (
	"context"
	"fmt"

	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/tools"
)

// askTool is the user's model of a subagent (D49): ONE agent, and a
// "subagent" is just a single API call to another provider/key — no nested
// loop, no tools, no memory, nothing persists. The reply comes back as the
// tool result. Ephemeral by nature: the call IS the subagent.
func (a *Agent) askTool() tools.Tool {
	return tools.Tool{
		Name:        "ask",
		Description: "Ask another model (a different provider/API key) one question. One-shot call: the other model has NO tools and NO memory — it just answers. Returns only its text reply. Use for a second opinion, a review, a different perspective, or parallel questions (multiple ask calls in one response run concurrently).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt":   map[string]any{"type": "string", "description": "The self-contained question for the other model."},
				"provider": map[string]any{"type": "string", "description": "Provider name from fender.toml (its own key). Empty = the configured subagent provider, else the parent's model."},
			},
			"required": []string{"prompt"},
		},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			prompt, _ := args["prompt"].(string)
			if prompt == "" {
				return "", fmt.Errorf("ask: empty prompt")
			}
			name, _ := args["provider"].(string)
			if name == "" {
				name = a.DefaultSubagent
			}
			llm := a.LLM
			if name != "" {
				if a.Resolver == nil {
					return "", fmt.Errorf("ask: provider %q requested but no resolver is configured", name)
				}
				child, err := a.Resolver(name)
				if err != nil {
					return "", fmt.Errorf("ask: %v", err)
				}
				llm = child
			}
			resp, err := llm.Chat(ctx, provider.Request{
				Messages: []provider.Message{{Role: "user", Content: prompt}},
			})
			if err != nil {
				return "", fmt.Errorf("ask: %v", err)
			}
			if len(resp.Choices) == 0 {
				return "", fmt.Errorf("ask: empty response")
			}
			return resp.Choices[0].Message.Content, nil
		},
	}
}
