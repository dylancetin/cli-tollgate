package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/dylancetin/cli-tollgate/windows/internal/i18n"
)

// Web apps ask for access by generating their own key and calling the
// engine's POST /access/requests. The app long-polls the pending list, shows
// the user an Allow / Deny prompt, and answers through the management API.
// Approved apps become grants that can be revoked from Settings.

// AccessRequest is a pending request from a local app or web page.
//
// Scopes, Upgrade, GrantID and AddedScopes come from engines that support web
// tools; an older engine omits them, which reads as ["models"] and "not an
// upgrade". An upgrade asks for more scopes on a grant that already exists.
type AccessRequest struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Origin      string    `json:"origin"`
	UserAgent   string    `json:"user_agent,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Scopes      []string  `json:"scopes"`
	Upgrade     bool      `json:"upgrade"`
	GrantID     string    `json:"grant_id,omitempty"`
	AddedScopes []string  `json:"added_scopes"`
}

// AccessGrant is an app that was allowed in.
type AccessGrant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Origin    string    `json:"origin"`
	KeyHint   string    `json:"key_hint"`
	GrantedAt time.Time `json:"granted_at"`
	Scopes    []string  `json:"scopes"`
}

const (
	ScopeModels = "models"
	ScopeWeb    = "web"
)

func hasScope(scopes []string, scope string) bool {
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// defaultScopes is what a missing or empty "scopes" means.
func defaultScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{ScopeModels}
	}
	return scopes
}

// UnmarshalJSON fills in the back-compat defaults for old engines.
func (r *AccessRequest) UnmarshalJSON(data []byte) error {
	type plain AccessRequest
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	p.Scopes = defaultScopes(p.Scopes)
	if p.AddedScopes == nil {
		p.AddedScopes = []string{}
	}
	if !p.Upgrade {
		p.GrantID, p.AddedScopes = "", []string{}
	}
	*r = AccessRequest(p)
	return nil
}

func (g *AccessGrant) UnmarshalJSON(data []byte) error {
	type plain AccessGrant
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	p.Scopes = defaultScopes(p.Scopes)
	*g = AccessGrant(p)
	return nil
}

// AsksWeb reports whether approving would give the app web access.
func (r AccessRequest) AsksWeb() bool {
	if r.Upgrade {
		return hasScope(r.AddedScopes, ScopeWeb)
	}
	return hasScope(r.Scopes, ScopeWeb)
}

// HasWeb reports whether the app may use the web tools.
func (g AccessGrant) HasWeb() bool { return hasScope(g.Scopes, ScopeWeb) }

type accessRequestList struct {
	Requests []AccessRequest `json:"requests"`
}

type accessGrantList struct {
	Grants []AccessGrant `json:"grants"`
}

// watchAccessRequests runs for the app's lifetime. Polling the list is also
// what tells the engine that someone can approve, so it keeps going even when
// nothing is pending.
func (a *App) watchAccessRequests() {
	for {
		if !a.running() {
			a.setAccessRequests(nil)
			time.Sleep(2 * time.Second)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
		var list accessRequestList
		err := a.client().LongGet(ctx, "/v8/management/access/requests", url.Values{"wait": {"50"}}, &list)
		cancel()
		switch {
		case isNotFound(err):
			time.Sleep(time.Minute) // engine without access requests
			continue
		case err != nil:
			time.Sleep(3 * time.Second)
			continue
		}
		if a.setAccessRequests(list.Requests) {
			a.Platform.ShowWindow("")
		}
		if len(list.Requests) > 0 {
			// The list answers at once while something is pending; don't spin.
			time.Sleep(time.Second)
		}
	}
}

// setAccessRequests stores the pending list and reports whether it holds a
// request that was not there before.
func (a *App) setAccessRequests(reqs []AccessRequest) bool {
	a.mu.Lock()
	known := map[string]bool{}
	for _, r := range a.accessRequests {
		known[r.ID] = true
	}
	fresh, same := false, len(reqs) == len(a.accessRequests)
	for _, r := range reqs {
		if !known[r.ID] {
			fresh, same = true, false
		}
	}
	a.accessRequests = reqs
	a.mu.Unlock()
	if !same {
		a.notify()
	}
	return fresh
}

func (a *App) RefreshGrants() {
	if !a.running() {
		return
	}
	ctx, cancel := ctx15()
	defer cancel()
	var list accessGrantList
	if err := a.client().Get(ctx, "/v8/management/access/grants", nil, &list); err != nil {
		if !isNotFound(err) {
			a.report(err)
		}
		return
	}
	a.mu.Lock()
	a.grants = list.Grants
	a.mu.Unlock()
	a.notify()
}

// AnswerAccessRequest approves or denies a pending request.
func (a *App) AnswerAccessRequest(id string, allow bool) error {
	if id == "" {
		return errors.New("missing request id")
	}
	verb := "deny"
	if allow {
		verb = "approve"
	}
	ctx, cancel := ctx15()
	defer cancel()
	err := a.client().Post(ctx, "/v8/management/access/requests/"+url.PathEscape(id)+"/"+verb, nil)

	// Drop it locally right away; the watcher brings back anything still pending.
	a.mu.Lock()
	kept := []AccessRequest{}
	for _, r := range a.accessRequests {
		if r.ID != id {
			kept = append(kept, r)
		}
	}
	a.accessRequests = kept
	a.mu.Unlock()
	a.notify()
	if isNotFound(err) {
		return errors.New(i18n.T("This request has expired or was already answered."))
	}
	if err != nil {
		return err
	}
	if allow {
		a.RefreshGrants()
	}
	return nil
}

// RevokeGrant removes an app's key; its requests fail from now on.
func (a *App) RevokeGrant(id string) error {
	ctx, cancel := ctx15()
	defer cancel()
	if err := a.client().Delete(ctx, "/v8/management/access/grants/"+url.PathEscape(id), nil); err != nil && !isNotFound(err) {
		return err
	}
	a.RefreshGrants()
	return nil
}
