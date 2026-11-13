//go:build windows

package winshell

import (
	"encoding/json"
	"log"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	webview2 "github.com/jchv/go-webview2"
	"github.com/jchv/go-webview2/webviewloader"
)

// Logical (96 DPI) window size; scaled for the monitor's DPI.
const (
	windowWidth  = 440
	windowHeight = 720
	minWidth     = 380
	minHeight    = 460
	edgeMargin   = 12
)

// windowManager owns the single WebView2 window. Each window runs its own
// message loop on a dedicated OS thread; closing it destroys it, and the tray
// keeps running. Without the WebView2 runtime, the UI opens in the browser.
type windowManager struct {
	dataPath string
	url      func(tab string) string
	openURL  func(string) error

	mu      sync.Mutex
	wv      webview2.WebView
	hwnd    uintptr
	opening bool
}

func hasWebView2() bool {
	v, err := webviewloader.GetInstalledVersion()
	return err == nil && v != ""
}

func (m *windowManager) Show(tab string) {
	m.mu.Lock()
	if wv := m.wv; wv != nil {
		hwnd := m.hwnd
		m.mu.Unlock()
		wv.Dispatch(func() {
			if tab != "" {
				b, _ := json.Marshal(tab)
				wv.Eval("window.tollgateShow && window.tollgateShow(" + string(b) + ")")
			}
			bringToFront(hwnd)
		})
		return
	}
	if m.opening {
		m.mu.Unlock()
		return
	}
	if !hasWebView2() {
		m.mu.Unlock()
		log.Print("WebView2 runtime not found; opening the UI in the browser")
		_ = m.openURL(m.url(tab))
		return
	}
	m.opening = true
	m.mu.Unlock()
	go m.run(tab)
}

func (m *windowManager) run(tab string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// WebView2 needs an STA COM apartment on the thread that hosts it; the
	// library only initializes the main thread, which the tray owns.
	if r, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded); int32(r) >= 0 {
		defer procCoUninitialize.Call()
	}

	wv := webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath:  m.dataPath,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "CLI Tollgate",
			Width:  windowWidth,
			Height: windowHeight,
			IconId: 1, // RT_GROUP_ICON #1 from winres
		},
	})
	if wv == nil {
		m.mu.Lock()
		m.opening = false
		m.mu.Unlock()
		log.Print("could not create the WebView2 window; opening the UI in the browser")
		_ = m.openURL(m.url(tab))
		return
	}
	hwnd := uintptr(wv.Window())
	scale := placeBottomRight(hwnd)
	wv.SetSize(int(minWidth*scale), int(minHeight*scale), webview2.HintMin)
	wv.Navigate(m.url(tab))
	bringToFront(hwnd)

	m.mu.Lock()
	m.wv, m.hwnd, m.opening = wv, hwnd, false
	m.mu.Unlock()

	wv.Run() // returns when the window is closed (WM_DESTROY)

	m.mu.Lock()
	m.wv, m.hwnd = nil, 0
	m.mu.Unlock()
}

// Close destroys the window if it is open (used on quit).
func (m *windowManager) Close() {
	m.mu.Lock()
	wv := m.wv
	m.mu.Unlock()
	if wv != nil {
		wv.Destroy()
	}
}

var (
	ole32              = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx = ole32.NewProc("CoInitializeEx")
	procCoUninitialize = ole32.NewProc("CoUninitialize")
)

const coinitApartmentThreaded = 0x2

type rect struct{ Left, Top, Right, Bottom int32 }

const (
	spiGetWorkArea = 0x0030
	swRestore      = 9
	swpNoZOrder    = 0x0004
	swpNoActivate  = 0x0010
)

// placeBottomRight puts the window above the notification area, sized for the
// monitor's DPI and the work area height. It returns the DPI scale factor.
func placeBottomRight(hwnd uintptr) float64 {
	scale := 1.0
	if procGetDpiForWindow.Find() == nil {
		if dpi, _, _ := procGetDpiForWindow.Call(hwnd); dpi > 0 {
			scale = float64(dpi) / 96
		}
	}
	var wa rect
	if r, _, _ := procSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&wa)), 0); r == 0 {
		return scale
	}
	margin := int32(edgeMargin * scale)
	w := int32(windowWidth * scale)
	h := int32(windowHeight * scale)
	if maxH := wa.Bottom - wa.Top - 2*margin; h > maxH {
		h = maxH
	}
	x := wa.Right - w - margin
	y := wa.Bottom - h - margin
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h), swpNoZOrder|swpNoActivate)
	return scale
}

func bringToFront(hwnd uintptr) {
	if r, _, _ := procIsIconic.Call(hwnd); r != 0 {
		procShowWindow.Call(hwnd, swRestore)
	}
	procSetForegroundWindow.Call(hwnd)
}
