package core

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dylancetin/cli-tollgate/windows/internal/i18n"
)

// EngineState is the lifecycle of the bundled engine process.
type EngineState string

const (
	EngineStopped  EngineState = "stopped"
	EngineStarting EngineState = "starting"
	EngineRunning  EngineState = "running"
	EngineFailed   EngineState = "failed"
)

// Engine runs the bundled CLIProxyAPI binary as a child process bound to
// 127.0.0.1, restarts it after crashes and reports its state.
type Engine struct {
	paths    Paths
	password string // random per launch, passed through MANAGEMENT_PASSWORD

	mu          sync.Mutex
	state       EngineState
	message     string
	port        int
	proc        *engineProc
	intentional bool
	wantRunning bool // latest user intent, for a Start that arrives while stopping
	crashes     []time.Time
	generation  int // bumps on every launch/stop to cancel stale health checks

	OnChange func(EngineState, string)
}

type engineProc struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func NewEngine(p Paths) *Engine {
	return &Engine{paths: p, password: randomHex(32), state: EngineStopped, port: DefaultPort}
}

func (e *Engine) Client() Management {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Management{Port: e.port, Password: e.password}
}

func (e *Engine) State() (EngineState, string, int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state, e.message, e.port
}

func (e *Engine) setState(s EngineState, msg string) {
	if e.state == s && e.message == msg {
		return
	}
	e.state, e.message = s, msg
	if cb := e.OnChange; cb != nil {
		go cb(s, msg)
	}
}

func (e *Engine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.wantRunning = true
	if e.proc != nil {
		if e.intentional {
			// Still shutting down: start again once the old process is gone.
			e.setState(EngineStarting, "")
			done := e.proc.done
			go func() {
				<-done
				e.mu.Lock()
				want := e.wantRunning
				e.mu.Unlock()
				if want {
					e.Start()
				}
			}()
		}
		return
	}
	e.intentional = false
	e.setState(EngineStarting, "")
	if err := e.paths.EnsureDirectories(); err != nil {
		e.setState(EngineFailed, err.Error())
		return
	}
	if err := BootstrapConfig(e.paths); err != nil {
		e.setState(EngineFailed, err.Error())
		return
	}
	e.port = ReadPort(e.paths)
	killStaleEngine(e.paths)
	if err := e.launchLocked(); err != nil {
		e.setState(EngineFailed, err.Error())
	}
}

// Stop ends the engine. It returns a channel closed once the process exited.
func (e *Engine) Stop() <-chan struct{} {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.intentional = true
	e.wantRunning = false
	e.generation++
	e.setState(EngineStopped, "")
	if e.proc == nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	terminate(e.proc.cmd.Process, e.proc.done)
	return e.proc.done
}

// Restart stops the engine, waits for the port to be released and starts it again.
func (e *Engine) Restart() {
	done := e.Stop()
	go func() {
		select {
		case <-done:
		case <-time.After(6 * time.Second):
		}
		e.mu.Lock()
		e.crashes = nil
		e.mu.Unlock()
		e.Start()
	}()
}

// StopAndWait is used on quit: stop and block until the process is gone.
func (e *Engine) StopAndWait(timeout time.Duration) {
	select {
	case <-e.Stop():
	case <-time.After(timeout):
	}
	_ = os.Remove(e.paths.PIDFile())
}

func (e *Engine) launchLocked() error {
	if _, err := os.Stat(e.paths.Engine); err != nil {
		return fmt.Errorf(i18n.T("the proxy engine is missing (%s)"), e.paths.Engine)
	}
	rotateLog(e.paths)
	log, err := os.OpenFile(e.paths.EngineLog(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}

	cmd := exec.Command(e.paths.Engine, "--config", e.paths.Config())
	cmd.Dir = e.paths.Data
	cmd.Env = append(os.Environ(), "MANAGEMENT_PASSWORD="+e.password)
	cmd.Stdout = log
	cmd.Stderr = log
	configureCommand(cmd)
	if err := cmd.Start(); err != nil {
		log.Close()
		return err
	}
	afterStart(cmd)

	proc := &engineProc{cmd: cmd, done: make(chan struct{})}
	e.proc = proc
	e.generation++
	_ = os.WriteFile(e.paths.PIDFile(), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600)

	go func() {
		err := cmd.Wait()
		log.Close()
		close(proc.done)
		e.handleExit(proc, err)
	}()
	go e.waitUntilHealthy(e.generation, Management{Port: e.port, Password: e.password})
	return nil
}

func (e *Engine) waitUntilHealthy(gen int, client Management) {
	for i := 0; i < 60; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		ok := client.Healthy(ctx)
		cancel()
		e.mu.Lock()
		if e.generation != gen {
			e.mu.Unlock()
			return
		}
		if ok {
			if e.state == EngineStarting {
				e.setState(EngineRunning, "")
			}
			e.mu.Unlock()
			return
		}
		e.mu.Unlock()
		time.Sleep(250 * time.Millisecond)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.generation == gen && e.state == EngineStarting {
		e.setState(EngineFailed, strings.TrimSpace(i18n.F("The engine did not answer on port %d. %s", e.port, LastLogLine(e.paths))))
	}
}

func (e *Engine) handleExit(proc *engineProc, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.proc != proc {
		return
	}
	e.proc = nil
	e.generation++
	_ = os.Remove(e.paths.PIDFile())
	if e.intentional {
		if !e.wantRunning {
			e.setState(EngineStopped, "")
		}
		return
	}
	failure := i18n.F("The engine exited. %s", LastLogLine(e.paths))
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		failure = i18n.F("The engine exited (status %d). %s", exitErr.ExitCode(), LastLogLine(e.paths))
	}
	// Restart on crash, but give up after 3 crashes within a minute.
	now := time.Now()
	recent := e.crashes[:0]
	for _, t := range e.crashes {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	e.crashes = append(recent, now)
	if len(e.crashes) >= 3 {
		e.setState(EngineFailed, strings.TrimSpace(failure))
		return
	}
	e.setState(EngineStarting, "")
	go func() {
		time.Sleep(time.Second)
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.proc != nil || e.intentional {
			return
		}
		if err := e.launchLocked(); err != nil {
			e.setState(EngineFailed, err.Error())
		}
	}()
}

func rotateLog(p Paths) {
	info, err := os.Stat(p.EngineLog())
	if err != nil || info.Size() < 5_000_000 {
		return
	}
	old := p.EngineLog()[:len(p.EngineLog())-len(".log")] + ".old.log"
	_ = os.Remove(old)
	_ = os.Rename(p.EngineLog(), old)
}

// LastLogLine returns the last non-empty line of the engine log.
func LastLogLine(p Paths) string {
	f, err := os.Open(p.EngineLog())
	if err != nil {
		return ""
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() > 4096 {
		_, _ = f.Seek(info.Size()-4096, 0)
	}
	last := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			last = line
		}
	}
	return last
}
