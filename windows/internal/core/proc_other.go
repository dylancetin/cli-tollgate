//go:build !windows

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func configureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func afterStart(*exec.Cmd) {}

// terminate asks the engine to exit, then kills it after 3 seconds.
func terminate(p *os.Process, done <-chan struct{}) {
	_ = p.Signal(syscall.SIGTERM)
	go func() {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = p.Kill()
		}
	}()
}

// killStaleEngine stops an engine left behind by a crashed app, but only
// when the pid still points at a tollgate-engine binary.
func killStaleEngine(p Paths) {
	defer os.Remove(p.PIDFile())
	b, err := os.ReadFile(p.PIDFile())
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return
	}
	exe, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
	if err != nil || !strings.HasPrefix(filepath.Base(exe), "tollgate-engine") {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	time.Sleep(500 * time.Millisecond)
	if syscall.Kill(pid, 0) == nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}
