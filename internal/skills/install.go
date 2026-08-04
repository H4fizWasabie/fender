package skills

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Install copies every "<src>/*/SKILL.md" directory into destDir.
// src may be a local path or a git URL (cloned to a temp dir first).
// Returns the installed skill names, sorted.
func Install(src, destDir string) ([]string, error) {
	local := src
	cleanup := func() {}
	if isGitURL(src) {
		tmp, err := os.MkdirTemp("", "fender-skill-*")
		if err != nil {
			return nil, err
		}
		cleanup = func() { os.RemoveAll(tmp) }
		cmd := exec.Command("git", "clone", "--depth", "1", src, tmp)
		if out, err := cmd.CombinedOutput(); err != nil {
			cleanup()
			return nil, fmt.Errorf("clone %s: %w: %s", src, err, strings.TrimSpace(string(out)))
		}
		local = tmp
	}
	defer cleanup()

	entries, err := os.ReadDir(local)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sp := filepath.Join(local, e.Name(), "SKILL.md")
		if _, err := os.Stat(sp); err != nil {
			continue // not a skill dir
		}
		dest := filepath.Join(destDir, e.Name())
		if err := copyDir(sp, dest); err != nil {
			return nil, fmt.Errorf("install %s: %w", e.Name(), err)
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

func isGitURL(s string) bool {
	return strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "git@")
}

// copyDir copies a skill dir's files (SKILL.md + support) into dest.
func copyDir(srcSkillFile, dest string) error {
	src := filepath.Dir(srcSkillFile)
	if err := os.MkdirAll(dest, 0700); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0700); err != nil {
			return err
		}
		return copyFile(path, out)
	})
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
