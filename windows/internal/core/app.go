package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dylancetin/cli-tollgate/windows/internal/i18n"
)

// Provider is one subscription the app signs in to.
type Provider string

const (
	Codex  Provider = "codex"
	Claude Provider = "claude"
)

var Providers = []Provider{Codex, Claude}

func (p Provider) Title() string {
	if p == Codex {
		return "ChatGPT"
	}
	return "Claude"
}

func (p Provider) Subtitle() string {
	if p == Codex {
		return i18n.T("Plus / Pro subscription via Codex")
	}
	return i18n.T("Pro / Max subscription")
}

func ParseProvider(s string) (Provider, error) {
	switch Provider(s) {
	case Codex, Claude:
		return Provider(s), nil
	}
	return "", fmt.Errorf("unknown provider %q", s)
}

// App is the app model: engine state plus everything read from and written to
// the engine's management API. All methods are safe for concurrent use.
type App struct {
	Paths    Paths
	Engine   *Engine
	Platform Platform
	Updater  *Updater
	Version  string
	Build    int

	settings     *settingsStore
	engineCommit string

	mu         sync.Mutex
	apiKey     string
	accounts   map[Provider]*Credential
	loginOf    Provider
	loginState string
	lastError  string
	catalog    map[Provider][]ModelDefinition
	excluded   map[Provider]map[string]bool
	aliases    map[Provider][]ModelAlias
	claudeOn   bool
	claudeList bool
	codexOn    bool
	usage      *Usage
	webTools   WebTools

	accessRequests []AccessRequest
	grants         []AccessGrant

	changeMu  sync.Mutex
	changed   chan struct{}
	revision  uint64
	listeners []func()
}

func NewApp(p Paths, platform Platform, version string, build int) *App {
	a := &App{
		Paths:        p,
		Engine:       NewEngine(p),
		Platform:     platform,
		Version:      version,
		Build:        build,
		settings:     loadSettings(p.Settings()),
		engineCommit: p.EngineCommit(),
		accounts:     map[Provider]*Credential{},
		catalog:      map[Provider][]ModelDefinition{},
		excluded:     map[Provider]map[string]bool{},
		aliases:      map[Provider][]ModelAlias{},
		claudeOn:     true,
		claudeList:   true,
		codexOn:      true,
		webTools:     DefaultWebTools(),
		changed:      make(chan struct{}),
	}
	a.Updater = NewUpdater(a)
	a.Engine.OnChange = func(s EngineState, _ string) {
		a.notify()
		if s == EngineRunning {
			go a.RefreshAll()
		}
	}
	return a
}

// Start launches the engine and the periodic refreshes.
func (a *App) Start() {
	_ = os.RemoveAll(a.Paths.Updates()) // installers from a previous run are stale
	a.Engine.Start()
	go a.watchAccessRequests()
	go func() {
		t := time.NewTicker(60 * time.Second)
		defer t.Stop()
		for range t.C {
			if st, _, _ := a.Engine.State(); st == EngineRunning {
				a.RefreshAccounts()
				a.RefreshUsage()
			}
		}
	}()
	a.Updater.StartSchedule()
}

// Shutdown stops the engine and, if an update is ready and auto-install is
// on, hands over to the installer (like Sparkle's install-on-quit).
func (a *App) Shutdown() {
	a.Engine.StopAndWait(4 * time.Second)
	if a.settings.Get().AutoInstallUpdates {
		a.Updater.InstallOnQuit()
	}
}

// OnChange registers a callback for any state change (used by the tray).
func (a *App) OnChange(f func()) {
	a.changeMu.Lock()
	a.listeners = append(a.listeners, f)
	a.changeMu.Unlock()
}

func (a *App) notify() {
	a.changeMu.Lock()
	a.revision++
	close(a.changed)
	a.changed = make(chan struct{})
	listeners := append([]func(){}, a.listeners...)
	a.changeMu.Unlock()
	for _, f := range listeners {
		f()
	}
}

