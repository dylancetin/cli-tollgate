// CLI Tollgate for Windows: a tray app that runs the bundled CLIProxyAPI
// engine on 127.0.0.1 and manages it through its management API.
package main

import (
	"flag"
	"strconv"
)

// Set at build time with -ldflags "-X main.version=… -X main.build=…".
var (
	version = "0.0.0-dev"
	build   = "0"
)

func main() {
	background := flag.Bool("background", false, "start without opening the window (used at sign-in)")
	uiAddr := flag.String("ui-addr", "", "fixed address for the UI server (development)")
	flag.Parse()
	n, _ := strconv.Atoi(build)
	run(version, n, *background, *uiAddr)
}
