//go:build windows

package winshell

import (
	"errors"

	"golang.org/x/sys/windows"
)

// Kernel object names. The installer checks the mutex (CheckForMutexes) to
// wait for the app to quit before it replaces files.
const (
	mutexName     = "CLITollgate.Running"
	showEventName = "CLITollgate.Show"
)

// acquireSingleInstance returns false when another copy is already running;
// in that case it asks that copy to show its window.
func acquireSingleInstance() (bool, windows.Handle, error) {
	name, _ := windows.UTF16PtrFromString(mutexName)
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		signalShow()
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	return true, h, nil
}

func signalShow() {
	name, _ := windows.UTF16PtrFromString(showEventName)
	ev, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, name)
	if err != nil {
		return
	}
	defer windows.CloseHandle(ev)
	// Let the running copy bring its window to the front.
	procAllowSetForegroundWindow.Call(uintptr(0xFFFFFFFF)) // ASFW_ANY
	_ = windows.SetEvent(ev)
}

// listenForShow calls show whenever a second launch signals the show event.
func listenForShow(show func()) {
	name, _ := windows.UTF16PtrFromString(showEventName)
	ev, err := windows.CreateEvent(nil, 0, 0, name) // auto-reset
	if err != nil {
		return
	}
	go func() {
		for {
			if ev, _ := windows.WaitForSingleObject(ev, windows.INFINITE); ev != windows.WAIT_OBJECT_0 {
				return
			}
			show()
		}
	}()
}

func closeHandle(h windows.Handle) {
	if h != 0 {
		windows.CloseHandle(h)
	}
}