// WaitForChange blocks until the state revision differs from since, or the
// context ends. It returns the current revision (long polling for the UI).
func (a *App) WaitForChange(ctx context.Context, since uint64) uint64 {
	a.changeMu.Lock()
	rev, ch := a.revision, a.changed
	a.changeMu.Unlock()
	if rev != since {
		return rev
	}
	select {
	case <-ch:
	case <-ctx.Done():
	}
	a.changeMu.Lock()
	defer a.changeMu.Unlock()
	return a.revision
}

func (a *App) client() Management { return a.Engine.Client() }

func (a *App) running() bool {
	st, _, _ := a.Engine.State()
	return st == EngineRunning
}

func ctx15() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
}

// ---- State snapshot -----------------------------------------------------

// AccountView is one provider row.
type AccountView struct {
	Provider  string `json:"provider"`
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle"`
	SignedIn  bool   `json:"signedIn"`
	Name      string `json:"name,omitempty"`
	Plan      string `json:"plan,omitempty"`
	Detail    string `json:"detail"`
	Warning   bool   `json:"warning"`
	LoggingIn bool   `json:"loggingIn"`
}

// ModelView is one catalog entry with its exposure and alias.
type ModelView struct {
	ID            string `json:"id"`
	DisplayName   string `json:"displayName,omitempty"`
	ContextLength int    `json:"contextLength,omitempty"`
	Exposed       bool   `json:"exposed"`
	Alias         string `json:"alias"`
}

// State is everything the UI and tray render.
type State struct {
	Revision      uint64                 `json:"revision"`
	Engine        string                 `json:"engine"`
	EngineMessage string                 `json:"engineMessage,omitempty"`
	Port          int                    `json:"port"`
	BaseURL       string                 `json:"baseURL"`
	APIKey        string                 `json:"apiKey"`
	Accounts      []AccountView          `json:"accounts"`
	LoginActive   bool                   `json:"loginActive"`
	LastError     string                 `json:"lastError,omitempty"`
	Models        map[string][]ModelView `json:"models"`
	Cloaking      struct {
		Claude          bool `json:"claude"`
		ClaudeModelList bool `json:"claudeModelList"`
		Codex           bool `json:"codex"`
	} `json:"cloaking"`
	WebTools       WebTools        `json:"webTools"`
	Usage          *Usage          `json:"usage,omitempty"`
	AccessRequests []AccessRequest `json:"accessRequests"`
	Grants         []AccessGrant   `json:"grants"`
	LaunchAtLogin  bool            `json:"launchAtLogin"`
	Settings       Settings        `json:"settings"`
	Update         UpdateStatus    `json:"update"`
	Snippets       []Snippet       `json:"snippets"`
	Version        string          `json:"version"`
	Build          int             `json:"build"`
	EngineCommit   string          `json:"engineCommit"`
	DataDir        string          `json:"dataDir"`
}

func (a *App) Snapshot() State {
	engineState, engineMsg, port := a.Engine.State()
	a.changeMu.Lock()
	rev := a.revision
	a.changeMu.Unlock()

	a.mu.Lock()
	defer a.mu.Unlock()
	s := State{
		Revision:       rev,
		Engine:         string(engineState),
		EngineMessage:  engineMsg,
		Port:           port,
		BaseURL:        fmt.Sprintf("http://127.0.0.1:%d", port),
		APIKey:         a.apiKey,
		LoginActive:    a.loginOf != "",
		LastError:      a.lastError,
		Models:         map[string][]ModelView{},
		WebTools:       a.webTools,
		Usage:          a.usage,
		AccessRequests: append([]AccessRequest{}, a.accessRequests...),
		Grants:         append([]AccessGrant{}, a.grants...),
		Settings:       a.settings.Get(),
		Version:        a.Version,
		Build:          a.Build,
		EngineCommit:   a.engineCommit,
		DataDir:        a.Paths.Data,
	}
	s.Cloaking.Claude, s.Cloaking.ClaudeModelList, s.Cloaking.Codex = a.claudeOn, a.claudeList, a.codexOn
	for _, p := range Providers {
		s.Accounts = append(s.Accounts, a.accountViewLocked(p))
		var models []ModelView
		for _, def := range a.catalog[p] {
			models = append(models, ModelView{
				ID: def.ID, DisplayName: def.DisplayName, ContextLength: def.ContextLength,
				Exposed: !a.excluded[p][def.ID], Alias: a.aliasLocked(p, def.ID),
			})
		}
		s.Models[string(p)] = models
	}
	if s.APIKey != "" {
		s.Snippets = Snippets(s.BaseURL, s.APIKey)
	}
	return s
}

