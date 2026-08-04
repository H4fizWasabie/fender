package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestManager points the artifact root at a temp dir so tests never
// touch /tmp/fender (and Cleanup never sweeps real runs).
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m := New()
	m.Root = filepath.Join(t.TempDir(), "run")
	return m
}

// artifactPath extracts the path from "[artifact: tool → N chars at PATH; ...]".
func artifactPath(t *testing.T, pointer string) string {
	t.Helper()
	_, after, ok := strings.Cut(pointer, " at ")
	if !ok {
		t.Fatalf("no path in %q", pointer)
	}
	path, _, _ := strings.Cut(after, ";")
	return path
}

func TestCompactOutputUnderLimitInline(t *testing.T) {
	m := newTestManager(t)
	output := strings.Repeat("x", InlineLimit)
	if got := m.CompactOutput("shell", output); got != output {
		t.Fatalf("inline output changed")
	}
}

func TestCompactOutputWritesArtifact(t *testing.T) {
	m := newTestManager(t)
	output := strings.Repeat("x", InlineLimit+1)
	got := m.CompactOutput("shell", output)
	if !strings.Contains(got, "[artifact:") {
		t.Fatalf("no artifact pointer: %.60q", got)
	}
	path := artifactPath(t, got)
	data, err := os.ReadFile(path)
	if err != nil || string(data) != output {
		t.Fatalf("artifact read: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat artifact: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("artifact perms = %o, want 600", info.Mode().Perm())
	}
	if cat := m.Catalog(); !strings.Contains(cat, path) {
		t.Fatalf("catalog missing artifact: %q", cat)
	}
}

func TestCompactOutputKeepsReadInline(t *testing.T) {
	m := newTestManager(t)
	output := strings.Repeat("x", InlineLimit+1)
	if got := m.CompactOutput("read_file", output); got != output {
		t.Fatal("read_file was compacted (D31: the one tool never compacted)")
	}
}

func TestCompactOutputNoOverwrite(t *testing.T) {
	m := newTestManager(t)
	p1 := artifactPath(t, m.CompactOutput("shell", strings.Repeat("a", InlineLimit+1)))
	p2 := artifactPath(t, m.CompactOutput("shell", strings.Repeat("b", InlineLimit+1)))
	if p1 == p2 {
		t.Fatal("repeated tool calls share an artifact path")
	}
	if data, _ := os.ReadFile(p1); string(data) != strings.Repeat("a", InlineLimit+1) {
		t.Fatal("first artifact overwritten")
	}
}

func TestChildIsolation(t *testing.T) {
	m := newTestManager(t)
	m.CompactOutput("shell", strings.Repeat("x", InlineLimit+1))
	c := m.Child()
	if c.Root == m.Root {
		t.Fatalf("child root %q equals parent root", c.Root)
	}
	if c.Catalog() != "" {
		t.Fatal("child catalog must start empty")
	}
	if c.ContextChars != m.ContextChars || c.MaxHistoryTurns != m.MaxHistoryTurns {
		t.Fatal("child settings diverged from parent")
	}
}

func TestCleanupRemovesStale(t *testing.T) {
	// Sweep granularity is RUN dirs (base-dir entries, mino-style): the
	// whole stale run goes, the fresh run stays.
	base := t.TempDir()
	m := New()
	m.Root = filepath.Join(base, "run-fresh")
	if err := os.MkdirAll(m.Root, 0o700); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(base, "run-stale", "1", "old.txt")
	if err := os.MkdirAll(filepath.Dir(stale), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("z"), 0o600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	os.Chtimes(filepath.Dir(filepath.Dir(stale)), past, past)
	m.Cleanup(time.Hour)
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("stale run survived sweep")
	}
	if _, err := os.Stat(m.Root); err != nil {
		t.Fatal("fresh run swept")
	}
}
