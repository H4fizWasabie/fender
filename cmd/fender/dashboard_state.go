package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/H4fizWasabie/fender/internal/agent"
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
	a, err := d.configuredAgent()
	if err != nil {
		return err
	}
	d.agent = a
	return nil
}

func (d *dashState) configuredAgent() (*agent.Agent, error) {
	a, err := buildAgent(d.cfgPath, nil, d.requestApproval)
	if err != nil {
		return nil, err
	}
	a.Observer = d.broadcast
	return a, nil
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
