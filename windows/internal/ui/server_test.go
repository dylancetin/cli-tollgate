package ui

import (
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"

	"github.com/dylancetin/cli-tollgate/windows/internal/core"
)

type fakePlatform struct{ copied string }

func (f *fakePlatform) CopyText(t string) error     { f.copied = t; return nil }
func (f *fakePlatform) OpenURL(string) error        { return nil }
func (f *fakePlatform) OpenFile(string) error       { return nil }
func (f *fakePlatform) RevealFile(string) error     { return nil }
func (f *fakePlatform) LaunchAtLogin() bool         { return false }
func (f *fakePlatform) SetLaunchAtLogin(bool) error { return nil }
func (f *fakePlatform) ShowWindow(string)           {}
func (f *fakePlatform) Quit()                       {}

func TestAccessControl(t *testing.T) {
	plat := &fakePlatform{}
	app := core.NewApp(core.Paths{Data: t.TempDir(), Engine: "missing"}, plat, "1.0", 0)
	s, err := Start(app, "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := "http://" + s.addr

	status := func(c *http.Client, method, url string, hdr map[string]string, body string) int {
		req, _ := http.NewRequest(method, url, strings.NewReader(body))
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		if k, ok := hdr["Host"]; ok {
			req.Host = k
		}
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	anon := &http.Client{}
	if got := status(anon, "GET", base+"/", nil, ""); got != 403 {
		t.Fatalf("page without cookie: %d", got)
	}
	if got := status(anon, "GET", base+"/auth?t=wrong", nil, ""); got != 403 {
		t.Fatalf("auth with wrong token: %d", got)
	}

	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	if got := status(c, "GET", s.URL("models"), nil, ""); got != 200 {
		t.Fatalf("auth + redirect to page: %d", got)
	}
	if got := status(anon, "GET", base+"/lang.js", nil, ""); got != 403 {
		t.Fatalf("lang.js without cookie: %d", got)
	}
	if got := status(c, "GET", base+"/lang.js", nil, ""); got != 200 {
		t.Fatalf("lang.js with cookie: %d", got)
	}
	api := map[string]string{"X-Tollgate": "1"}
	if got := status(c, "GET", base+"/api/state", api, ""); got != 200 {
		t.Fatalf("state with cookie+header: %d", got)
	}
	if got := status(c, "GET", base+"/api/state", nil, ""); got != 403 {
		t.Fatalf("state without header: %d", got)
	}
	if got := status(c, "GET", base+"/api/state", map[string]string{"X-Tollgate": "1", "Host": "attacker.example:80"}, ""); got != 403 {
		t.Fatalf("rebinding host: %d", got)
	}
	if got := status(c, "POST", base+"/api/copy", api, `{"text":"hello"}`); got != 204 || plat.copied != "hello" {
		t.Fatalf("copy action: %d %q", got, plat.copied)
	}
	if got := status(c, "POST", base+"/api/openURL", api, `{"text":"file:///C:/Windows/System32/calc.exe"}`); got != 422 {
		t.Fatalf("non-https openURL: %d", got)
	}
	if got := status(c, "POST", base+"/api/nope", api, `{}`); got != 422 {
		t.Fatalf("unknown action: %d", got)
	}
}
