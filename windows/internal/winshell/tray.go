//go:build windows

package winshell

import (
	"fmt"
	"strings"
	"sync"

	"fyne.io/systray"

	"github.com/dylancetin/cli-tollgate/windows/internal/core"
	"github.com/dylancetin/cli-tollgate/windows/internal/ico"
)

// tray is the notification-area icon. Left click opens the window; right
// click shows the menu with the most common actions.
type tray struct {
	app  *core.App
	show func(tab string)
	quit func()

	status, usage               *systray.MenuItem
	open, models, settings      *systray.MenuItem
	copyURL, copyKey, copySetup *systray.MenuItem
	snippetItems                map[string]*systray.MenuItem
	accounts                    map[core.Provider]*systray.MenuItem
	cloak, running, restart     *systray.MenuItem
	install, checkUpdates       *systray.MenuItem
	logs, dataFolder, quitItem  *systray.MenuItem

	icons   map[ico.State][]byte
	lastIco ico.State
	dirty   chan struct{}
	once    sync.Once
}

func newTray(app *core.App, show func(string), quit func()) *tray {
	return &tray{app: app, show: show, quit: quit, icons: map[ico.State][]byte{}, lastIco: -1, dirty: make(chan struct{}, 1)}
}

// menuText escapes & (a mnemonic marker in Win32 menus).
func menuText(s string) string { return strings.ReplaceAll(s, "&", "&&") }

func (t *tray) onReady() {
	systray.SetTitle(core.AppName)
	systray.SetOnTapped(func() { t.show("") })

	t.status = systray.AddMenuItem("CLI Tollgate", "")
	t.status.Disable()
	t.usage = systray.AddMenuItem("", "")
	t.usage.Disable()
	t.usage.Hide()
	systray.AddSeparator()

	t.open = systray.AddMenuItem("Open CLI Tollgate", "")
	t.models = systray.AddMenuItem("Models…", "")
	t.settings = systray.AddMenuItem("Settings…", "")
	systray.AddSeparator()

	t.copyURL = systray.AddMenuItem("Copy Base URL", "")
	t.copyKey = systray.AddMenuItem("Copy API Key", "")
	t.copySetup = systray.AddMenuItem("Copy Setup For", "")
	t.snippetItems = map[string]*systray.MenuItem{}
	for _, s := range core.Snippets("", "") {
		item := t.copySetup.AddSubMenuItem(menuText(s.Title), "")
		t.snippetItems[s.ID] = item
		id := s.ID
		go t.onClick(item, func() { t.copySnippet(id) })
	}
	systray.AddSeparator()

	t.accounts = map[core.Provider]*systray.MenuItem{}
	for _, p := range core.Providers {
		item := systray.AddMenuItem(p.Title(), "")
		t.accounts[p] = item
		provider := p
		go t.onClick(item, func() { t.accountClicked(provider) })
	}
	systray.AddSeparator()

	t.cloak = systray.AddMenuItemCheckbox("Cloak Requests", "Disguise each request to match its model", true)
	t.running = systray.AddMenuItemCheckbox("Proxy Running", "", false)
	t.restart = systray.AddMenuItem("Restart Engine", "")
	systray.AddSeparator()

	t.install = systray.AddMenuItem("Install Update and Restart", "")
	t.install.Hide()
	t.checkUpdates = systray.AddMenuItem("Check for Updates…", "")
	t.logs = systray.AddMenuItem("Open Logs", "")
	t.dataFolder = systray.AddMenuItem("Show Data Folder", "")
	systray.AddSeparator()
	t.quitItem = systray.AddMenuItem("Quit CLI Tollgate", "")

	go t.onClick(t.open, func() { t.show("overview") })
	go t.onClick(t.models, func() { t.show("models") })
	go t.onClick(t.settings, func() { t.show("settings") })
	go t.onClick(t.copyURL, func() { t.app.Copy(t.app.Snapshot().BaseURL) })
	go t.onClick(t.copyKey, func() {
		if key := t.app.Snapshot().APIKey; key != "" {
			t.app.Copy(key)
		}
	})
	go t.onClick(t.cloak, func() { t.app.SetCloaking(!t.app.CloakingEnabled()) })
	go t.onClick(t.running, func() {
		st, _, _ := t.app.Engine.State()
		t.app.SetEngineEnabled(st == core.EngineStopped || st == core.EngineFailed)
	})
	go t.onClick(t.restart, func() { t.app.Engine.Restart() })
	go t.onClick(t.install, func() {
		if err := t.app.Updater.InstallNow(); err != nil {
			t.app.ReportError(err.Error())
			t.show("overview")
		}
	})
	go t.onClick(t.checkUpdates, func() {
		t.show("settings")
		t.app.Updater.Check(true)
	})
	go t.onClick(t.logs, func() { _ = shellOpen(t.app.Paths.EngineLog(), "") })
	go t.onClick(t.dataFolder, func() { _ = revealFile(t.app.Paths.Config()) })
	go t.onClick(t.quitItem, t.quit)

	t.app.OnChange(t.markDirty)
	go t.updateLoop()
	t.markDirty()
}

