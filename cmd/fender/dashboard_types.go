package main

import (
	"sync"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/guardrail"
	"github.com/H4fizWasabie/fender/internal/provider"
)

type dashApproval struct {
	ID       string
	Command  string
	Reason   string
	Response chan bool
}

// dashState owns one browser session. Its private methods form the seam used
// by HTTP handlers and tests; persistence remains authoritative across reloads.
type dashState struct {
	mu          sync.Mutex
	cfgPath     string
	agent       *agent.Agent
	session     *sessionFile
	history     []provider.Message
	events      []agent.Event
	subs        map[chan agent.Event]struct{}
	busy        bool
	mode        guardrail.Mode
	workspace   string
	pending     *dashApproval
	approvalSeq uint64
	restored    int
	persistErr  string
}

type dashMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type dashSessionSummary struct {
	ID           string `json:"id"`
	Started      string `json:"started"`
	Updated      string `json:"updated,omitempty"`
	Title        string `json:"title"`
	Status       string `json:"status,omitempty"`
	MessageCount int    `json:"messageCount"`
}

type dashApprovalView struct {
	ID      string `json:"id"`
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

type dashSnapshot struct {
	SessionID        string            `json:"sessionId"`
	Started          string            `json:"started"`
	Status           string            `json:"status"`
	Meter            map[string]any    `json:"meter,omitempty"` // D56: CH rate, usage %, window
	Terminal         bool              `json:"terminal"`
	Busy             bool              `json:"busy"`
	Workspace        string            `json:"workspace"`
	Mode             string            `json:"mode"`
	Provider         string            `json:"provider"`
	Model            string            `json:"model"`
	Messages         []dashMessage     `json:"messages"`
	Events           []agent.Event     `json:"events,omitempty"`
	Approval         *dashApprovalView `json:"approval,omitempty"`
	Restored         int               `json:"restoredCount,omitempty"`
	PersistenceError string            `json:"persistenceError,omitempty"`
}
