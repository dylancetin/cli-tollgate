package core

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Update feed and signing key. The key is the same Ed25519 key Sparkle uses
// for the macOS app (SUPublicEDKey in project.yml); CI signs the Windows
// installer with the matching private key.
var (
	UpdateFeedURL    = "https://github.com/dylancetin/cli-tollgate/releases/latest/download/latest-windows.json"
	UpdatePublicKey  = "JEcjqZEN3+v14zLeix+J+jfPd+92NyLxPwLtcjSvk4Y="
	updateCheckEvery = 24 * time.Hour
)

// UpdateFeed is latest-windows.json, attached to every GitHub release.
type UpdateFeed struct {
	Version   string `json:"version"`
	Build     int    `json:"build"`
	URL       string `json:"url"`
	Length    int64  `json:"length"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"` // base64 Ed25519 over the installer bytes
	NotesURL  string `json:"notesUrl"`
}

// UpdateStatus is what the UI shows.
type UpdateStatus struct {
	Supported bool   `json:"supported"`
	Checking  bool   `json:"checking"`
	Available string `json:"available,omitempty"` // version ready to install
	NotesURL  string `json:"notesUrl,omitempty"`
	Message   string `json:"message,omitempty"`
}

// Updater checks the feed, downloads and verifies installers, and hands over
// to them. Installing is platform specific (see InstallFunc).
type Updater struct {
	app *App

	mu        sync.Mutex
	checking  bool
	ready     *UpdateFeed
	readyPath string
	message   string
	timer     *time.Timer

	// InstallFunc runs a verified installer. relaunch asks it to start the
	// app again afterwards. Nil means updates are unsupported (dev builds).
	InstallFunc func(installer string, relaunch bool) error
}

func NewUpdater(a *App) *Updater { return &Updater{app: a} }

func (u *Updater) supported() bool { return u.InstallFunc != nil && u.app.Build > 0 }

func (u *Updater) Status() UpdateStatus {
	u.mu.Lock()
	defer u.mu.Unlock()
	st := UpdateStatus{Supported: u.supported(), Checking: u.checking, Message: u.message}
	if u.ready != nil {
		st.Available, st.NotesURL = u.ready.Version, u.ready.NotesURL
	}
	return st
}

// StartSchedule (re)arms the daily background check.
func (u *Updater) StartSchedule() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.timer != nil {
		u.timer.Stop()
		u.timer = nil
	}
	if !u.supported() || !u.app.Settings().AutoCheckUpdates {
		return
	}
	var tick func()
	tick = func() {
		u.Check(false)
		u.mu.Lock()
		u.timer = time.AfterFunc(updateCheckEvery, tick)
		u.mu.Unlock()
	}
	u.timer = time.AfterFunc(30*time.Second, tick)
}

// Check fetches the feed and downloads a newer, correctly signed installer.
// interactive reports "up to date" and errors to the user.
func (u *Updater) Check(interactive bool) {
	if !u.supported() {
		if interactive {
			u.setMessage("Updates are only available in released builds.")
		}
		return
	}
	u.mu.Lock()
	if u.checking {
		u.mu.Unlock()
		return
	}
	u.checking, u.message = true, ""
	u.mu.Unlock()
	u.app.notify()

	msg, err := u.check()
	u.mu.Lock()
	u.checking = false
	switch {
	case err != nil && interactive:
		u.message = "Update check failed: " + err.Error()
	case err == nil && (interactive || u.ready != nil):
		u.message = msg
	}
	u.mu.Unlock()
	u.app.notify()
}

func (u *Updater) setMessage(m string) {
	u.mu.Lock()
	u.message = m
	u.mu.Unlock()
	u.app.notify()
}

func (u *Updater) check() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	feed, err := fetchFeed(ctx, UpdateFeedURL)
	if err != nil {
		return "", err
	}
	if feed.Build <= u.app.Build {
		return fmt.Sprintf("CLI Tollgate %s is the latest version.", u.app.Version), nil
	}
	u.mu.Lock()
	already := u.ready != nil && u.ready.Build == feed.Build
	u.mu.Unlock()
	if already {
		return fmt.Sprintf("Version %s is ready to install.", feed.Version), nil
	}

	path, err := downloadVerified(feed, u.app.Paths.Updates())
	if err != nil {
		return "", err
	}
	u.mu.Lock()
	u.ready, u.readyPath = feed, path
	u.mu.Unlock()
	return fmt.Sprintf("Version %s is ready to install.", feed.Version), nil
}

// InstallNow runs the downloaded installer and quits; the installer restarts the app.
func (u *Updater) InstallNow() error {
	u.mu.Lock()
	path := u.readyPath
	u.mu.Unlock()
	if path == "" {
		return errors.New("no update has been downloaded")
	}
	u.app.Engine.StopAndWait(4 * time.Second)
	if err := u.InstallFunc(path, true); err != nil {
		u.app.Engine.Start()
		return err
	}
	u.mu.Lock()
	u.ready, u.readyPath = nil, "" // don't run it again on quit
	u.mu.Unlock()
	u.app.Platform.Quit()
	return nil
}

// InstallOnQuit installs a downloaded update silently without relaunching.
func (u *Updater) InstallOnQuit() {
	u.mu.Lock()
	path := u.readyPath
	u.mu.Unlock()
	if path != "" && u.InstallFunc != nil {
		_ = u.InstallFunc(path, false)
	}
}

var updateHTTP = &http.Client{Timeout: 10 * time.Minute}

func fetchFeed(ctx context.Context, feedURL string) (*UpdateFeed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := updateHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("no Windows release has been published yet")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update feed returned %s", resp.Status)
	}
	var feed UpdateFeed
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&feed); err != nil {
		return nil, fmt.Errorf("invalid update feed: %w", err)
	}
	if feed.URL == "" || feed.Signature == "" || feed.Build <= 0 {
		return nil, errors.New("invalid update feed: missing url, signature or build")
	}
	if !strings.HasPrefix(feed.URL, "https://") {
		return nil, errors.New("invalid update feed: download is not https")
	}
	return &feed, nil
}

func downloadVerified(feed *UpdateFeed, dir string) (string, error) {
	resp, err := updateHTTP.Get(feed.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned %s", resp.Status)
	}
	limit := int64(512 << 20)
	if feed.Length > 0 {
		limit = feed.Length + 1
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return "", err
	}
	if err := VerifyUpdate(data, feed); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("CLI-Tollgate-Setup-%d.exe", feed.Build))
	if err := os.WriteFile(path, data, 0o700); err != nil {
		return "", err
	}
	return path, nil
}

// VerifyUpdate checks length, SHA-256 and the Ed25519 signature of an installer.
func VerifyUpdate(data []byte, feed *UpdateFeed) error {
	if feed.Length > 0 && int64(len(data)) != feed.Length {
		return fmt.Errorf("download is %d bytes, expected %d", len(data), feed.Length)
	}
	if feed.SHA256 != "" {
		sum := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(sum[:]), feed.SHA256) {
			return errors.New("download checksum does not match")
		}
	}
	pub, err := base64.StdEncoding.DecodeString(UpdatePublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errors.New("bad embedded update key")
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(feed.Signature))
	if err != nil || !ed25519.Verify(ed25519.PublicKey(pub), data, sig) {
		return errors.New("update signature is not valid; refusing to install")
	}
	return nil
}
