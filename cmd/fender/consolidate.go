package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/H4fizWasabie/fender/internal/provider"
)

// Consolidation (D43, D32 layer 6): at session end, distill the log with the
// small model into durable facts (one .md per fact) + one episode. Fire in a
// goroutine; failures are silent — the session stays unconsolidated and is
// re-distilled at the next session end.

const minConsolidateExchanges = 4 // each exchange = user + assistant message

const distillPrompt = `You distill a coding-agent conversation into long-term memory.

From the exchanges below, extract:
1. durable facts about the project, its architecture, decisions, or conventions —
   only things worth remembering in a month; skip chit-chat and one-offs.
2. one single-sentence episode summarizing what happened.

Reply with ONLY this JSON:
{"facts": [{"subject": "<what>", "content": "<one sentence>"}], "episode": "<one sentence>"}

Exchanges:
%s`

// consolidateSession distills history into facts + an episode (sync; tests
// call it directly, the REPL fires it in a goroutine).
func consolidateSession(s *sessionFile, llm agentLLM, projectDir string) error {
	if s == nil || s.Consolidated {
		return nil
	}
	if len(s.Messages) < minConsolidateExchanges {
		return nil
	}
	var sb strings.Builder
	for _, m := range s.Messages {
		if m.Role == "assistant" || m.Role == "user" {
			fmt.Fprintf(&sb, "%s: %s\n", m.Role, truncate(m.Content, 500))
		}
	}
	resp, err := llm.Chat(context.Background(), provider.Request{
		Messages: []provider.Message{{Role: "user", Content: fmt.Sprintf(distillPrompt, sb.String())}},
	})
	if err != nil {
		return err
	}
	text := ""
	if len(resp.Choices) > 0 {
		text = resp.Choices[0].Message.Content
	}
	if text == "" {
		return fmt.Errorf("empty distillation response")
	}
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return fmt.Errorf("no JSON in distillation response")
	}
	var distilled struct {
		Facts []struct {
			Subject string `json:"subject"`
			Content string `json:"content"`
		} `json:"facts"`
		Episode string `json:"episode"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &distilled); err != nil {
		return fmt.Errorf("bad distillation JSON: %w", err)
	}
	factsDir := filepath.Join(projectDir, ".fender", "memory", "facts")
	if err := os.MkdirAll(factsDir, 0700); err != nil {
		return err
	}
	written := 0
	for _, f := range distilled.Facts {
		if f.Subject == "" || f.Content == "" || strings.Contains(f.Subject, "<") {
			continue
		}
		slug := slugify(f.Subject)
		path := filepath.Join(factsDir, slug+".md")
		if _, err := os.Stat(path); err == nil {
			continue // already recorded (dedup by subject, D32 layer 6)
		}
		content := fmt.Sprintf("---\ndate: %s\nsubject: %s\n---\n\n%s\n", time.Now().Format("2006-01-02"), f.Subject, f.Content)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			continue
		}
		written++
	}
	if distilled.Episode != "" && !strings.Contains(distilled.Episode, "<") {
		episodesDir := filepath.Join(projectDir, ".fender", "memory", "sessions")
		os.MkdirAll(episodesDir, 0700)
		line := fmt.Sprintf("- %s: %s\n", time.Now().Format("2006-01-02"), distilled.Episode)
		f, err := os.OpenFile(filepath.Join(episodesDir, "episodes.md"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err == nil {
			f.WriteString(line)
			f.Close()
		}
	}
	s.Consolidated = true
	return saveSession(s)
}

// agentLLM is the subset of the provider client the consolidator needs.
type agentLLM interface {
	Chat(ctx context.Context, req provider.Request) (*provider.Response, error)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "fact"
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}
