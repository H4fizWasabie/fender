package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// readCap is the inline safety ceiling for read_file output. read_file is
// never compacted (D31: its result is the explicit slice the model asked
// for), so this cap is the only bound on inline size — it stays.
const readCap = 1 << 20 // 1 MiB

func readTool(projectDir string, ruleLoader ...func(dir string) string) Tool {
	var loader func(dir string) string
	if len(ruleLoader) > 0 {
		loader = ruleLoader[0]
	}
	return Tool{
		Name:        "read_file",
		Description: "Read a file inside the project directory. Use offset and limit (1-based) to fetch only the line range you need instead of the whole file — prefer slices for large files. Read before editing, and re-read after edits to verify.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string", "description": "Path to the file (absolute or relative to the project root)"},
				"offset": map[string]any{"type": "integer", "description": "First line to return (1-based)"},
				"limit":  map[string]any{"type": "integer", "description": "Maximum number of lines to return"},
			},
			"required": []string{"path"},
		},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			full, err := inProject(projectDir, path)
			if err != nil {
				return "", err
			}
			var rules string
			if loader != nil {
				rules = loader(filepath.Dir(full)) // D46: nested AGENTS.md
			}
			data, err := os.ReadFile(full)
			if err != nil {
				return "", err
			}
			if len(data) > readCap {
				data = data[:readCap]
				return rules + string(data) + fmt.Sprintf("\n... (truncated at %d bytes)\n", readCap), nil
			}
			text := rules + string(data)
			off, hasOff := intArg(args, "offset")
			if !hasOff {
				return text, nil
			}
			lines := strings.Split(text, "\n")
			if off < 1 {
				return "", fmt.Errorf("read_file: offset must be >= 1")
			}
			if off > len(lines) {
				return "", nil
			}
			end := len(lines)
			if lim, ok := intArg(args, "limit"); ok {
				if lim < 1 {
					return "", fmt.Errorf("read_file: limit must be >= 1")
				}
				if off-1+lim < end {
					end = off - 1 + lim
				}
			}
			return strings.Join(lines[off-1:end], "\n"), nil
		},
	}
}

// intArg reads an integer tool argument (JSON numbers decode as float64).
func intArg(args map[string]any, key string) (int, bool) {
	switch n := args[key].(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}
