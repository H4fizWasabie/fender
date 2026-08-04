package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Bootstrap struct {
	Convention []ConventionFile
	ProjectMD  string // PROJECT.md content, "" if absent
	MAPMD      string // MAP.md content, "" if absent (navigation is on-demand, never always-loaded)
}

// Bootstrap = Ensure + Detect + read PROJECT.md/MAP.md.
// Unreadable convention files are skipped, never an error (a broken
// rules file must not kill the session).
func (m *Memory) Bootstrap() (*Bootstrap, error) {
	if err := m.Ensure(); err != nil {
		return nil, err
	}
	m.pruneWorking()
	b := &Bootstrap{Convention: m.Detect(m.root)}
	b.ProjectMD = readQuiet(filepath.Join(m.root, ".fender", "memory", "PROJECT.md"))
	b.MAPMD = readQuiet(filepath.Join(m.root, ".fender", "memory", "MAP.md"))
	return b, nil
}

func readQuiet(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// pruneWorking removes working/*.md older than NotesMaxAge. patterns.md is
// exempt — durable operational knowledge (mino patterns.md rule).
func (m *Memory) pruneWorking() {
	dir := filepath.Join(m.root, ".fender", "memory", "working")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "patterns.md" {
			continue
		}
		info, err := e.Info()
		if err != nil || time.Since(info.ModTime()) > NotesMaxAge {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// Working lists surviving working files: "<basename>: <path> (<age>)".
func (m *Memory) Working() []string {
	dir := filepath.Join(m.root, ".fender", "memory", "working")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		age := time.Since(info.ModTime()).Round(time.Hour)
		out = append(out, fmt.Sprintf("%s: %s (%s)", e.Name(), filepath.Join(dir, e.Name()), age))
	}
	return out
}

// System composes the always-loaded layer: convention files (in precedence
// order) + PROJECT.md, provenance-marked. Capped at SystemCap — excess is
// truncated oldest-first with a marker (prevention over compression, D14).
func (b *Bootstrap) System() string {
	var sb strings.Builder
	for _, cf := range b.Convention {
		content := readQuiet(cf.Path)
		if content == "" {
			continue
		}
		fmt.Fprintf(&sb, "<<%s (%s): %s>>\n%s\n", cf.Kind, cf.Layer, cf.Path, strings.TrimSpace(content))
	}
	if b.ProjectMD != "" {
		sb.WriteString("\n<<PROJECT.md>>\n")
		sb.WriteString(strings.TrimSpace(b.ProjectMD))
	}
	if sb.Len() <= SystemCap {
		return sb.String()
	}
	// truncate oldest sections first: keep only the tail that fits, with a marker
	drop := sb.Len() - SystemCap
	marker := "\n[memory: earlier layers truncated — cap 8K]\n"
	kept := sb.String()[drop+len(marker):]
	return marker + kept
}
