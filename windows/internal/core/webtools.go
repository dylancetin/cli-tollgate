package core

import (
	"context"
	"encoding/json"
	"strings"
)

// Web access: the engine's `web-tools` config subtree lets connected apps
// search and read the web through this computer. It is read and written as one
// object through /v8/management/config/web-tools.

const webToolsPath = "/v8/management/config/web-tools"

// SearchProviders is the fixed provider set, in the order they are offered.
var SearchProviders = []string{"brave", "tavily", "exa"}

const (
	DomainsDenylist  = "denylist"
	DomainsAllowlist = "allowlist"
)

// WebProvider holds one search provider's settings.
type WebProvider struct {
	APIKey string `json:"api-key"`
}

// WebSearch is the search part of the config.
type WebSearch struct {
	Default   string                 `json:"default"`
	Providers map[string]WebProvider `json:"providers"`
}

// WebDomains restricts which sites may be fetched.
type WebDomains struct {
	Mode string   `json:"mode"`
	List []string `json:"list"`
}

// WebTools mirrors the engine's `web-tools` section.
type WebTools struct {
	Enabled bool       `json:"enabled"`
	Search  WebSearch  `json:"search"`
	Domains WebDomains `json:"domains"`
}

// DefaultWebTools is what an engine without a `web-tools` section behaves like.
func DefaultWebTools() WebTools {
	w := WebTools{Enabled: true}
	w.Normalize()
	return w
}

// UnmarshalJSON starts from the defaults, so missing or null fields keep them.
func (w *WebTools) UnmarshalJSON(data []byte) error {
	type plain WebTools
	p := plain(DefaultWebTools())
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*w = WebTools(p)
	w.Normalize()
	return nil
}

// Normalize fills gaps and cleans values so the object can always be sent whole.
func (w *WebTools) Normalize() {
	providers := map[string]WebProvider{}
	for k, v := range w.Search.Providers {
		providers[k] = v
	}
	for _, p := range SearchProviders {
		providers[p] = WebProvider{APIKey: strings.TrimSpace(providers[p].APIKey)}
	}
	w.Search.Providers = providers
	// The default must be a provider that can search; otherwise pick the first that can.
	if w.Search.Providers[w.Search.Default].APIKey == "" || !isSearchProvider(w.Search.Default) {
		if with := w.ProvidersWithKeys(); len(with) > 0 {
			w.Search.Default = with[0]
		} else if !isSearchProvider(w.Search.Default) {
			w.Search.Default = SearchProviders[0]
		}
	}
	if w.Domains.Mode != DomainsAllowlist {
		w.Domains.Mode = DomainsDenylist
	}
	w.Domains.List = NormalizeDomains(w.Domains.List)
}

func isSearchProvider(name string) bool {
	for _, p := range SearchProviders {
		if p == name {
			return true
		}
	}
	return false
}

// ProvidersWithKeys lists the providers that have an API key, in offer order.
func (w WebTools) ProvidersWithKeys() []string {
	out := []string{}
	for _, p := range SearchProviders {
		if w.Search.Providers[p].APIKey != "" {
			out = append(out, p)
		}
	}
	return out
}

// NormalizeDomains trims, lowercases and drops empty and repeated hostnames.
func NormalizeDomains(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, d := range in {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}

// loadWebTools reads the section; absent (404) means the defaults.
func loadWebTools(ctx context.Context, m Management) (WebTools, error) {
	var w WebTools
	found, err := m.GetOptional(ctx, webToolsPath, &w)
	if err != nil {
		return DefaultWebTools(), err
	}
	if !found {
		return DefaultWebTools(), nil
	}
	return w, nil
}

// saveWebTools sends the whole (normalized) object.
func saveWebTools(ctx context.Context, m Management, w WebTools) error {
	w.Normalize()
	return m.Put(ctx, webToolsPath, w)
}

// WebTools returns the current web access settings.
func (a *App) WebTools() WebTools {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.webTools
}

// SetWebTools stores and saves new web access settings.
func (a *App) SetWebTools(w WebTools) {
	w.Normalize()
	a.mu.Lock()
	a.webTools = w
	a.mu.Unlock()
	a.notify()
	ctx, cancel := ctx15()
	defer cancel()
	if err := saveWebTools(ctx, a.client(), w); err != nil {
		a.report(err)
		a.RefreshSettings()
	}
}
