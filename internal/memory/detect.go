package memory

import (
	"os"
	"path/filepath"
)

// ConventionFile is a detected project-rules file. Content is read at
// assembly time (System), never copied into PROJECT.md (canonical sources).
type ConventionFile struct {
	Path  string // absolute path
	Kind  string // "AGENTS.md" | "CLAUDE.md" | "CONTEXT.md"
	Layer string // "user" | "project"
}

// Detect finds convention files for dir, in precedence order:
// user ~/.fender/AGENTS.md → project AGENTS.md (fallback CLAUDE.md) → CONTEXT.md.
// README.md / CONTRIBUTING.md / .cursorrules are never auto-loaded.
// Dir-aware for the nested-AGENTS.md seam (v1 loads root level only).
func (m *Memory) Detect(dir string) []ConventionFile {
	var out []ConventionFile
	home, err := os.UserHomeDir()
	if err == nil {
		if p := filepath.Join(home, ".fender", "AGENTS.md"); exists(p) {
			out = append(out, ConventionFile{Path: p, Kind: "AGENTS.md", Layer: "user"})
		}
	}
	agents := filepath.Join(dir, "AGENTS.md")
	if exists(agents) {
		out = append(out, ConventionFile{Path: agents, Kind: "AGENTS.md", Layer: "project"})
	} else if claude := filepath.Join(dir, "CLAUDE.md"); exists(claude) {
		out = append(out, ConventionFile{Path: claude, Kind: "CLAUDE.md", Layer: "project"})
	}
	if ctx := filepath.Join(dir, "CONTEXT.md"); exists(ctx) {
		out = append(out, ConventionFile{Path: ctx, Kind: "CONTEXT.md", Layer: "project"})
	}
	return out
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
