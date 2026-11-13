//go:build windows

package winshell

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procOpenClipboard             = user32.NewProc("OpenClipboard")
	procCloseClipboard            = user32.NewProc("CloseClipboard")
	procEmptyClipboard            = user32.NewProc("EmptyClipboard")
	procSetClipboardData          = user32.NewProc("SetClipboardData")
	procMessageBoxW               = user32.NewProc("MessageBoxW")
	procSetForegroundWindow       = user32.NewProc("SetForegroundWindow")
	procShowWindow                = user32.NewProc("ShowWindow")
	procIsIconic                  = user32.NewProc("IsIconic")
	procSetWindowPos              = user32.NewProc("SetWindowPos")
	procGetDpiForWindow           = user32.NewProc("GetDpiForWindow")
	procSystemParametersInfoW     = user32.NewProc("SystemParametersInfoW")
	procSetProcessDpiAwarenessCtx = user32.NewProc("SetProcessDpiAwarenessContext")
	procAllowSetForegroundWindow  = user32.NewProc("AllowSetForegroundWindow")

	procGlobalAlloc   = kernel32.NewProc("GlobalAlloc")
	procGlobalFree    = kernel32.NewProc("GlobalFree")
	procGlobalLock    = kernel32.NewProc("GlobalLock")
	procGlobalUnlock  = kernel32.NewProc("GlobalUnlock")
	procRtlMoveMemory = kernel32.NewProc("RtlMoveMemory")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// copyText puts text on the clipboard as CF_UNICODETEXT.
func copyText(text string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	utf16, err := windows.UTF16FromString(text)
	if err != nil {
		return err
	}
	var opened bool
	for i := 0; i < 10; i++ { // another app may hold the clipboard briefly
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			opened = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !opened {
		return errors.New("the clipboard is in use by another app")
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()

	size := uintptr(len(utf16) * 2)
	h, _, err := procGlobalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return err
	}
	p, _, err := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return err
	}
	procRtlMoveMemory.Call(p, uintptr(unsafe.Pointer(&utf16[0])), size)
	procGlobalUnlock.Call(h)
	if r, _, err := procSetClipboardData.Call(cfUnicodeText, h); r == 0 {
		procGlobalFree.Call(h)
		return err
	}
	return nil // the clipboard owns h now
}

func shellOpen(file, args string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	f, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	var a *uint16
	if args != "" {
		a, _ = windows.UTF16PtrFromString(args)
	}
	return windows.ShellExecute(0, verb, f, a, nil, windows.SW_SHOWNORMAL)
}

func revealFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return shellOpen(dirOf(path), "")
	}
	return shellOpen("explorer.exe", `/select,"`+path+`"`)
}

func dirOf(path string) string {
	if i := strings.LastIndexAny(path, `\/`); i > 0 {
		return path[:i]
	}
	return path
}

func messageBox(title, text string, flags uint32) {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(text)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), uintptr(flags))
}

const (
	mbOK         = 0x0
	mbIconError  = 0x10
	mbIconInfo   = 0x40
	mbSetForegnd = 0x10000
	mbTopMost    = 0x40000
)

// ---- Launch at sign-in (HKCU Run key) ---------------------------------------

const (
	runKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue = "CLI Tollgate"
)

func launchCommand() string {
	exe, _ := os.Executable()
	return `"` + exe + `" --background`
}

func launchAtLogin() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue(runValue)
	if err != nil {
		return false
	}
	exe, _ := os.Executable()
	return strings.Contains(strings.ToLower(v), strings.ToLower(exe))
}

func setLaunchAtLogin(on bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if on {
		return k.SetStringValue(runValue, launchCommand())
	}
	if err := k.DeleteValue(runValue); err != nil && !errors.Is(err, registry.ErrNotExist) && !errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
		return err
	}
	return nil
}

// ---- Installer hand-over -----------------------------------------------------

// runInstaller starts a verified Inno Setup installer silently. It waits for
// this app to exit (it watches the single-instance mutex) before replacing files.
func runInstaller(path string, relaunch bool) error {
	args := []string{"/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/SP-"}
	if relaunch {
		args = append(args, "/RELAUNCH=1")
	}
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
