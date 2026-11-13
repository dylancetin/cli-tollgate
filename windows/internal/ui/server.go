// Package ui serves the app's HTML interface on a random loopback port. The
// WebView2 window (or, as a fallback, the default browser) loads it.
//
// Access control: a random per-launch token is exchanged once at /auth for a
// SameSite=Strict, HttpOnly cookie. API calls also need the X-Tollgate header
// (cross-origin pages cannot send it without a preflight we never answer), and
// the Host header must be our own loopback address (no DNS rebinding).
package ui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dylancetin/cli-tollgate/windows/internal/core"
)

//go:embed web
var webFS embed.FS

const cookieName = "tollgate"

type Server struct {
	app   *core.App
	token string
	addr  string
	srv   *http.Server
}

func Start(app *core.App, addr string) (*Server, error) {
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	s := &Server{app: app, token: hex.EncodeToString(b), addr: ln.Addr().String()}
	s.srv = &http.Server{Handler: s.routes(), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = s.srv.Serve(ln) }()
	return s, nil
}

// URL opens the UI on a tab ("overview", "models", "settings").
func (s *Server) URL(tab string) string {
	u := fmt.Sprintf("http://%s/auth?t=%s", s.addr, s.token)
	if tab != "" {
		u += "#" + tab
	}
	return u
}

func (s *Server) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = s.srv.Shutdown(ctx)
}

func (s *Server) routes() http.Handler {
	static, _ := fs.Sub(webFS, "web")
	files := http.FileServer(http.FS(static))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth", s.handleAuth)
	mux.Handle("GET /", s.requireCookie(noCache(files)))
	mux.Handle("GET /api/state", s.requireAPI(http.HandlerFunc(s.handleState)))
	mux.Handle("POST /api/{action}", s.requireAPI(http.HandlerFunc(s.handleAction)))
	return s.checkHost(securityHeaders(mux))
}

func (s *Server) checkHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != s.addr {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("t")), []byte(s.token)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: s.token, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
	// The fragment (tab) survives the redirect.
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) authorized(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	return err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.token)) == 1
}

func (s *Server) requireCookie(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			http.Error(w, "Open CLI Tollgate from its tray icon.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) || r.Header.Get("X-Tollgate") != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleState long-polls: with ?since=<revision> it waits up to 25s for a change.
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if since, err := strconv.ParseUint(r.URL.Query().Get("since"), 10, 64); err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		s.app.WaitForChange(ctx, since)
		cancel()
	}
	writeJSON(w, s.app.FullSnapshot())
}

type actionBody struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Alias    string `json:"alias"`
	On       bool   `json:"on"`
	Port     int    `json:"port"`
	Text     string `json:"text"`
	Tab      string `json:"tab"`
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	var b actionBody
	if r.ContentLength != 0 {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&b); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
	}
	if err := s.dispatch(r.PathValue("action"), b); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var errUnknownAction = errors.New("unknown action")

func (s *Server) dispatch(action string, b actionBody) error {
	a := s.app
	provider := func() (core.Provider, error) { return core.ParseProvider(b.Provider) }
	switch action {
	case "refresh":
		go a.RefreshAll()
	case "setEngine":
		a.SetEngineEnabled(b.On)
	case "restartEngine":
		a.Engine.Restart()
	case "login":
		p, err := provider()
		if err != nil {
			return err
		}
		go a.Login(p) // long-running: the UI follows progress through state
	case "cancelLogin":
		a.CancelLogin()
	case "logout":
		p, err := provider()
		if err != nil {
			return err
		}
		a.Logout(p)
	case "setExposed":
		p, err := provider()
		if err != nil {
			return err
		}
		a.SetExposed(p, b.Model, b.On)
	case "setAllExposed":
		p, err := provider()
		if err != nil {
			return err
		}
		a.SetAllExposed(p, b.On)
	case "setAlias":
		p, err := provider()
		if err != nil {
			return err
		}
		a.SetAlias(p, b.Model, b.Alias)
	case "setCloaking":
		a.SetCloaking(b.On)
	case "setClaudeCloaking":
		a.SetClaudeCloaking(b.On)
	case "setClaudeCloakModelList":
		a.SetClaudeCloakModelList(b.On)
	case "setCodexCloaking":
		a.SetCodexCloaking(b.On)
	case "regenerateAPIKey":
		return a.RegenerateAPIKey()
	case "setPort":
		return a.ChangePort(b.Port)
	case "setLaunchAtLogin":
		a.SetLaunchAtLogin(b.On)
	case "setAutoCheckUpdates":
		a.UpdateSettings(func(s *core.Settings) { s.AutoCheckUpdates = b.On })
	case "setAutoInstallUpdates":
		a.UpdateSettings(func(s *core.Settings) { s.AutoInstallUpdates = b.On })
	case "checkForUpdates":
		go a.Updater.Check(true)
	case "installUpdate":
		go func() {
			if err := a.Updater.InstallNow(); err != nil {
				a.ReportError(err.Error())
			}
		}()
	case "copy":
		a.Copy(b.Text)
	case "openURL":
		if !strings.HasPrefix(b.Text, "https://") {
			return errors.New("only https links can be opened")
		}
		return a.Platform.OpenURL(b.Text)
	case "openLogs":
		return a.Platform.OpenFile(a.Paths.EngineLog())
	case "openDataFolder":
		return a.Platform.RevealFile(a.Paths.Config())
	case "dismissError":
		a.DismissError()
	case "quit":
		go a.Platform.Quit()
	default:
		return errUnknownAction
	}
	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
