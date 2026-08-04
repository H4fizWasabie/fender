package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Searcher is the codebase-search backend seam (D10): graphify/cce/codegraph
// plug in here later; v1 ships the walk-based default (Task 4).
type Searcher func(query string) ([]SearchResult, error)

// SearchResult is one match: file, 1-based line, matching text.
type SearchResult struct {
	Path string
	Line int
	Text string
}

const maxSearchResults = 50

var searchSkipDirs = map[string]bool{
	".git": true, ".fender": true, ".scratch": true, "node_modules": true,
	"vendor": true, "dist": true, "build": true, "graphify-out": true,
}

var searchSkipExts = map[string]bool{
	".lock": true, ".sum": true, ".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".webp": true, ".ico": true, ".pdf": true, ".zip": true,
	".tar": true, ".gz": true, ".so": true, ".a": true, ".exe": true,
}

// DefaultSearcher walks projectDir, skipping build/vendor dirs and binary
// files, and returns case-insensitive substring matches (max 50).
func DefaultSearcher(projectDir string) Searcher {
	return func(query string) ([]SearchResult, error) {
		if query == "" {
			return nil, fmt.Errorf("search: empty query")
		}
		q := strings.ToLower(query)
		var out []SearchResult
		err := filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if d.IsDir() {
				if path != projectDir && searchSkipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if searchSkipExts[strings.ToLower(filepath.Ext(d.Name()))] {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			head := make([]byte, 8192)
			n, _ := f.Read(head)
			if bytes.Contains(head[:n], []byte{0}) {
				return nil // binary sniff
			}
			if _, err := f.Seek(0, 0); err != nil {
				return nil
			}
			sc := bufio.NewScanner(f)
			line := 0
			for sc.Scan() {
				line++
				if strings.Contains(strings.ToLower(sc.Text()), q) {
					out = append(out, SearchResult{Path: path, Line: line, Text: sc.Text()})
					if len(out) >= maxSearchResults {
						return filepath.SkipAll
					}
				}
			}
			return nil
		})
		if err != nil && err != fs.SkipAll {
			return nil, err
		}
		return out, nil
	}
}

func searchTool(projectDir string, searcher Searcher) Tool {
	if searcher == nil {
		searcher = DefaultSearcher(projectDir)
	}
	return Tool{
		Name:        "search",
		Description: "Case-insensitive substring search over project files (skips .git, vendored, and binary files). Returns up to 50 matches as path:line: text.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
			},
			"required": []string{"query"},
		},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			query, _ := args["query"].(string)
			results, err := searcher(query)
			if err != nil {
				return "", err
			}
			if len(results) == 0 {
				return "no matches", nil
			}
			var sb strings.Builder
			for _, r := range results {
				fmt.Fprintf(&sb, "%s:%d: %s\n", r.Path, r.Line, r.Text)
			}
			return strings.TrimSuffix(sb.String(), "\n"), nil
		},
	}
}
