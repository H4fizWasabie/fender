package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// Registry holds configured providers loaded from fender.toml (D7, D25).
type Registry struct {
	clients  map[string]*Client
	order    []string        // sorted provider names
	explicit map[string]bool // provider names with a default_model set
	fallback string          // configured backup provider/key (D50)
}

func Load(path string) (*Registry, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return build(cfg), nil
}

// LoadDefault tries ./fender.toml, then ~/.fender/fender.toml.
func LoadDefault() (*Registry, error) {
	for _, path := range []string{"fender.toml"} {
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}
	home, err := os.UserHomeDir()
	if err == nil {
		path := filepath.Join(home, ".fender", "fender.toml")
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}
	return nil, fmt.Errorf("no fender.toml found (tried ./fender.toml and ~/.fender/fender.toml)")
}

func build(cfg Config) *Registry {
	r := &Registry{
		clients:  make(map[string]*Client, len(cfg.Providers)),
		explicit: make(map[string]bool, len(cfg.Providers)),
		fallback: cfg.Fallback,
	}
	for name, p := range cfg.Providers {
		r.clients[name] = New(name, p)
		r.explicit[name] = p.DefaultModel != ""
		r.order = append(r.order, name)
	}
	sort.Strings(r.order)
	return r
}

// WithFallback wraps primary with the configured backup provider/key. The
// backup is transport resilience, never a subagent. Selecting the backup as
// primary avoids a self-referential chain.
func (r *Registry) WithFallback(primary *Client) (ModelClient, error) {
	if r.fallback == "" || primary.Name() == r.fallback {
		return primary, nil
	}
	backup, ok := r.Client(r.fallback)
	if !ok {
		return nil, fmt.Errorf("fallback provider %q is not configured", r.fallback)
	}
	return NewFallback(primary, backup), nil
}

func (r *Registry) Client(name string) (*Client, bool) {
	c, ok := r.clients[name]
	return c, ok
}

func (r *Registry) Names() []string {
	return r.order
}

// Default returns the first provider (sorted) with a default_model set,
// else the first provider that has any model.
func (r *Registry) Default() (*Client, bool) {
	for _, name := range r.order {
		if name == r.fallback {
			continue
		}
		if r.explicit[name] {
			return r.clients[name], true
		}
	}
	for _, name := range r.order {
		if name == r.fallback {
			continue
		}
		if c := r.clients[name]; c.Model() != "" {
			return c, true
		}
	}
	if c, ok := r.clients[r.fallback]; ok && c.Model() != "" {
		return c, true
	}
	return nil, false
}
