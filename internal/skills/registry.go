package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	MatchTopN          = 3
	BodyBudget         = 8000
	DescriptionListCap = 4000
)

type Skill struct {
	Name           string
	Description    string
	Body           string
	Source         string // "bundled" | "user" | "project"
	Path           string
	ModelInvokable bool
}

type Registry struct {
	all map[string]Skill
}

// Load scans dir/*/SKILL.md. A missing dir is an empty registry, nil error.
// Broken skills are skipped (warning-worthy, never fatal).
func Load(dir string) (*Registry, error) {
	r := &Registry{all: map[string]Skill{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sp := filepath.Join(dir, e.Name(), "SKILL.md")
		data, err := os.ReadFile(sp)
		if err != nil {
			continue
		}
		m, body, ok := parseFrontmatter(string(data))
		if !ok {
			continue // broken installed skill — skip, never fatal
		}
		r.all[m.Name] = Skill{Name: m.Name, Description: m.Description, Body: body, Source: layer(dir), Path: sp, ModelInvokable: m.ModelInvokable}
	}
	return r, nil
}

func layer(dir string) string {
	if strings.Contains(dir, ".fender") {
		return "project"
	}
	return "user"
}

// Merge returns a new registry with precedence project > user > r (receiver
// is the bundled registry).
func (r *Registry) Merge(project, user *Registry) *Registry {
	out := &Registry{all: map[string]Skill{}}
	for name, s := range r.all {
		out.all[name] = s
	}
	for name, s := range user.all {
		out.all[name] = s
	}
	for name, s := range project.all {
		out.all[name] = s
	}
	return out
}

func (r *Registry) ByName(name string) (Skill, bool) {
	s, ok := r.all[name]
	return s, ok
}

// Descriptions is the always-loaded catalog: one line per skill, capped.
func (r *Registry) Descriptions() string {
	names := make([]string, 0, len(r.all))
	for n := range r.all {
		names = append(names, n)
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, n := range names {
		s := r.all[n]
		line := fmt.Sprintf("- %s: %s\n", s.Name, s.Description)
		if sb.Len()+len(line) > DescriptionListCap {
			sb.WriteString("- ...(catalog truncated)\n")
			break
		}
		sb.WriteString(line)
	}
	return sb.String()
}

// PonytailCore is the always-loaded discipline (D30): the bundled ponytail
// skill, never matched, never shadowed by installs.
func (r *Registry) PonytailCore() (Skill, bool) {
	s, ok := r.all["ponytail"]
	return s, ok && s.Source == "bundled"
}
