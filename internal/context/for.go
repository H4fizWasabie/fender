package context

import (
	"fmt"

	"github.com/H4fizWasabie/fender/internal/provider"
)

// For applies the budget arithmetic (mino ContextFor, D31): compacts
// oversized user messages to HEAD/TAIL, notes oversized history messages,
// truncates history to the last MaxHistoryTurns turns with a compaction
// marker, appends the artifact catalog, and guarantees
// len(system) + Σ message content ≤ ContextChars. The current user turn is
// never dropped (mino's rule).
func (m *Manager) For(system string, msgs []provider.Message) []provider.Message {
	available := m.chars() - len(system) - len(m.Catalog())
	preview := PreviewLimit
	if p := available / 4; p < preview {
		preview = p
	}
	if preview < 512 {
		preview = 512
	}

	// Compress (D31): compact oversized user messages (HEAD/TAIL), note
	// oversized history messages instead of dropping them silently.
	history := make([]provider.Message, 0, len(msgs))
	for _, msg := range msgs {
		if len(msg.Content) <= preview {
			history = append(history, msg)
			continue
		}
		if msg.Role == "user" {
			compacted, _ := m.CompactInput(msg.Content, preview)
			history = append(history, provider.Message{Role: "user", Content: compacted})
			continue
		}
		history = append(history, provider.Message{Role: msg.Role,
			Content: fmt.Sprintf("[Large previous %s message (%d chars) is retained in the session artifact catalog.]", msg.Role, len(msg.Content))})
	}

	// Turns truncation with a compaction marker (mino ContextMessages).
	keep := m.turns() * 2
	if len(history) > keep {
		marker := fmt.Sprintf("[%d earlier turns compacted; full content is not retained.]", (len(history)-keep)/2)
		history = append([]provider.Message{{Role: "assistant", Content: marker}}, history[len(history)-keep:]...)
	}

	// Hard budget bound: drop oldest pairs until the budget fits. The last
	// pair (marker + current user turn) is never dropped.
	budget := m.chars() - len(system) - len(m.Catalog())
	total := 0
	for _, msg := range history {
		total += len(msg.Content)
	}
	for total > budget && len(history) > 2 {
		total -= len(history[0].Content) + len(history[1].Content)
		history = history[2:]
	}

	// Artifact catalog rides in context (D31: select).
	if cat := m.Catalog(); cat != "" {
		history = append(history, provider.Message{Role: "assistant", Content: cat})
	}
	return history
}
