package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/provider"
)

func newDashState(cfgPath string) (*dashState, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("dashboard workspace: %w", err)
	}
	d := &dashState{
		cfgPath:   cfgPath,
		subs:      map[chan agent.Event]struct{}{},
		mode:      configuredMode(cfgPath),
		workspace: filepath.Base(wd),
		session:   freshDashboardSession(),
	}
	if err := d.rebuild(); err != nil {
		return nil, err
	}
	return d, nil
}

func freshDashboardSession() *sessionFile {
	return &sessionFile{
		ID:      newSessionID(),
		Started: time.Now().Format(time.RFC3339),
		Status:  "ready",
	}
}

func (d *dashState) rebuild() error {
	a, err := buildAgent(d.cfgPath, nil, d.requestApproval)
	if err != nil {
		return err
	}
	a.Observer = d.broadcast
	d.agent = a
	return nil
}

// run executes and persists one turn under the stable dashboard session ID.
func (d *dashState) run(ctx context.Context, text string) (string, string, error) {
	before, msgs, a, err := d.beginTurn(text)
	if err != nil {
		return "", "", err
	}
	if err := saveSession(&before); err != nil {
		d.failPersistence(fmt.Errorf("save working session: %w", err))
		return "", "", fmt.Errorf("start turn: session could not be persisted: %w", err)
	}

	res := a.Run(ctx, msgs)
	saved := d.finishTurn(res.Status, res.Reply)
	if err := saveSession(&saved); err != nil {
		d.failPersistence(fmt.Errorf("save completed session: %w", err))
		return res.Status, res.Reply, fmt.Errorf("turn finished but session could not be persisted: %w", err)
	}
	d.clearPersistenceError()
	return res.Status, res.Reply, nil
}

func (d *dashState) beginTurn(text string) (sessionFile, []provider.Message, *agent.Agent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.busy {
		return sessionFile{}, nil, nil, fmt.Errorf("busy: another turn is running")
	}
	d.busy = true
	d.persistErr = ""
	d.history = append(d.history, provider.Message{Role: "user", Content: text})
	d.session.Messages = append([]provider.Message(nil), d.history...)
	d.session.Events = append([]agent.Event(nil), d.events...)
	d.session.Status = "working"
	d.session.Updated = time.Now().Format(time.RFC3339)
	return cloneSession(d.session), append([]provider.Message(nil), d.history...), d.agent, nil
}

func (d *dashState) finishTurn(status, reply string) sessionFile {
	d.mu.Lock()
	defer d.mu.Unlock()
	if reply != "" {
		d.history = append(d.history, provider.Message{Role: "assistant", Content: reply})
	}
	d.session.Messages = append([]provider.Message(nil), d.history...)
	d.session.Events = append([]agent.Event(nil), d.events...)
	d.session.Status = status
	d.session.Updated = time.Now().Format(time.RFC3339)
	d.busy = false
	return cloneSession(d.session)
}

func (d *dashState) failPersistence(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.busy = false
	d.persistErr = err.Error()
}

func (d *dashState) recordPersistenceError(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.persistErr = err.Error()
}

func (d *dashState) clearPersistenceError() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.persistErr = ""
}

func cloneSession(s *sessionFile) sessionFile {
	clone := *s
	clone.Messages = append([]provider.Message(nil), s.Messages...)
	clone.Events = append([]agent.Event(nil), s.Events...)
	return clone
}

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
