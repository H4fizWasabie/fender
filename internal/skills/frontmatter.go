package skills

import (
	"regexp"
	"strings"
)

// meta is the parsed SKILL.md frontmatter. The format is narrow (name +
// description + optional disable-model-invocation), so a regex parser
// replaces a YAML dependency (ponytail ladder).
type meta struct {
	Name           string
	Description    string
	ModelInvokable bool
}

var (
	nameRe    = regexp.MustCompile(`(?m)^name:\s*(.+?)\s*$`)
	descRe    = regexp.MustCompile(`(?m)^description:\s*(.+?)\s*$`)
	disableRe = regexp.MustCompile(`(?m)^disable-model-invocation:\s*true\s*$`)
	foldedRe  = regexp.MustCompile(`(?m)^description:\s*>\s*$`)
)

// parseFrontmatter extracts (meta, body, ok) from a SKILL.md file.
// ok=false on any structural failure — caller skips the skill with a warning.
func parseFrontmatter(content string) (meta, string, bool) {
	if !strings.HasPrefix(content, "---") {
		return meta{}, "", false
	}
	end := strings.Index(content[3:], "---")
	if end < 0 {
		return meta{}, "", false
	}
	head := content[:3+end]
	body := strings.TrimSpace(content[3+end+3:])
	m := meta{ModelInvokable: true}

	nameMatch := nameRe.FindStringSubmatch(head)
	if nameMatch == nil || strings.TrimSpace(nameMatch[1]) == "" {
		return meta{}, "", false
	}
	m.Name = strings.TrimSpace(nameMatch[1])

	if disableRe.MatchString(head) {
		m.ModelInvokable = false
	}

	if foldedRe.MatchString(head) {
		// description: >  followed by indented lines
		var lines []string
		idx := strings.Index(head, "description:")
		rest := head[idx+len("description:"):]
		for _, line := range strings.Split(rest, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == ">" || trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "disable-model-invocation:") {
				break
			}
			if strings.Contains(line, ":") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				break // next frontmatter key
			}
			lines = append(lines, trimmed)
		}
		m.Description = strings.Join(lines, " ")
	} else {
		descMatch := descRe.FindStringSubmatch(head)
		if descMatch == nil || strings.TrimSpace(descMatch[1]) == "" {
			return meta{}, "", false
		}
		m.Description = strings.Trim(strings.TrimSpace(descMatch[1]), `"`)
	}
	if m.Name == "" || m.Description == "" {
		return meta{}, "", false
	}
	return m, body, true
}
