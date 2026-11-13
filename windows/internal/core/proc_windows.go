//go:build windows

package core

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

func configureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

var (
	jobOnce   sync.Once
	jobHandle windows.Handle
)

// killOnCloseJob is a job object that kills its processes when the app's
// handle closes, so the engine never outlives the app, even after a crash.
func killOnCloseJob() windows.Handle {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			windows.CloseHandle(h)
			return
		}
		jobHandle = h
	})
	return jobHandle
}

func afterStart(cmd *exec.Cmd) {
	job := killOnCloseJob()
	if job == 0 {
		return
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	_ = windows.AssignProcessToJobObject(job, h)
}

// terminate ends the engine. Windows has no SIGTERM for a windowless child,
// and the engine keeps its state in crash-safe files (SQLite, atomic writes).
func terminate(p *os.Process, _ <-chan struct{}) { _ = p.Kill() }

// killStaleEngine is a no-op: the job object already guarantees an engine
// cannot outlive the app that started it.
func killStaleEngine(Paths) {}
