package agent

import (
	"encoding/json"
	"strings"

	"github.com/H4fizWasabie/fender/internal/provider"
)

const (
	completionToolName = "complete_task"
	completionError    = "Error: complete_task must be called alone with status complete|blocked and a non-empty reply."
)

// completeSchema is the terminal protocol tool (D37, ported from mino):
// only complete_task can end the turn; plain text is progress, not completion.
func completeSchema() provider.ToolDef {
	return provider.ToolDef{
		Type: "function",
		Function: provider.ToolFunctionDef{
			Name:        completionToolName,
			Description: "Finish the task. Call ALONE only after all work is complete (status complete) or genuinely blocked (status blocked), with the final user-facing reply.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{"type": "string", "enum": []string{"complete", "blocked"}},
					"reply":  map[string]any{"type": "string"},
				},
				"required": []string{"status", "reply"},
			},
		},
	}
}

// completionArgs extracts and normalizes status/reply from the call args.
func completionArgs(argsJSON string) (status, reply string) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", ""
	}
	status, _ = args["status"].(string)
	reply, _ = args["reply"].(string)
	return strings.ToLower(strings.TrimSpace(status)), strings.TrimSpace(reply)
}

// canonicalArgs normalizes tool-call args so identical calls share a dedup
// key regardless of JSON key order (D32 layer 1: tool dedup).
func canonicalArgs(argsJSON string) string {
	var v any
	if err := json.Unmarshal([]byte(argsJSON), &v); err != nil {
		return argsJSON
	}
	b, _ := json.Marshal(v)
	return string(b)
}
