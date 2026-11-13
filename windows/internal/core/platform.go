package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Platform is what the app needs from the desktop shell.
type Platform interface {
	CopyText(text string) error
	OpenURL(url string) error
	OpenFile(path string) error   // open with the default app (logs)
	RevealFile(path string) error // show in Explorer / file manager
	LaunchAtLogin() bool
	SetLaunchAtLogin(enabled bool) error
	// ShowWindow brings the app window to the front on a tab ("" keeps the current one).
	ShowWindow(tab string)
	Quit()
}

// Settings are the app's own preferences (the engine owns config.yaml).
type Settings struct {
	AutoCheckUpdates   bool `json:"autoCheckUpdates"`
	AutoInstallUpdates bool `json:"autoInstallUpdates"`
}

type settingsStore struct {
	mu   sync.Mutex
	path string
	s    Settings
}

func loadSettings(path string) *settingsStore {
	st := &settingsStore{path: path, s: Settings{AutoCheckUpdates: true, AutoInstallUpdates: true}}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &st.s)
	}
	return st
}

func (st *settingsStore) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s
}

func (st *settingsStore) Update(f func(*Settings)) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	f(&st.s)
	b, err := json.MarshalIndent(st.s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(st.path), 0o700); err != nil {
		return err
	}
	tmp := st.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, st.path)
}
