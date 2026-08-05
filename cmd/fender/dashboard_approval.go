package main

import (
	"context"
	"fmt"

	"github.com/H4fizWasabie/fender/internal/agent"
)

func (d *dashState) requestApproval(ctx context.Context, command, reason string) (bool, error) {
	d.mu.Lock()
	if d.pending != nil {
		d.mu.Unlock()
		return false, fmt.Errorf("another approval is already pending")
	}
	d.approvalSeq++
	p := &dashApproval{ID: fmt.Sprintf("approval-%d", d.approvalSeq), Command: command, Reason: reason, Response: make(chan bool, 1)}
	d.pending = p
	d.mu.Unlock()

	d.broadcast(agent.Event{Kind: "approval", ID: p.ID, Text: command, Detail: reason, Status: "pending"})
	var allowed bool
	var err error
	select {
	case allowed = <-p.Response:
	case <-ctx.Done():
		err = ctx.Err()
	}

	d.mu.Lock()
	if d.pending == p {
		d.pending = nil
	}
	d.mu.Unlock()

	status := "denied"
	if err != nil {
		status = "cancelled"
	} else if allowed {
		status = "approved"
	}
	d.broadcast(agent.Event{Kind: "approval", ID: p.ID, Text: command, Detail: reason, Status: status})
	return allowed, err
}

func (d *dashState) respondApproval(id string, allowed bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pending == nil || d.pending.ID != id {
		return fmt.Errorf("approval %q is no longer pending", id)
	}
	select {
	case d.pending.Response <- allowed:
		return nil
	default:
		return fmt.Errorf("approval %q already answered", id)
	}
}
