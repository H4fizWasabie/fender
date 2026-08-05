package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func editTool(projectDir string, ruleLoader ...func(dir string) string) Tool {
	var loader func(dir string) string
	if len(ruleLoader) > 0 {
		loader = ruleLoader[0]
	}
	return Tool{
		Name:        "edit_file",
		Description: "Replace a unique occurrence of old_text with new_text in a file inside the project directory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":     map[string]any{"type": "string"},
				"old_text": map[string]any{"type": "string", "description": "Exact text to replace; must occur exactly once"},
				"new_text": map[string]any{"type": "string"},
			},
			"required": []string{"path", "old_text", "new_text"},
		},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			oldText, _ := args["old_text"].(string)
			newText, _ := args["new_text"].(string)
			full, err := inProject(projectDir, path)
			if err != nil {
				return "", err
			}
			if oldText == "" {
				return "", fmt.Errorf("edit_file: old_text must not be empty")
			}
			data, err := os.ReadFile(full)
			if err != nil {
				return "", err
			}
			content := string(data)
			switch n := strings.Count(content, oldText); n {
			case 0:
				return "", fmt.Errorf("edit_file: old_text not found in %s", path)
			case 1:
				// ok
			default:
				return "", fmt.Errorf("edit_file: old_text occurs %d times in %s; include more context", n, path)
			}
			info, err := os.Stat(full)
			if err != nil {
				return "", err
			}
			out := strings.Replace(content, oldText, newText, 1)
			if err := os.WriteFile(full, []byte(out), info.Mode().Perm()); err != nil {
				return "", err
			}
			var rules string
			if loader != nil {
				rules = loader(filepath.Dir(full)) // D46: nested AGENTS.md
			}
			return rules + fmt.Sprintf("edited %s (%d -> %d bytes)", path, len(content), len(out)), nil
		},
	}
}
