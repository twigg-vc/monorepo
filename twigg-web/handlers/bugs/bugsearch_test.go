package bugs

import (
	"encoding/json"
	"monorepo/twigg-web/user"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestHandleSearchBugs(t *testing.T) {
	h, db, w, zukoId := newTestHandler(t)
	aangId, err := db.CreateUser(w, "aang@twigg.vc", user.UserState_NoSubscription,
		/*isOrganization*/ false, "aang", "password-hash",
		user.Subscription_None, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateBug(w, testRepoId, zukoId, "Fix Iroh's tea", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateBug(w, testRepoId, zukoId, "Fix the kettle", "")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = db.SetBugAssignee(w, testRepoId, 2, zukoId, aangId)
	if err != nil {
		t.Fatal(err)
	}

	req := newReadReq("/zuko/tea/bug-search?q=assignee:aang")
	req.Flags.SearchBugsUi = true
	rec := httptest.NewRecorder()
	h.handleSearchBugs(rec, req, w)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}
	var resp BugSearchResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	numbers := []uint64{}
	for _, b := range resp.Bugs {
		numbers = append(numbers, b.Number)
	}
	if !reflect.DeepEqual(numbers, []uint64{2}) || resp.NextCursor != "" {
		t.Fatalf("expected only b/2 and no cursor, got %+v", resp)
	}
}

func TestHandleSearchBugsResolvesMe(t *testing.T) {
	h, db, w, zukoId := newTestHandler(t)
	_, err := db.CreateBug(w, testRepoId, zukoId, "Fix Iroh's tea", "")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = db.SetBugAssignee(w, testRepoId, 1, zukoId, zukoId)
	if err != nil {
		t.Fatal(err)
	}

	req := newReadReq("/zuko/tea/bug-search?q=assignee:me")
	req.Flags.SearchBugsUi = true
	req.IsLoggedIn = true
	req.MaybeUserWithReadPermission = &user.User{Id: zukoId, Username: "zuko"}
	rec := httptest.NewRecorder()
	h.handleSearchBugs(rec, req, w)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}
	var resp BugSearchResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Bugs) != 1 || resp.Bugs[0].Number != 1 {
		t.Fatalf("expected b/1 assigned to the logged in user, got %+v", resp)
	}
}

func TestHandleSearchBugsFails(t *testing.T) {
	h, _, w, _ := newTestHandler(t)

	req := newReadReq("/zuko/tea/bug-search?q=assignee:me")
	req.Flags.SearchBugsUi = true
	rec := httptest.NewRecorder()
	h.handleSearchBugs(rec, req, w)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf(`"me" while anonymous: expected 400, got %d`, rec.Code)
	}

	req = newReadReq("/zuko/tea/bug-search?q=is:nope")
	req.Flags.SearchBugsUi = true
	rec = httptest.NewRecorder()
	h.handleSearchBugs(rec, req, w)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad query: expected 400, got %d", rec.Code)
	}

	req = newReadReq("/zuko/tea/bug-search?q=queue")
	req.Flags.SearchBugsUi = true
	rec = httptest.NewRecorder()
	h.handleSearchBugs(rec, req, w)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("free text: expected 400, got %d", rec.Code)
	}

	req = newReadReq("/zuko/tea/bug-search?q=is:open")
	req.Flags.SearchBugsUi = false
	rec = httptest.NewRecorder()
	h.handleSearchBugs(rec, req, w)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("flag off: expected 404, got %d", rec.Code)
	}
}