func (t *tray) onClick(item *systray.MenuItem, f func()) {
	for range item.ClickedCh {
		f()
	}
}

func (t *tray) copySnippet(id string) {
	s := t.app.Snapshot()
	for _, sn := range s.Snippets {
		if sn.ID == id {
			t.app.Copy(sn.Text)
			return
		}
	}
}

func (t *tray) accountClicked(p core.Provider) {
	s := t.app.Snapshot()
	for _, a := range s.Accounts {
		if a.Provider == string(p) && !a.SignedIn && !s.LoginActive && s.Engine == string(core.EngineRunning) {
			go t.app.Login(p)
			t.show("overview")
			return
		}
	}
	t.show("overview")
}

func (t *tray) markDirty() {
	select {
	case t.dirty <- struct{}{}:
	default:
	}
}

func (t *tray) updateLoop() {
	for range t.dirty {
		t.update()
	}
}

func (t *tray) update() {
	s := t.app.Snapshot()
	upd := t.app.Updater.Status()

	var statusText string
	switch core.EngineState(s.Engine) {
	case core.EngineRunning:
		statusText = "Running on " + strings.TrimPrefix(s.BaseURL, "http://")
	case core.EngineStarting:
		statusText = "Starting…"
	case core.EngineStopped:
		statusText = "Stopped"
	default:
		statusText = "Engine failed"
	}
	t.status.SetTitle(menuText(statusText))
	systray.SetTooltip(truncate("CLI Tollgate — "+statusText, 120))

	if s.Usage != nil {
		t.usage.SetTitle(fmt.Sprintf("Today: %d requests · %s tokens", s.Usage.Requests, formatTokens(s.Usage.Tokens.Total)))
		t.usage.Show()
	} else {
		t.usage.Hide()
	}

	running := s.Engine == string(core.EngineRunning)
	signedIn := 0
	for _, a := range s.Accounts {
		item := t.accounts[core.Provider(a.Provider)]
		switch {
		case a.LoggingIn:
			item.SetTitle(menuText(a.Title + ": signing in…"))
		case a.SignedIn:
			signedIn++
			item.SetTitle(menuText(a.Title + ": " + a.Detail))
		default:
			item.SetTitle(menuText(a.Title + ": Sign In…"))
		}
		if running {
			item.Enable()
		} else {
			item.Disable()
		}
	}

	setChecked(t.cloak, s.Cloaking.Claude && s.Cloaking.Codex)
	setEnabled(t.cloak, running)
	setChecked(t.running, running || s.Engine == string(core.EngineStarting))
	hasKey := s.APIKey != ""
	setEnabled(t.copyKey, hasKey)
	setEnabled(t.copySetup, hasKey)

	if upd.Available != "" {
		t.install.SetTitle(menuText("Install Update " + upd.Available + " and Restart"))
		t.install.Show()
	} else {
		t.install.Hide()
	}
	setEnabled(t.checkUpdates, upd.Supported && !upd.Checking)

	state := ico.Off
	switch core.EngineState(s.Engine) {
	case core.EngineRunning:
		state = ico.Ready
		if signedIn == 0 {
			state = ico.NeedsLogin
		}
	case core.EngineStarting:
		state = ico.Busy
	case core.EngineFailed:
		state = ico.Problem
	}
	if state != t.lastIco {
		data, ok := t.icons[state]
		if !ok {
			data = ico.TrayIcon(state)
			t.icons[state] = data
		}
		systray.SetIcon(data)
		t.lastIco = state
	}
}

func setChecked(item *systray.MenuItem, on bool) {
	if on != item.Checked() {
		if on {
			item.Check()
		} else {
			item.Uncheck()
		}
	}
}

func setEnabled(item *systray.MenuItem, on bool) {
	if on == item.Disabled() {
		if on {
			item.Enable()
		} else {
			item.Disable()
		}
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func formatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}
