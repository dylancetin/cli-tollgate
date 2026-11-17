package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"sync"
	"testing"
)

// fakeEngine serves /v8/management/config/web-tools like the real engine:
// 404 until something is saved, then the saved body.
type fakeEngine struct {
	mu    sync.Mutex
	saved []byte
	puts  int
}

func (f *fakeEngine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.URL.Path == "/v8/management/config/web-tools" && r.Method == http.MethodGet:
		if f.saved == nil {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not_found"}`))
			return
		}
		_, _ = w.Write(f.saved)
	case r.URL.Path == "/v8/management/config/web-tools" && r.Method == http.MethodPut:
		f.saved, _ = io.ReadAll(r.Body)
		f.puts++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	case r.URL.Path == "/v8/management/config/access/api-keys":
		_, _ = w.Write([]byte(`["tg-test"]`))
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found"}`))
	}
}

func fakeClient(t *testing.T, h http.Handler) Management {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return Management{Port: port, Password: "pw"}
}

func TestLoadWebToolsDefaultsOn404(t *testing.T) {
	m := fakeClient(t, &fakeEngine{})
	w, err := loadWebTools(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	if !w.Enabled || w.Search.Default != "brave" || w.Domains.Mode != DomainsDenylist || len(w.Domains.List) != 0 {
		t.Fatalf("defaults: %+v", w)
	}
	for _, p := range SearchProviders {
		if _, ok := w.Search.Providers[p]; !ok {
			t.Errorf("provider %s missing from defaults", p)
		}
	}
	if len(w.ProvidersWithKeys()) != 0 {
		t.Fatal("no keys by default")
	}
}

func TestWebToolsToleratesMissingAndNullFields(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"enabled":null,"search":null,"domains":null}`,
		`{"search":{"providers":null},"domains":{"list":null}}`,
	} {
		var w WebTools
		if err := json.Unmarshal([]byte(body), &w); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if !w.Enabled || w.Domains.List == nil || w.Search.Providers["exa"].APIKey != "" || w.Domains.Mode != DomainsDenylist {
			t.Errorf("%s -> %+v", body, w)
		}
	}
	var off WebTools
	if err := json.Unmarshal([]byte(`{"enabled":false}`), &off); err != nil || off.Enabled {
		t.Fatalf("explicit false must stay false: %+v %v", off, err)
	}
}

func TestWebToolsRoundTrip(t *testing.T) {
	f := &fakeEngine{}
	m := fakeClient(t, f)
	ctx := context.Background()

	in := WebTools{
		Enabled: false,
		Search: WebSearch{
			Default: "tavily",
			Providers: map[string]WebProvider{
				"brave":  {APIKey: ""},
				"tavily": {APIKey: " tvly-123 "},
				"exa":    {APIKey: "exa-1"},
			},
		},
		Domains: WebDomains{Mode: DomainsAllowlist, List: []string{" Example.COM ", "", "docs.go.dev", "example.com"}},
	}
	if err := saveWebTools(ctx, m, in); err != nil {
		t.Fatal(err)
	}
	var sent map[string]any
	if err := json.Unmarshal(f.saved, &sent); err != nil {
		t.Fatal(err)
	}
	// The whole object goes out, with the engine's key names.
	search := sent["search"].(map[string]any)
	providers := search["providers"].(map[string]any)
	if sent["enabled"] != false || search["default"] != "tavily" || len(providers) != 3 {
		t.Fatalf("sent: %s", f.saved)
	}
	if providers["tavily"].(map[string]any)["api-key"] != "tvly-123" {
		t.Fatalf("key not trimmed: %s", f.saved)
	}

	got, err := loadWebTools(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled || got.Search.Default != "tavily" || got.Domains.Mode != DomainsAllowlist {
		t.Fatalf("loaded: %+v", got)
	}
	if want := []string{"example.com", "docs.go.dev"}; !reflect.DeepEqual(got.Domains.List, want) {
		t.Fatalf("domains = %v, want %v", got.Domains.List, want)
	}
}

func TestProvidersWithKeysAndDefault(t *testing.T) {
	w := DefaultWebTools()
	w.Search.Providers["exa"] = WebProvider{APIKey: "k"}
	w.Search.Providers["tavily"] = WebProvider{APIKey: "k"}
	if got, want := w.ProvidersWithKeys(), []string{"tavily", "exa"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ProvidersWithKeys = %v, want %v", got, want)
	}
	// "brave" has no key, so the default moves to the first provider that does.
	w.Normalize()
	if w.Search.Default != "tavily" {
		t.Fatalf("default = %q", w.Search.Default)
	}
	// A default with a key is left alone.
	w.Search.Default = "exa"
	w.Normalize()
	if w.Search.Default != "exa" {
		t.Fatalf("default = %q", w.Search.Default)
	}
	// Removing every key keeps a valid provider name.
	w.Search.Providers["exa"] = WebProvider{}
	w.Search.Providers["tavily"] = WebProvider{}
	w.Normalize()
	if !isSearchProvider(w.Search.Default) || len(w.ProvidersWithKeys()) != 0 {
		t.Fatalf("after clearing keys: %+v", w)
	}
	// Unknown modes and providers are repaired.
	w.Domains.Mode, w.Search.Default = "weird", "bing"
	w.Normalize()
	if w.Domains.Mode != DomainsDenylist || w.Search.Default != "brave" {
		t.Fatalf("repair: %+v", w)
	}
}

func TestAppRefreshAndSetWebTools(t *testing.T) {
	f := &fakeEngine{}
	m := fakeClient(t, f)
	a := NewApp(Paths{Data: t.TempDir()}, nil, "1.0", 0)
	a.Engine.port, a.Engine.password, a.Engine.state = m.Port, m.Password, EngineRunning

	a.RefreshSettings()
	if w := a.WebTools(); !w.Enabled || w.Search.Default != "brave" {
		t.Fatalf("defaults after 404: %+v", w)
	}
	if got := a.Snapshot().WebTools; !got.Enabled {
		t.Fatalf("snapshot: %+v", got)
	}

	w := a.WebTools()
	w.Enabled = false
	w.Search.Providers["exa"] = WebProvider{APIKey: "exa-key"}
	a.SetWebTools(w)
	if f.puts != 1 {
		t.Fatalf("puts = %d", f.puts)
	}
	a.RefreshSettings()
	got := a.WebTools()
	if got.Enabled || got.Search.Providers["exa"].APIKey != "exa-key" || got.Search.Default != "exa" {
		t.Fatalf("after save and reload: %+v", got)
	}
}