// Fill the fields that call out to the platform or updater without holding a.mu.
func (a *App) FullSnapshot() State {
	s := a.Snapshot()
	s.LaunchAtLogin = a.Platform.LaunchAtLogin()
	s.Update = a.Updater.Status()
	return s
}

func (a *App) accountViewLocked(p Provider) AccountView {
	v := AccountView{Provider: string(p), Title: p.Title(), Subtitle: p.Subtitle(), LoggingIn: a.loginOf == p}
	c := a.accounts[p]
	if c != nil {
		v.SignedIn, v.Name, v.Plan = true, c.DisplayName(), c.Plan()
	}
	switch {
	case v.LoggingIn:
		v.Detail = i18n.T("Finish signing in in your browser…")
	case c == nil:
		v.Detail = p.Subtitle()
	case c.Disabled:
		v.Detail, v.Warning = i18n.T("Disabled"), true
	case c.Unavailable:
		v.Detail, v.Warning = c.StatusMessage, true
		if v.Detail == "" {
			v.Detail = i18n.T("Cooling down (rate limited)")
		}
	default:
		v.Detail = c.DisplayName()
	}
	return v
}

// ---- Refresh --------------------------------------------------------------

func (a *App) RefreshAll() {
	a.RefreshAccounts()
	a.RefreshSettings()
	a.RefreshModels()
	a.RefreshUsage()
	a.RefreshGrants()
}

func (a *App) RefreshAccounts() {
	if !a.running() {
		return
	}
	ctx, cancel := ctx15()
	defer cancel()
	var list credentialList
	if err := a.client().Get(ctx, "/v8/management/credentials", nil, &list); err != nil {
		a.report(err)
		return
	}
	next := map[Provider]*Credential{}
	for _, p := range Providers {
		// One account per provider: prefer a healthy one if several exist.
		var first, healthy *Credential
		for i := range list.Files {
			c := &list.Files[i]
			if Provider(strings.ToLower(c.Provider)) != p {
				continue
			}
			if first == nil {
				first = c
			}
			if healthy == nil && c.Healthy() {
				healthy = c
			}
		}
		if healthy != nil {
			next[p] = healthy
		} else if first != nil {
			next[p] = first
		}
	}
	a.mu.Lock()
	a.accounts = next
	a.mu.Unlock()
	a.notify()
}

func (a *App) RefreshSettings() {
	if !a.running() {
		return
	}
	ctx, cancel := ctx15()
	defer cancel()
	c := a.client()
	var keys []string
	found, err := c.GetOptional(ctx, "/v8/management/config/access/api-keys", &keys)
	if err != nil {
		a.report(err)
		return
	}
	if found && len(keys) > 0 {
		a.mu.Lock()
		a.apiKey = keys[0]
		a.mu.Unlock()
	} else if err := a.RegenerateAPIKey(); err != nil { // never leave the proxy open
		a.report(err)
	}
	var claude claudeUpstream
	if ok, err := c.GetOptional(ctx, "/v8/management/config/upstream/claude", &claude); err != nil {
		a.report(err)
	} else if ok {
		a.mu.Lock()
		a.claudeOn = claude.DisableCloakMode == nil || !*claude.DisableCloakMode
		a.claudeList = claude.DisableCloakingModelList == nil || !*claude.DisableCloakingModelList
		a.mu.Unlock()
	}
	var codex codexUpstream
	if ok, err := c.GetOptional(ctx, "/v8/management/config/upstream/codex", &codex); err != nil {
		a.report(err)
	} else if ok {
		a.mu.Lock()
		a.codexOn = codex.DisableCloaking == nil || !*codex.DisableCloaking
		a.mu.Unlock()
	}
	if w, err := loadWebTools(ctx, c); err != nil {
		a.report(err)
	} else {
		a.mu.Lock()
		a.webTools = w
		a.mu.Unlock()
	}
	a.notify()
}

