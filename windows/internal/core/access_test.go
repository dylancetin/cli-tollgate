package core

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSetAccessRequestsReportsNewOnes(t *testing.T) {
	a := &App{changed: make(chan struct{})}
	r1 := AccessRequest{ID: "1", Name: "One", Origin: "https://one.example"}
	r2 := AccessRequest{ID: "2", Name: "Two", Origin: "https://two.example"}

	if !a.setAccessRequests([]AccessRequest{r1}) {
		t.Fatal("first request should be reported as new")
	}
	rev := a.revision
	if a.setAccessRequests([]AccessRequest{r1}) {
		t.Fatal("same list should not be reported as new")
	}
	if a.revision != rev {
		t.Fatal("unchanged list should not notify")
	}
	if !a.setAccessRequests([]AccessRequest{r1, r2}) {
		t.Fatal("added request should be reported as new")
	}
	if a.setAccessRequests([]AccessRequest{r2}) {
		t.Fatal("removing a request is not new")
	}
	if a.revision == rev {
		t.Fatal("changed list should notify")
	}
}

func TestAccessRequestDecoding(t *testing.T) {
	var list accessRequestList
	body := `{"requests":[
		{"id":"old","name":"Old","origin":"https://old.example"},
		{"id":"new","name":"New","origin":"https://new.example","scopes":["models","web"]},
		{"id":"up","name":"Up","origin":"https://up.example","scopes":["models","web"],"upgrade":true,"grant_id":"g1","added_scopes":["web"]}
	]}`
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatal(err)
	}
	old, fresh, up := list.Requests[0], list.Requests[1], list.Requests[2]
	if !reflect.DeepEqual(old.Scopes, []string{"models"}) || old.Upgrade || old.AsksWeb() {
		t.Errorf("request from an old engine: %+v", old)
	}
	if !fresh.AsksWeb() || fresh.Upgrade {
		t.Errorf("new connection with web: %+v", fresh)
	}
	if !up.Upgrade || up.GrantID != "g1" || !reflect.DeepEqual(up.AddedScopes, []string{"web"}) || !up.AsksWeb() {
		t.Errorf("upgrade: %+v", up)
	}

	// What the web UI receives: arrays, never null.
	out, _ := json.Marshal(old)
	if !strings.Contains(string(out), `"scopes":["models"]`) || !strings.Contains(string(out), `"added_scopes":[]`) {
		t.Errorf("encoded old request: %s", out)
	}
}

func TestAccessUpgradeWithoutWebDoesNotAskForWeb(t *testing.T) {
	var r AccessRequest
	if err := json.Unmarshal([]byte(`{"id":"1","scopes":["models","web"],"upgrade":true,"grant_id":"g","added_scopes":["models"]}`), &r); err != nil {
		t.Fatal(err)
	}
	if r.AsksWeb() {
		t.Fatal("an upgrade that adds no web scope must not show the web prompt")
	}
}

func TestAccessGrantDecoding(t *testing.T) {
	var list accessGrantList
	body := `{"grants":[{"id":"a","name":"A","key_hint":"…1234"},{"id":"b","name":"B","scopes":["models","web"]}]}`
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatal(err)
	}
	if g := list.Grants[0]; g.HasWeb() || !reflect.DeepEqual(g.Scopes, []string{"models"}) {
		t.Errorf("grant without scopes: %+v", g)
	}
	if !list.Grants[1].HasWeb() {
		t.Errorf("grant with web: %+v", list.Grants[1])
	}
}
