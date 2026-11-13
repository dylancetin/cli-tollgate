//go:build windows

package main

import "github.com/dylancetin/cli-tollgate/windows/internal/winshell"

func run(version string, build int, background bool, uiAddr string) {
	winshell.Run(winshell.Options{Version: version, Build: build, Background: background, UIAddr: uiAddr})
}
