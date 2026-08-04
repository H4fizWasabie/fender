package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
)

//go:embed bundled
var bundledFS embed.FS

// Bundled loads all embedded skills. Every bundled skill must parse —
// a broken bundled skill is a build-time bug, surfaced here as an error.
func Bundled() (*Registry, error) {
	r := &Registry{all: map[string]Skill{}}
	entries, err := fs.ReadDir(bundledFS, "bundled")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		data, err := bundledFS.ReadFile(filepath.Join("bundled", name, "SKILL.md"))
		if err != nil {
			return nil, fmt.Errorf("bundled %s: %w", name, err)
		}
		m, body, ok := parseFrontmatter(string(data))
		if !ok {
			return nil, fmt.Errorf("bundled %s: frontmatter parse failed", name)
		}
		r.all[m.Name] = Skill{
			Name: m.Name, Description: m.Description, Body: body,
			Source: "bundled", Path: "bundled/" + name + "/SKILL.md",
			ModelInvokable: m.ModelInvokable,
		}
	}
	return r, nil
}
