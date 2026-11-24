// tollgatectl manages connected apps on a CLIProxyAPI engine that already
// runs as a service (for example a Linux server), without the Tollgate window.
// It talks to the engine's management API on loopback, so run it on the same
// machine as the engine.
//
//	tollgatectl grant --name NAME [--scopes models,web]   mint a revocable key
//	tollgatectl apps                                     list connected apps
//	tollgatectl revoke GRANT_ID                          turn an app's key off
//	tollgatectl watch                                    answer access requests in the terminal
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dylancetin/cli-tollgate/windows/internal/core"
)

// grantOrigin labels keys minted here in the grant list. The engine keys
// pending requests and scope upgrades by origin.
const grantOrigin = "tollgatectl"

const usage = `Usage: tollgatectl [--port N] [--password-file PATH] <command> [args]

Commands:
  grant --name NAME [--scopes models,web]  mint a revocable key and print it
  apps                                     list connected apps
  revoke GRANT_ID                          remove an app and turn its key off
  watch                                    approve or deny access requests from local apps

Defaults: port 8317 ($TOLLGATE_PORT), password file
~/.config/cliproxyapi/management-password ($TOLLGATE_PASSWORD_FILE).
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tollgatectl:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	global := flag.NewFlagSet("tollgatectl", flag.ContinueOnError)
	global.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	port := global.Int("port", envInt("TOLLGATE_PORT", 8317), "engine port")
	passwordFile := global.String("password-file", envOr("TOLLGATE_PASSWORD_FILE", defaultPasswordFile()), "file holding the management password")
	if err := global.Parse(args); err != nil {
		return err
	}
	if global.NArg() == 0 {
		global.Usage()
		return errors.New("missing command")
	}
	cmd, rest := global.Arg(0), global.Args()[1:]
	if cmd == "help" || cmd == "-h" || cmd == "--help" {
		global.Usage()
		return nil
	}

	data, err := os.ReadFile(*passwordFile)
	if err != nil {
		return fmt.Errorf("reading the management password: %w", err)
	}
	password := strings.TrimSpace(string(data))
	if password == "" {
		return fmt.Errorf("%s is empty", *passwordFile)
	}
	c := ctl{m: core.Management{Port: *port, Password: password}, port: *port}

	switch cmd {
	case "grant":
		return c.grant(rest)
	case "apps":
		return c.apps(rest)
	case "revoke":
		return c.revoke(rest)
	case "watch":
		return c.watch(rest)
	default:
		global.Usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

type ctl struct {
	m    core.Management
	port int
}

var plainHTTP = &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil}}

func (c ctl) grant(args []string) error {
	fs := flag.NewFlagSet("grant", flag.ContinueOnError)
	name := fs.String("name", "", "app name shown in the grant list (required)")
	scopeList := fs.String("scopes", core.ScopeModels, "comma-separated scopes: models, web")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*name) == "" || fs.NArg() > 0 {
		return errors.New("usage: tollgatectl grant --name NAME [--scopes models,web]")
	}
	scopes := []string{}
	for _, s := range strings.Split(*scopeList, ",") {
		if s = strings.TrimSpace(s); s != "" && !slices.Contains(scopes, s) {
			scopes = append(scopes, s)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// The engine only takes access requests while an approver has polled the
	// pending list recently. This poll makes tollgatectl that approver for the
	// request it is about to create.
	var pending struct {
		Requests []core.AccessRequest `json:"requests"`
	}
	if err := c.m.Get(ctx, "/v8/management/access/requests", nil, &pending); err != nil {
		return fmt.Errorf("management API: %w", err)
	}

	key := core.GenerateAPIKey()
	id, err := c.createRequest(ctx, *name, key, scopes)
	if err != nil {
		return err
	}
	var g core.AccessGrant
	if err := c.m.Post(ctx, "/v8/management/access/requests/"+url.PathEscape(id)+"/approve", &g); err != nil {
		return fmt.Errorf("approving the request: %w", err)
	}
	// The engine reloads its config after saving the new key; wait until the
	// key is live so the caller can use it right away.
	if err := c.waitForKey(ctx, key, http.StatusOK); err != nil {
		return fmt.Errorf("grant %s was saved but its key is not accepted yet: %w", g.ID, err)
	}
	fmt.Fprintf(os.Stderr, "Granted %q (id %s, scopes %s). The key is shown only once:\n", g.Name, g.ID, strings.Join(g.Scopes, ","))
	fmt.Println(key)
	return nil
}

func (c ctl) createRequest(ctx context.Context, name, key string, scopes []string) (string, error) {
	body, _ := json.Marshal(map[string]any{"name": name, "key": key, "scopes": scopes})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("/access/requests"), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", grantOrigin)
	resp, err := plainHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		ID    string `json:"id"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	_ = json.Unmarshal(data, &out)
	if resp.StatusCode != http.StatusAccepted || out.ID == "" {
		msg := out.Error
		if msg == "" {
			msg = strings.TrimSpace(string(data))
		}
		if out.Code != "" {
			msg += " (" + out.Code + ")"
		}
		return "", fmt.Errorf("access request refused: %d %s", resp.StatusCode, msg)
	}
	return out.ID, nil
}

