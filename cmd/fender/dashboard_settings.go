package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/H4fizWasabie/fender/internal/provider"
)

// Settings API (ticket 18): manage providers (add/edit keys), guardrail
// mode, and the fallback provider from the dashboard. Keys are masked in
// GET responses; a blank api_key + matching key_hint preserves the stored
// key on POST.

type settingsView struct {
	Mode      string               `json:"mode"`
	Fallback  string               `json:"fallback"`
	Providers []settingsProvider   `json:"providers"`
}

type settingsProvider struct {
	Name         string   `json:"name"`
	BaseURL      string   `json:"base_url"`
	Path         string   `json:"path"` // API path prefix (default "/v1"; OpenRouter "/api/v1")
	APIKey       string   `json:"api_key"`   // masked in GET; "" = keep on POST
	KeyHint      string   `json:"key_hint"`  // last 4 chars, for display
	Models       []string `json:"models"`
	DefaultModel string   `json:"default_model"`
	Thinking     bool     `json:"thinking"`
}

// resolveConfigPath returns the config file the dashboard manages.
func (d *dashState) resolveConfigPath() string {
	if d.cfgPath != "" {
		return d.cfgPath
	}
	if _, err := os.Stat("fender.toml"); err == nil {
		return "fender.toml"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".fender", "fender.toml")
}

func (d *dashState) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		d.getSettings(w, r)
	case http.MethodPost:
		d.postSettings(w, r)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, fmt.Errorf("GET or POST only"))
	}
}

func (d *dashState) getSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := d.loadConfig()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	view := settingsView{Mode: cfg.Mode, Fallback: cfg.Fallback}
	for name, p := range cfg.Providers {
		view.Providers = append(view.Providers, settingsProvider{
			Name: name, BaseURL: p.BaseURL, Path: p.Path,
			APIKey:       maskKey(p.APIKey),
			KeyHint:      keyHint(p.APIKey),
			Models:       p.Models,
			DefaultModel: p.DefaultModel,
			Thinking:     p.ModelConfigs[p.DefaultModel].Thinking,
		})
	}
	writeJSON(w, http.StatusOK, view)
}

func (d *dashState) postSettings(w http.ResponseWriter, r *http.Request) {
	var view settingsView
	if err := decodeJSON(w, r, 1<<20, &view); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	existing, err := d.loadConfig()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	cfg := provider.Config{Mode: view.Mode, Fallback: view.Fallback, Providers: map[string]provider.Provider{}}
	for _, sp := range view.Providers {
		if sp.Name == "" {
			continue
		}
		if cfg.Fallback == sp.Name {
			cfg.Fallback = "" // never leave a dangling fallback
		}
		key := sp.APIKey
		if key == "" || key == maskKey(existing.Providers[sp.Name].APIKey) {
			key = existing.Providers[sp.Name].APIKey // preserve stored key
		}
		p := provider.Provider{
			BaseURL: sp.BaseURL, APIKey: key,
			Path: sp.Path, // preserved — dropping it re-breaks /api/v1 providers (openrouter)
			Models: sp.Models, DefaultModel: sp.DefaultModel,
		}
		if sp.Thinking && sp.DefaultModel != "" {
			p.ModelConfigs = map[string]provider.ModelConfig{
				sp.DefaultModel: {Thinking: true, ThinkingLevels: map[string]string{
					"low": "low", "medium": "medium", "high": "high",
				}},
			}
		}
		cfg.Providers[sp.Name] = p
	}
	path := d.resolveConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	// live-rebuild the agent so mode/providers apply immediately
	if err := d.rebuild(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, fmt.Errorf("config saved but rebuild failed: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "saved", "path": path})
}

func (d *dashState) loadConfig() (provider.Config, error) {
	var cfg provider.Config
	if _, err := toml.DecodeFile(d.resolveConfigPath(), &cfg); err != nil {
		return cfg, err
	}
	if cfg.Providers == nil {
		cfg.Providers = map[string]provider.Provider{}
	}
	return cfg, nil
}

func maskKey(k string) string {
	if len(k) <= 4 {
		return ""
	}
	return "sk-…" + k[len(k)-4:]
}

func keyHint(k string) string {
	if len(k) <= 4 {
		return ""
	}
	return strings.TrimPrefix(k, "sk-")
}
