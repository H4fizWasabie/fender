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
	cfg, err := decodeConfig(path)
	if err != nil {
		return nil, err
	}
	return build(cfg), nil
}

// LoadDefault tries ./fender.toml, then ~/.fender/fender.toml.
func LoadDefault() (*Registry, error) {
	return LoadSelected("")
}

// LoadSelected loads an explicit config path, or Fender's canonical default.
func LoadSelected(path string) (*Registry, error) {
	selected, err := ConfigPath(path)
	if err != nil {
		return nil, err
	}
	return Load(selected)
}

// LoadConfig returns the selected TOML data for callers that need settings
// outside the provider registry, without duplicating config path resolution.
func LoadConfig(path string) (Config, error) {
	selected, err := ConfigPath(path)
	if err != nil {
		return Config{}, err
	}
	return decodeConfig(selected)
}

// ConfigPath is the single source of truth for explicit/default config lookup.
func ConfigPath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if _, err := os.Stat("fender.toml"); err == nil {
		return "fender.toml", nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".fender", "fender.toml")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no fender.toml found (tried ./fender.toml and ~/.fender/fender.toml)")
}

func decodeConfig(path string) (Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	return cfg, nil
}

func build(cfg Config) *Registry {
	r := &Registry{
		clients:  make(map[string]*Client, len(cfg.Providers)),
		explicit: make(map[string]bool, len(cfg.Providers)),
		fallback: cfg.Fallback,
	}
	for name, p := range cfg.Providers {
		r.clients[name] = New(name, p)
		if p.API == "anthropic" {
			r.clients[name] = NewAnthropic(name, p) // D42: Messages API transport
		}
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
