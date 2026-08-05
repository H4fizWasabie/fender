package main

import (
	"strings"

	"github.com/H4fizWasabie/fender/internal/agent"
)

func terminalStatus(status string) bool {
	switch status {
	case "complete", "blocked", "stalled", "error", "cancelled":
		return true
	default:
		return false
	}
}

func (d *dashState) snapshot() dashSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := dashSnapshot{Busy: d.busy, Workspace: d.workspace, Mode: string(d.mode), Status: "ready", Restored: d.restored, PersistenceError: d.persistErr}
	if d.session != nil {
		s.SessionID, s.Started, s.Status = d.session.ID, d.session.Started, d.session.Status
		s.Terminal = terminalStatus(d.session.Status)
	}
	if named, ok := d.agent.LLM.(interface {
		Name() string
		Model() string
	}); ok {
		s.Provider, s.Model = named.Name(), named.Model()
	}
	for _, m := range d.history {
		if (m.Role == "user" || m.Role == "assistant") && strings.TrimSpace(m.Content) != "" {
			s.Messages = append(s.Messages, dashMessage{Role: m.Role, Content: m.Content})
		}
	}
	s.Events = append([]agent.Event(nil), d.events...)
	if d.pending != nil {
		s.Approval = &dashApprovalView{ID: d.pending.ID, Command: d.pending.Command, Reason: d.pending.Reason}
	}
	return s
}
