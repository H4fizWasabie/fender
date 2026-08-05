package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/H4fizWasabie/fender/internal/provider"
)

// sessionFile is one persisted REPL session (D41). The history carries
// absolute artifact pointers, so resumed sessions fetch slices identically
// while /tmp artifacts survive the 24h sweep.
type sessionFile struct {
	ID           string             `json:"id"`
	Started      string             `json:"started"`
	Updated      string             `json:"updated,omitempty"`
	Status       string             `json:"status,omitempty"`
	Messages     []provider.Message `json:"messages"`
	Consolidated bool               `json:"consolidated,omitempty"` // D43
}

func sessionsDir() (string, error) {
	dir := filepath.Join(".fender", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// saveSession writes the session atomically (temp + rename).
func saveSession(s *sessionFile) error {
	dir, err := sessionsDir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, s.ID+".tmp")
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, s.ID+".json"))
}

// loadLatestSession returns the newest session file, or nil when none.
func loadLatestSession() (*sessionFile, error) {
	files, err := listSessions()
	if err != nil || len(files) == 0 {
		return nil, err
	}
	return loadSession(files[0].ID)
}

// loadSession restores one explicitly selected session. IDs are filenames,
// not paths; rejecting separators keeps the HTTP and CLI resume seams inside
// .fender/sessions.
func loadSession(id string) (*sessionFile, error) {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\\`) {
		return nil, fmt.Errorf("invalid session id %q", id)
	}
	dir, err := sessionsDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		return nil, err
	}
	var s sessionFile
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.ID != id {
		return nil, fmt.Errorf("session id mismatch: requested %q, file contains %q", id, s.ID)
	}
	return &s, nil
}

// listSessions returns saved sessions, newest first.
func listSessions() ([]sessionFile, error) {
	dir, err := sessionsDir()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []sessionFile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var s sessionFile
		if json.Unmarshal(data, &s) == nil && s.ID != "" {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func newSessionID() string {
	return time.Now().Format("20060102-150405.000000000")
}

func formatSessions(files []sessionFile) string {
	var s string
	for _, f := range files {
		s += fmt.Sprintf("%s  started=%s  %d messages\n", f.ID, f.Started, len(f.Messages))
	}
	return s
}
