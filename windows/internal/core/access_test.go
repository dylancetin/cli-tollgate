package core

import "testing"

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