func (a *App) RefreshModels() {
	if !a.running() {
		return
	}
	ctx, cancel := ctx15()
	defer cancel()
	c := a.client()
	catalog := map[Provider][]ModelDefinition{}
	for _, p := range Providers {
		var defs modelDefinitionList
		if err := c.Get(ctx, "/v8/management/routing/model-definitions/"+string(p), nil, &defs); err != nil {
			a.report(err)
			return
		}
		catalog[p] = defs.Models
	}
	var ex map[string][]string
	if _, err := c.GetOptional(ctx, "/v8/management/config/oauth/excluded-models", &ex); err != nil {
		a.report(err)
		return
	}
	var al map[string][]ModelAlias
	if _, err := c.GetOptional(ctx, "/v8/management/config/oauth/model-alias", &al); err != nil {
		a.report(err)
		return
	}
	a.mu.Lock()
	a.catalog = catalog
	for _, p := range Providers {
		set := map[string]bool{}
		for _, id := range ex[string(p)] {
			set[id] = true
		}
		a.excluded[p] = set
		a.aliases[p] = al[string(p)]
	}
	a.mu.Unlock()
	a.notify()
}

func (a *App) RefreshUsage() {
	if !a.running() {
		return
	}
	ctx, cancel := ctx15()
	defer cancel()
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var summary usageSummary
	err := a.client().Get(ctx, "/v8/management/observability/usage/summary",
		url.Values{"from": {strconv.FormatInt(startOfDay.Unix(), 10)}}, &summary)
	a.mu.Lock()
	if err != nil {
		a.usage = nil
	} else {
		a.usage = &summary.Total
	}
	a.mu.Unlock()
	a.notify()
}

// ---- Accounts -------------------------------------------------------------

// Login runs the provider's OAuth flow in the browser. Signing in again
// replaces that provider's account.
func (a *App) Login(p Provider) {
	a.mu.Lock()
	if a.loginOf != "" {
		a.mu.Unlock()
		return
	}
	a.loginOf = p
	a.lastError = ""
	a.mu.Unlock()
	a.notify()

	defer func() {
		a.mu.Lock()
		a.loginOf, a.loginState = "", ""
		a.mu.Unlock()
		a.notify()
	}()
	if err := a.login(p); err != nil {
		a.report(err)
	}
}

