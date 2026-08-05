package provider

import (
	"context"
	"fmt"
	"log/slog"
)

// ModelClient is the provider surface used by the agent and fallback chain.
// *Client and *FallbackClient both satisfy it.
type ModelClient interface {
	Chat(context.Context, Request) (*Response, error)
	StreamChat(context.Context, Request, func(string), ...func(string)) (*Response, error)
	Name() string
	Model() string
	SetThinking(string) error
	Thinking() string
}

// FallbackClient retries a failed model request once against a separately
// configured provider/key. It is provider resilience, not another agent.
type FallbackClient struct {
	primary ModelClient
	backup  ModelClient
}

func NewFallback(primary, backup ModelClient) *FallbackClient {
	return &FallbackClient{primary: primary, backup: backup}
}

func (c *FallbackClient) Name() string  { return c.primary.Name() }
func (c *FallbackClient) Model() string { return c.primary.Model() }
func (c *FallbackClient) Thinking() string {
	return c.primary.Thinking()
}

// SetThinking keeps the primary authoritative. The backup receives the same
// level when supported; an incompatible backup falls back to its own default.
func (c *FallbackClient) SetThinking(level string) error {
	if err := c.primary.SetThinking(level); err != nil {
		return err
	}
	if err := c.backup.SetThinking(level); err != nil {
		slog.Warn("fallback provider does not support primary thinking level", "provider", c.backup.Name(), "level", level, "error", err)
	}
	return nil
}

func (c *FallbackClient) Chat(ctx context.Context, req Request) (*Response, error) {
	resp, err := c.primary.Chat(ctx, req)
	if err == nil || ctx.Err() != nil {
		return resp, err
	}
	resp, backupErr := c.backup.Chat(ctx, req)
	if backupErr != nil {
		return nil, fmt.Errorf("primary %s failed: %v; fallback %s failed: %w", c.primary.Name(), err, c.backup.Name(), backupErr)
	}
	return resp, nil
}

// StreamChat falls back only before the primary emits output. Retrying after
// a partial stream would duplicate visible text and make the transcript lie.
func (c *FallbackClient) StreamChat(ctx context.Context, req Request, onDelta func(string), onThinking ...func(string)) (*Response, error) {
	started := false
	resp, err := c.primary.StreamChat(ctx, req, func(s string) {
		started = true
		onDelta(s)
	}, func(s string) {
		started = true
		if len(onThinking) > 0 {
			onThinking[0](s)
		}
	})
	if err == nil || started || ctx.Err() != nil {
		return resp, err
	}
	resp, backupErr := c.backup.StreamChat(ctx, req, onDelta, onThinking...)
	if backupErr != nil {
		return nil, fmt.Errorf("primary %s failed: %v; fallback %s failed: %w", c.primary.Name(), err, c.backup.Name(), backupErr)
	}
	return resp, nil
}
