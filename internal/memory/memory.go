package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	NotesMaxAge = 7 * 24 * time.Hour // working/ prune window (mino working_memory.md rule)
	SystemCap   = 8000               // always-loaded layer ceiling (prevention over compression)
)

const projectTemplate = `# PROJECT.md — always-loaded memory (Layer 0)

What this project is, conventions, build commands. Keep small (<2K chars).
`

const mapTemplate = `# MAP.md — navigation (Layer 1)

_Not generated yet — code-intel (ticket 07) replaces this body.
Maintain by hand until then: one "## <area>" section per module, one line each.
_

## Areas

- (none recorded yet)
`

type Memory struct {
	root string
}

func New(root string) *Memory {
	return &Memory{root: root}
}

// Child returns a distinct memory handle for an ephemeral child agent. Both
// handles read the same canonical project memory; child conversation and
// artifacts live outside Memory and remain isolated by Agent and context.
func (m *Memory) Child() *Memory {
	if m == nil {
		return nil
	}
	return &Memory{root: m.root}
}

// Ensure creates the .fender/ workspace if missing. Idempotent: never
// overwrites existing files (user edits survive).
func (m *Memory) Ensure() error {
	dirs := []string{
		filepath.Join(m.root, ".fender", "memory", "reference"),
		filepath.Join(m.root, ".fender", "memory", "working"),
		filepath.Join(m.root, ".fender", "memory", "facts"), // reserved (D39)
		filepath.Join(m.root, ".fender", "skills"),          // ticket 06 seam
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0700); err != nil {
			return fmt.Errorf("memory ensure: %w", err)
		}
	}
	files := []struct {
		path    string
		content string
	}{
		{filepath.Join(m.root, ".fender", "memory", "PROJECT.md"), projectTemplate},
		{filepath.Join(m.root, ".fender", "memory", "MAP.md"), mapTemplate},
	}
	for _, f := range files {
		if _, err := os.Stat(f.path); os.IsNotExist(err) {
			if err := os.WriteFile(f.path, []byte(f.content), 0600); err != nil {
				return fmt.Errorf("memory ensure: %w", err)
			}
		}
	}
	return nil
}
