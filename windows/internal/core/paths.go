package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// AppName is the product name used for folders, windows and the tray.
const AppName = "CLI Tollgate"

// Paths are the file locations owned by the app. Everything lives in one
// folder (%LOCALAPPDATA%\CLI Tollgate on Windows) so uninstalling is one folder.
type Paths struct {
	Data   string // config.yaml, auths/, logs/, settings.json
	Engine string // the bundled tollgate-engine binary
}

// DefaultPaths resolves the data folder and the engine next to the executable.
// TOLLGATE_DATA_DIR and TOLLGATE_ENGINE override them (development).
func DefaultPaths() (Paths, error) {
	data := os.Getenv("TOLLGATE_DATA_DIR")
	if data == "" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			var err error
			if base, err = os.UserConfigDir(); err != nil {
				return Paths{}, err
			}
		}
		data = filepath.Join(base, AppName)
	}
	engine := os.Getenv("TOLLGATE_ENGINE")
	if engine == "" {
		exe, err := os.Executable()
		if err != nil {
			return Paths{}, err
		}
		name := "tollgate-engine"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		engine = filepath.Join(filepath.Dir(exe), name)
	}
	return Paths{Data: data, Engine: engine}, nil
}

func (p Paths) Config() string    { return filepath.Join(p.Data, "config.yaml") }
func (p Paths) Auths() string     { return filepath.Join(p.Data, "auths") }
func (p Paths) Logs() string      { return filepath.Join(p.Data, "logs") }
func (p Paths) EngineLog() string { return filepath.Join(p.Logs(), "engine.log") }
func (p Paths) AppLog() string    { return filepath.Join(p.Logs(), "app.log") }
func (p Paths) PIDFile() string   { return filepath.Join(p.Data, "engine.pid") }
func (p Paths) Settings() string  { return filepath.Join(p.Data, "settings.json") }
func (p Paths) WebView() string   { return filepath.Join(p.Data, "WebView2") }
func (p Paths) Updates() string   { return filepath.Join(p.Data, "updates") }

// EngineCommit reads engine-commit.txt written next to the engine by the build.
func (p Paths) EngineCommit() string {
	b, err := os.ReadFile(filepath.Join(filepath.Dir(p.Engine), "engine-commit.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (p Paths) EnsureDirectories() error {
	for _, dir := range []string{p.Data, p.Auths(), p.Logs()} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}

// DefaultPort is the proxy port written into a fresh config.yaml.
const DefaultPort = 8317

// BootstrapConfig writes the first config.yaml. After that the engine owns the
// file and every change goes through its management API, which hot-reloads it.
func BootstrapConfig(p Paths) error {
	if _, err := os.Stat(p.Config()); err == nil {
		return nil
	}
	yaml := ConfigTemplate(DefaultPort, GenerateAPIKey(), p.Auths())
	return os.WriteFile(p.Config(), []byte(yaml), 0o600)
}

// ConfigTemplate is the minimal first config: loopback only, one random API
// key, cloaking on, usage tracking on.
func ConfigTemplate(port int, apiKey, authDir string) string {
	return fmt.Sprintf(`# Managed by CLI Tollgate. Edit through the app; manual edits are hot-reloaded.
config-version: 8

server:
  # Loopback only. Web pages can still call it (CORS is open, API key required).
  host: "127.0.0.1"
  port: %d

management:
  allow-remote: false
  # The app passes a per-launch MANAGEMENT_PASSWORD; no key is stored here.
  secret-key: ""
  disable-control-panel: true

access:
  api-keys:
    - %s

oauth:
  auth-dir: %s

upstream:
  codex:
    disable-codex-cloaking: false
  claude:
    disable-claude-cloak-mode: false
    disable-cloaking-model-list: false

usage-tracker:
  enabled: true
`, port, yamlQuote(apiKey), yamlQuote(filepath.ToSlash(authDir)))
}

// yamlQuote single-quotes a scalar: no escape sequences, so Windows paths
// with backslashes survive (only ' itself is doubled).
func yamlQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// GenerateAPIKey returns a fresh client API key ("tg-" + 48 hex chars).
func GenerateAPIKey() string { return "tg-" + randomHex(24) }

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // the OS RNG never fails in practice
	}
	return hex.EncodeToString(b)
}

// ReadPort scans the top-level server: block of config.yaml for port:,
// without a YAML dependency. The engine rewrites the file in normalized block
// style, so this is reliable enough.
func ReadPort(p Paths) int {
	b, err := os.ReadFile(p.Config())
	if err != nil {
		return DefaultPort
	}
	return parsePort(string(b))
}

func parsePort(text string) int {
	inServer := false
	childIndent := -1
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		if indent == 0 {
			inServer = trimmed == "server:"
			childIndent = -1
			continue
		}
		if !inServer {
			continue
		}
		if childIndent < 0 {
			childIndent = indent
		}
		if indent != childIndent || !strings.HasPrefix(trimmed, "port:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "port:"))
		if i := strings.Index(value, " #"); i >= 0 {
			value = value[:i]
		}
		value = strings.Trim(value, ` "'`)
		if port, err := strconv.Atoi(value); err == nil && port >= 1 && port <= 65535 {
			return port
		}
	}
	return DefaultPort
}