func (a *App) login(p Provider) error {
	c := a.client()
	ctx := context.Background()
	before, err := a.credentialNames(p)
	if err != nil {
		return err
	}
	var start authURLResponse
	if err := c.Get(ctx, "/v8/management/oauth/auth-url",
		url.Values{"provider": {string(p)}, "is_webui": {"true"}}, &start); err != nil {
		return err
	}
	a.mu.Lock()
	a.loginState = start.State
	a.mu.Unlock()
	if err := a.Platform.OpenURL(start.URL); err != nil {
		return fmt.Errorf(i18n.T("could not open the browser: %w"), err)
	}

	deadline := time.Now().Add(5 * time.Minute)
	for {
		time.Sleep(1500 * time.Millisecond)
		a.mu.Lock()
		cancelled := a.loginState != start.State
		a.mu.Unlock()
		if cancelled {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New(i18n.T("Login timed out."))
		}
		var status authStatusResponse
		if err := c.Get(ctx, "/v8/management/oauth/status", url.Values{"state": {start.State}}, &status); err != nil {
			return err
		}
		if status.Status == "ok" {
			break
		}
		if status.Status == "error" {
			if status.Error == "" {
				status.Error = i18n.T("Login failed.")
			}
			return errors.New(status.Error)
		}
	}

	// Keep exactly one account per provider: drop the ones that existed before.
	var added map[string]bool
	for i := 0; i < 10; i++ {
		now, err := a.credentialNames(p)
		if err != nil {
			return err
		}
		added = map[string]bool{}
		for n := range now {
			if !before[n] {
				added[n] = true
			}
		}
		if len(added) > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if len(added) > 0 {
		for n := range before {
			if !added[n] {
				dctx, cancel := ctx15()
				_ = c.Delete(dctx, "/v8/management/credentials", url.Values{"name": {n}})
				cancel()
			}
		}
	}
	a.RefreshAccounts()
	a.Platform.ShowWindow("")
	return nil
}

func (a *App) CancelLogin() {
	a.mu.Lock()
	state := a.loginState
	a.loginState = ""
	a.mu.Unlock()
	if state == "" {
		return
	}
	ctx, cancel := ctx15()
	defer cancel()
	_ = a.client().Delete(ctx, "/v8/management/oauth/session", url.Values{"state": {state}})
}

func (a *App) Logout(p Provider) {
	names, err := a.credentialNames(p)
	if err != nil {
		a.report(err)
		return
	}
	ctx, cancel := ctx15()
	defer cancel()
	for n := range names {
		if err := a.client().Delete(ctx, "/v8/management/credentials", url.Values{"name": {n}}); err != nil {
			a.report(err)
			break
		}
	}
	a.RefreshAccounts()
}

func (a *App) credentialNames(p Provider) (map[string]bool, error) {
	ctx, cancel := ctx15()
	defer cancel()
	var list credentialList
	if err := a.client().Get(ctx, "/v8/management/credentials", nil, &list); err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, c := range list.Files {
		if Provider(strings.ToLower(c.Provider)) == p {
			names[c.Name] = true
		}
	}
	return names, nil
}

// ---- Models ---------------------------------------------------------------

func (a *App) aliasLocked(p Provider, model string) string {
	for _, al := range a.aliases[p] {
		if al.Name == model {
			return al.Alias
		}
	}
	return ""
}

func (a *App) SetExposed(p Provider, model string, exposed bool) {
	a.mu.Lock()
	set := map[string]bool{}
	for k := range a.excluded[p] {
		set[k] = true
	}
	if exposed {
		delete(set, model)
	} else {
		set[model] = true
	}
	a.excluded[p] = set
	list := sortedKeys(set)
	a.mu.Unlock()
	a.notify()
	a.save("/v8/management/config/oauth/excluded-models/"+string(p), list)
}

func (a *App) SetAllExposed(p Provider, exposed bool) {
	a.mu.Lock()
	set := map[string]bool{}
	if !exposed {
		for _, def := range a.catalog[p] {
			set[def.ID] = true
		}
	}
	a.excluded[p] = set
	list := sortedKeys(set)
	a.mu.Unlock()
	a.notify()
	a.save("/v8/management/config/oauth/excluded-models/"+string(p), list)
}

// SetAlias exposes model under an extra client-visible name (the original
// stays available). An empty alias removes it.
func (a *App) SetAlias(p Provider, model, alias string) {
	alias = strings.TrimSpace(alias)
	a.mu.Lock()
	list := []ModelAlias{}
	for _, al := range a.aliases[p] {
		if al.Name != model {
			list = append(list, al)
		}
	}
	if alias != "" {
		list = append(list, ModelAlias{Name: model, Alias: alias, Fork: true})
	}
	a.aliases[p] = list
	a.mu.Unlock()
	a.notify()
	a.save("/v8/management/config/oauth/model-alias/"+string(p), list)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---- Cloaking ---------------------------------------------------------------
//
// The engine picks the disguise from the model: Claude models go to the Claude
// executor (Claude Code identity), GPT/Codex models go to the Codex executor
// (Codex CLI identity). The master switch turns both on or off together.

func (a *App) CloakingEnabled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.claudeOn && a.codexOn
}

func (a *App) SetCloaking(on bool) {
	a.mu.Lock()
	a.claudeOn, a.codexOn = on, on
	a.mu.Unlock()
	a.notify()
	a.save("/v8/management/config/upstream/claude/disable-claude-cloak-mode", !on)
	a.save("/v8/management/config/upstream/codex/disable-codex-cloaking", !on)
}

func (a *App) SetClaudeCloaking(on bool) {
	a.mu.Lock()
	a.claudeOn = on
	a.mu.Unlock()
	a.notify()
	a.save("/v8/management/config/upstream/claude/disable-claude-cloak-mode", !on)
}

func (a *App) SetClaudeCloakModelList(on bool) {
	a.mu.Lock()
	a.claudeList = on
	a.mu.Unlock()
	a.notify()
	a.save("/v8/management/config/upstream/claude/disable-cloaking-model-list", !on)
}

func (a *App) SetCodexCloaking(on bool) {
	a.mu.Lock()
	a.codexOn = on
	a.mu.Unlock()
	a.notify()
	a.save("/v8/management/config/upstream/codex/disable-codex-cloaking", !on)
}

// ---- Settings ---------------------------------------------------------------

// RegenerateAPIKey replaces the app's own key (the first one). Keys that were
// given to apps through access requests stay valid.
func (a *App) RegenerateAPIKey() error {
	key := GenerateAPIKey()
	ctx, cancel := ctx15()
	defer cancel()
	var keys []string
	if _, err := a.client().GetOptional(ctx, "/v8/management/config/access/api-keys", &keys); err != nil {
		return err
	}
	next := []string{key}
	if len(keys) > 1 {
		next = append(next, keys[1:]...)
	}
	if err := a.client().Put(ctx, "/v8/management/config/access/api-keys", next); err != nil {
		return err
	}
	a.mu.Lock()
	a.apiKey = key
	a.mu.Unlock()
	a.notify()
	return nil
}

func (a *App) ChangePort(port int) error {
	_, _, current := a.Engine.State()
	if port < 1024 || port > 65535 {
		return errors.New(i18n.T("port must be between 1024 and 65535"))
	}
	if port == current {
		return nil
	}
	ctx, cancel := ctx15()
	defer cancel()
	if err := a.client().Put(ctx, "/v8/management/config/server/port", port); err != nil {
		return err
	}
	a.Engine.Restart()
	return nil
}

func (a *App) SetLaunchAtLogin(on bool) {
	if err := a.Platform.SetLaunchAtLogin(on); err != nil {
		a.report(err)
	}
	a.notify()
}

func (a *App) UpdateSettings(f func(*Settings)) {
	if err := a.settings.Update(f); err != nil {
		a.report(err)
	}
	a.notify()
	a.Updater.StartSchedule()
}

func (a *App) Settings() Settings { return a.settings.Get() }

func (a *App) SetEngineEnabled(on bool) {
	if on {
		a.Engine.Start()
	} else {
		a.Engine.Stop()
	}
}

// ---- Helpers ----------------------------------------------------------------

func (a *App) save(path string, body any) {
	ctx, cancel := ctx15()
	defer cancel()
	if err := a.client().Put(ctx, path, body); err != nil {
		a.report(err)
		a.RefreshSettings()
		a.RefreshModels()
	}
}

func (a *App) report(err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	a.ReportError(err.Error())
}

// ReportError shows a message in the error banner.
func (a *App) ReportError(msg string) {
	a.mu.Lock()
	a.lastError = msg
	a.mu.Unlock()
	a.notify()
}

func (a *App) DismissError() {
	a.mu.Lock()
	a.lastError = ""
	a.mu.Unlock()
	a.notify()
}

// Copy puts text on the clipboard, reporting failures in the banner.
func (a *App) Copy(text string) {
	if err := a.Platform.CopyText(text); err != nil {
		a.report(fmt.Errorf(i18n.T("copy failed: %w"), err))
	}
}