// waitForKey polls /v1/models with key until it answers want.
func (c ctl) waitForKey(ctx context.Context, key string, want int) error {
	last := 0
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url("/v1/models"), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+key)
		if resp, err := plainHTTP.Do(req); err == nil {
			resp.Body.Close()
			last = resp.StatusCode
			if last == want {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("last status %d", last)
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (c ctl) apps(args []string) error {
	if len(args) > 0 {
		return errors.New("usage: tollgatectl apps")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var list struct {
		Grants []core.AccessGrant `json:"grants"`
	}
	if err := c.m.Get(ctx, "/v8/management/access/grants", nil, &list); err != nil {
		return fmt.Errorf("management API: %w", err)
	}
	if len(list.Grants) == 0 {
		fmt.Fprintln(os.Stderr, "No connected apps.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tORIGIN\tSCOPES\tKEY\tGRANTED")
	for _, g := range list.Grants {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", g.ID, g.Name, g.Origin, strings.Join(g.Scopes, ","), g.KeyHint, g.GrantedAt.Local().Format("2006-01-02 15:04"))
	}
	return w.Flush()
}

func (c ctl) revoke(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: tollgatectl revoke GRANT_ID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err := c.m.Delete(ctx, "/v8/management/access/grants/"+url.PathEscape(args[0]), nil)
	var apiErr *core.APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return fmt.Errorf("no grant with id %s (see tollgatectl apps)", args[0])
	}
	if err != nil {
		return fmt.Errorf("management API: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Revoked %s; its key no longer works.\n", args[0])
	return nil
}

// watch is the terminal version of the desktop app's Allow / Deny prompt.
// While it runs, programs on this machine can ask for access with
// POST /access/requests; without it the engine refuses them (no_approver).
func (c ctl) watch(args []string) error {
	if len(args) > 0 {
		return errors.New("usage: tollgatectl watch")
	}
	ctx := context.Background()
	in := bufio.NewReader(os.Stdin)
	answered := map[string]bool{}
	fmt.Fprintln(os.Stderr, "Waiting for access requests (Ctrl-C to stop)...")
	for {
		pollCtx, cancel := context.WithTimeout(ctx, 70*time.Second)
		var list struct {
			Requests []core.AccessRequest `json:"requests"`
		}
		err := c.m.LongGet(pollCtx, "/v8/management/access/requests", url.Values{"wait": {"50"}}, &list)
		cancel()
		if err != nil {
			fmt.Fprintln(os.Stderr, "management API:", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, r := range list.Requests {
			if answered[r.ID] {
				continue
			}
			answered[r.ID] = true
			allow, err := ask(in, r)
			if err != nil {
				return err
			}
			verb := "deny"
			if allow {
				verb = "approve"
			}
			err = c.m.Post(ctx, "/v8/management/access/requests/"+url.PathEscape(r.ID)+"/"+verb, nil)
			var apiErr *core.APIError
			switch {
			case errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound:
				fmt.Fprintln(os.Stderr, "  That request expired or was already answered.")
			case err != nil:
				fmt.Fprintln(os.Stderr, "  management API:", err)
			case allow:
				fmt.Fprintln(os.Stderr, "  Allowed.")
			default:
				fmt.Fprintln(os.Stderr, "  Denied.")
			}
		}
		if len(list.Requests) > 0 {
			time.Sleep(time.Second) // the list answers at once while something is pending
		}
	}
}

func ask(in *bufio.Reader, r core.AccessRequest) (bool, error) {
	what := "use your subscriptions"
	if r.Upgrade {
		what = "get more scopes: " + strings.Join(r.AddedScopes, ",")
	} else if r.AsksWeb() {
		what = "use your subscriptions and search the web"
	}
	fmt.Fprintf(os.Stderr, "\n%q from %s wants to %s.\n", r.Name, r.Origin, what)
	if r.UserAgent != "" {
		fmt.Fprintf(os.Stderr, "  User agent: %s\n", r.UserAgent)
	}
	fmt.Fprintf(os.Stderr, "  Expires %s. Allow? [y/N] ", r.ExpiresAt.Local().Format("15:04:05"))
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return false, fmt.Errorf("reading the answer: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}

func (c ctl) url(path string) string { return fmt.Sprintf("http://127.0.0.1:%d%s", c.port, path) }

func defaultPasswordFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "management-password"
	}
	return filepath.Join(home, ".config", "cliproxyapi", "management-password")
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func envInt(name string, fallback int) int {
	if n, err := strconv.Atoi(os.Getenv(name)); err == nil && n > 0 {
		return n
	}
	return fallback
}
