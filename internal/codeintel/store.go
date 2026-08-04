package codeintel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Stamp struct {
	MtimeNanos int64 `json:"mtime_nanos"`
	Size       int64 `json:"size"`
}

// Store owns .fender/codeintel/: stamps, extraction cache, graph (D34-3).
type Store struct {
	dir         string
	projectDir  string
	stamps      map[string]Stamp
	extractions map[string]Extraction
	graph       *Graph
}

func Open(root string) (*Store, error) {
	dir := filepath.Join(root, ".fender", "codeintel")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, projectDir: root, stamps: map[string]Stamp{}, extractions: map[string]Extraction{}}
	s.load()
	return s, nil
}

func (s *Store) load() {
	if data, err := os.ReadFile(filepath.Join(s.dir, "stamps.json")); err == nil {
		json.Unmarshal(data, &s.stamps)
	}
	if data, err := os.ReadFile(filepath.Join(s.dir, "extractions.json")); err == nil {
		json.Unmarshal(data, &s.extractions)
	}
}

func (s *Store) save() error {
	stamps, err := json.Marshal(s.stamps)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.dir, "stamps.json"), stamps, 0600); err != nil {
		return err
	}
	ex, err := json.Marshal(s.extractions)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "extractions.json"), ex, 0600)
}

// skipDirs are never indexed (build/vendor noise).
var skipDirs = map[string]bool{".git": true, ".fender": true, "vendor": true, "node_modules": true, ".venv": true, "dist": true, "build": true}

// Refresh re-extracts only stamp-changed files (D34-3, graphify cache.py
// pattern). Returns the number of files re-extracted.
func (s *Store) Refresh() (int, error) {
	changed := 0
	err := filepath.WalkDir(s.projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != s.projectDir && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if _, ok := specFor(path); !ok {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		st := Stamp{MtimeNanos: info.ModTime().UnixNano(), Size: info.Size()}
		if prev, ok := s.stamps[path]; ok && prev == st {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		ex, err := ExtractFile(path, src)
		if err != nil {
			return nil
		}
		s.extractions[path] = ex
		s.stamps[path] = st
		changed++
		return nil
	})
	if err != nil {
		return changed, err
	}
	// prune stamps/extractions for deleted files
	for path := range s.stamps {
		if _, err := os.Stat(filepath.Join(s.projectDir, path)); err != nil {
			delete(s.stamps, path)
			delete(s.extractions, path)
		}
	}
	if changed > 0 {
		return changed, s.save()
	}
	return changed, nil
}

func (s *Store) Extractions() map[string]Extraction { return s.extractions }

func (s *Store) Rebuild() (*Graph, error) {
	g := Build(s.extractions)
	s.graph = g
	data, err := json.Marshal(g)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(s.dir, "graph.json"), data, 0600); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *Store) LoadGraph() *Graph {
	if s.graph != nil {
		return s.graph
	}
	if data, err := os.ReadFile(filepath.Join(s.dir, "graph.json")); err == nil {
		g := &Graph{}
		json.Unmarshal(data, g)
		s.graph = g
	}
	return s.graph
}
