package main

import (
	"fmt"
	"time"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/provider"
)

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
