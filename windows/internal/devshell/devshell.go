//go:build !windows

// Package devshell runs the app without a tray on Linux/macOS for development:
// the engine and UI server start, and the UI URL is printed for a browser.
package devshell

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/dylancetin/cli-tollgate/windows/internal/core"
	"github.com/dylancetin/cli-tollgate/windows/internal/ui"
)

type platform struct {
	server *ui.Server
	quit   chan struct{}
}

func opener() string {
	if runtime.GOOS == "darwin" {
		return "open"
	}
	return "xdg-open"
}

func (p *platform) CopyText(text string) error {
	for _, c := range [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"pbcopy"}} {
		if _, err := exec.LookPath(c[0]); err == nil {
			cmd := exec.Command(c[0], c[1:]...)
			cmd.Stdin = strings.NewReader(text)
			if err := cmd.Run(); err == nil {
				return nil
			}
			break
		}
	}
	log.Printf("copy (no usable clipboard): %d chars", len(text))
	return nil
}

func (p *platform) OpenURL(url string) error {
	log.Printf("open %s", url)
	return exec.Command(opener(), url).Start()
}
func (p *platform) OpenFile(path string) error { return exec.Command(opener(), path).Start() }
func (p *platform) RevealFile(path string) error {
	return exec.Command(opener(), filepath.Dir(path)).Start()
}
func (p *platform) LaunchAtLogin() bool { return false }
func (p *platform) SetLaunchAtLogin(bool) error {
	return errors.New("launch at sign-in is only available on Windows")
}
func (p *platform) ShowWindow(tab string) {
	if p.server != nil {
		log.Printf("show window: %s", p.server.URL(tab))
	}
}
func (p *platform) Quit() {
	select {
	case p.quit <- struct{}{}:
	default:
	}
}

// Options configure the development shell.
type Options struct {
	Version string
	Build   int
	UIAddr  string // fixed UI address, e.g. 127.0.0.1:8399
}

func Run(opts Options) error {
	paths, err := core.DefaultPaths()
	if err != nil {
		return err
	}
	plat := &platform{quit: make(chan struct{}, 1)}
	app := core.NewApp(paths, plat, opts.Version, opts.Build)
	server, err := ui.Start(app, opts.UIAddr)
	if err != nil {
		return err
	}
	plat.server = server
	app.Start()
	fmt.Printf("CLI Tollgate (dev)\n  data:   %s\n  engine: %s\n  UI:     %s\n", paths.Data, paths.Engine, server.URL(""))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
	case <-plat.quit:
	}
	server.Close()
	app.Shutdown()
	return nil
}
