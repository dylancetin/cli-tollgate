//go:build windows

// Package winshell is the Windows desktop shell: single instance, tray icon,
// WebView2 window, clipboard, launch at sign-in and installer hand-over.
package winshell

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"fyne.io/systray"

	"github.com/dylancetin/cli-tollgate/windows/internal/core"
	"github.com/dylancetin/cli-tollgate/windows/internal/ui"
)

type Options struct {
	Version    string
	Build      int
	Background bool // launched at sign-in: don't open the window
	UIAddr     string
}

type platform struct {
	window   *windowManager
	quitOnce sync.Once
}

func (p *platform) CopyText(text string) error   { return copyText(text) }
func (p *platform) OpenURL(url string) error     { return shellOpen(url, "") }
func (p *platform) OpenFile(path string) error   { return shellOpen(path, "") }
func (p *platform) RevealFile(path string) error { return revealFile(path) }
func (p *platform) LaunchAtLogin() bool          { return launchAtLogin() }
func (p *platform) SetLaunchAtLogin(on bool) error {
	return setLaunchAtLogin(on)
}
func (p *platform) ShowWindow(tab string) {
	if p.window != nil {
		p.window.Show(tab)
	}
}
func (p *platform) Quit() {
	p.quitOnce.Do(func() {
		if p.window != nil {
			p.window.Close()
		}
		systray.Quit()
	})
}

// Run starts the app and blocks until it quits.
func Run(opts Options) {
	// Fallback if the manifest's DPI setting is missing: per-monitor v2.
	if procSetProcessDpiAwarenessCtx.Find() == nil {
		procSetProcessDpiAwarenessCtx.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
	}

	first, mutex, err := acquireSingleInstance()
	if err != nil {
		fatal(err)
	}
	if !first {
		return // the running copy shows its window
	}
	defer closeHandle(mutex)

	paths, err := core.DefaultPaths()
	if err != nil {
		fatal(err)
	}
	if err := paths.EnsureDirectories(); err != nil {
		fatal(err)
	}
	setupLogging(paths)
	log.Printf("CLI Tollgate %s (%d) starting", opts.Version, opts.Build)

	plat := &platform{}
	app := core.NewApp(paths, plat, opts.Version, opts.Build)
	app.Updater.InstallFunc = runInstaller

	server, err := ui.Start(app, opts.UIAddr)
	if err != nil {
		fatal(fmt.Errorf("could not start the UI server: %w", err))
	}
	plat.window = &windowManager{dataPath: paths.WebView(), url: server.URL, openURL: plat.OpenURL}

	t := newTray(app, plat.ShowWindow, plat.Quit)
	listenForShow(func() { plat.ShowWindow("overview") })

	systray.Run(func() {
		t.onReady()
		app.Start()
		if !opts.Background {
			plat.ShowWindow("overview")
		}
	}, func() {})

	log.Print("quitting")
	plat.window.Close()
	server.Close()
	app.Shutdown()
}

func setupLogging(p core.Paths) {
	if info, err := os.Stat(p.AppLog()); err == nil && info.Size() > 2_000_000 {
		_ = os.Rename(p.AppLog(), p.AppLog()+".old")
	}
	f, err := os.OpenFile(p.AppLog(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		log.SetOutput(io.Discard)
		return
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func fatal(err error) {
	log.Print(err)
	messageBox("CLI Tollgate", err.Error(), mbOK|mbIconError|mbSetForegnd|mbTopMost)
	os.Exit(1)
}
