//go:build !windows

package main

import (
	"log"

	"github.com/dylancetin/cli-tollgate/windows/internal/devshell"
)

func run(version string, build int, _ bool, uiAddr string) {
	if err := devshell.Run(devshell.Options{Version: version, Build: build, UIAddr: uiAddr}); err != nil {
		log.Fatal(err)
	}
}
