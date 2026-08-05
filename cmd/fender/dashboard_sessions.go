package main

import (
	"fmt"
	"strings"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/provider"
)

func (d *dashState) startNew() error {
	return d.installSession(freshDashboardSession(), 0)
}

func (d *dashState) resume(id string) error {
	s, err := loadSession(id)
	if err != nil {
		return err
	}
	return d.installSession(s, len(s.Messages))
}

func (d *dashState) installSession(s *sessionFile, restored int) error {
	d.mu.Lock()
	if d.busy {
		d.mu.Unlock()
		return fmt.Errorf("busy: finish or stop the current turn first")
	}
	d.mu.Unlock()

	a, err := buildAgent(d.cfgPath, nil, d.requestApproval)
	if err != nil {
		return err
	}
	a.Observer = d.broadcast

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.busy {
		return fmt.Errorf("busy: finish or stop the current turn first")
	}
	d.agent = a
	d.session = s
	d.history = append([]provider.Message(nil), s.Messages...)
	d.events = append([]agent.Event(nil), s.Events...)
	d.pending = nil
	d.restored = restored
	d.persistErr = ""
	return nil
}

func dashboardSessionSummaries() ([]dashSessionSummary, error) {
	sessions, err := listSessions()
	if err != nil {
		return nil, err
	}
	out := make([]dashSessionSummary, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, dashSessionSummary{ID: s.ID, Started: s.Started, Updated: s.Updated, Title: sessionTitle(s.Messages), Status: s.Status, MessageCount: len(s.Messages)})
	}
	return out, nil
}

func sessionTitle(messages []provider.Message) string {
	for _, m := range messages {
		if m.Role != "user" {
			continue
		}
		title := strings.Join(strings.Fields(m.Content), " ")
		if runes := []rune(title); len(runes) > 72 {
			title = string(runes[:69]) + "…"
		}
		if title != "" {
			return title
		}
	}
	return "Untitled session"
}
