package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/provider"
)

// TestForBudgetBound is acceptance #3 — the load-bearing test: system +
// Σ messages ≤ ContextChars, with oversized user input compacted to
// HEAD/TAIL and the artifact catalog riding in context (port of mino
// TestContextForBoundsCurrentInputAndKeepsArtifactCatalog).
func TestForBudgetBound(t *testing.T) {
	m := newTestManager(t)
	m.ContextChars = 12000
	old := filepath.Join(m.Root, "old-result.txt")
	if err := os.MkdirAll(m.Root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("old result"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.record(Artifact{Label: "bash", Path: old, Size: 10})

	msgs := []provider.Message{
		{Role: "user", Content: "HEAD=orchid"},
		{Role: "assistant", Content: "ack"},
		{Role: "user", Content: strings.Repeat("x", 140000)},
		{Role: "assistant", Content: "middle"},
		{Role: "user", Content: strings.Repeat("u", 30000)}, // current task
	}
	system := strings.Repeat("s", 500)
	out := m.For(system, msgs)

	total := len(system)
	joined := ""
	for _, msg := range out {
		total += len(msg.Content)
		joined += msg.Content
	}
	if total > m.ContextChars {
		t.Fatalf("budget exceeded: %d > %d", total, m.ContextChars)
	}
	if !strings.Contains(joined, "large user input") || !strings.Contains(joined, old) {
		t.Fatalf("context lost input or catalog: %q", joined)
	}
}

// TestForKeepsLastTurnsOnly ports mino TestContextMessagesKeepsLastNTurnsOnly:
// only the last MaxHistoryTurns turns survive, with a compaction marker.
func TestForKeepsLastTurnsOnly(t *testing.T) {
	m := newTestManager(t)
	m.MaxHistoryTurns = 2
	msgs := []provider.Message{
		{Role: "user", Content: "turn1-q"}, {Role: "assistant", Content: "turn1-a"},
		{Role: "user", Content: "turn2-q"}, {Role: "assistant", Content: "turn2-a"},
		{Role: "user", Content: "turn3-q"}, {Role: "assistant", Content: "turn3-a"},
		{Role: "user", Content: "turn4-q"}, {Role: "assistant", Content: "turn4-a"},
		{Role: "user", Content: "turn5-q"}, {Role: "assistant", Content: "turn5-a"},
	}
	out := m.For("", msgs)
	joined := ""
	for _, msg := range out {
		joined += msg.Content
	}
	if !strings.Contains(joined, "turn4-q") || !strings.Contains(joined, "turn5-a") {
		t.Fatalf("last 2 turns missing: %q", joined)
	}
	if strings.Contains(joined, "turn1") || strings.Contains(joined, "turn2") || strings.Contains(joined, "turn3") {
		t.Fatalf("older turns leaked: %q", joined)
	}
	if !strings.Contains(joined, "3 earlier turns compacted") {
		t.Fatalf("compaction marker missing: %q", joined)
	}
}

// TestForCompactsOversizedHistory: non-user messages over the preview are
// replaced by a catalog note, never dropped silently (mino ContextMessages).
func TestForCompactsOversizedHistory(t *testing.T) {
	m := newTestManager(t)
	msgs := []provider.Message{
		{Role: "assistant", Content: strings.Repeat("z", PreviewLimit+1)},
		{Role: "user", Content: "current task"},
	}
	out := m.For("", msgs)
	if !strings.Contains(out[0].Content, "Large previous assistant message") {
		t.Fatalf("history not noted: %.80q", out[0].Content)
	}
	if out[1].Content != "current task" {
		t.Fatalf("current user message altered: %q", out[1].Content)
	}
}

// TestForCatalogRides: artifacts recorded before For() are listed in the
// returned messages (D31: select — the catalog rides in context).
func TestForCatalogRides(t *testing.T) {
	m := newTestManager(t)
	m.CompactOutput("shell", strings.Repeat("x", DefaultInlineLimit+1))
	out := m.For("", []provider.Message{{Role: "user", Content: "hi"}})
	found := false
	for _, msg := range out {
		if strings.Contains(msg.Content, "Live session artifacts") {
			found = true
		}
	}
	if !found {
		t.Fatal("catalog did not ride in context")
	}
}

// TestForBudgetDropsOldestPairs: when turns truncation alone cannot fit the
// budget, the oldest pairs are dropped — but the current user turn survives.
func TestForBudgetDropsOldestPairs(t *testing.T) {
	m := newTestManager(t)
	m.ContextChars = 9000
	m.MaxHistoryTurns = 100 // disable turns truncation; budget drop must do the work
	var msgs []provider.Message
	for i := 0; i < 12; i++ {
		msgs = append(msgs,
			provider.Message{Role: "user", Content: strings.Repeat("a", 7000) + fmt.Sprint(i)},
			provider.Message{Role: "assistant", Content: strings.Repeat("b", 7000)})
	}
	system := strings.Repeat("s", 1000)
	out := m.For(system, msgs)
	total := len(system)
	for _, msg := range out {
		total += len(msg.Content)
	}
	if total > m.ContextChars {
		t.Fatalf("budget exceeded: %d > %d", total, m.ContextChars)
	}
	// The current user turn survives the drops (the catalog is appended
	// after it, so scan for the last user message rather than out[len-1]).
	lastUser := ""
	for _, msg := range out {
		if msg.Role == "user" {
			lastUser = msg.Content
		}
	}
	if !strings.Contains(lastUser, "11") {
		t.Fatal("current user turn was dropped")
	}
}
