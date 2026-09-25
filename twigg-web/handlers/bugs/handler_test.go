package bugs

import (
	"context"
	"encoding/json"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/services/bugpermissions"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webdb"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

const testRepoId = uint64(7)

// A handler whose db has "zuko" as the author of the repo's bugs.
func newTestHandler(t *testing.T) (h handler, db webdb.WebDb, w context.Context, zukoId int64) {
	t.Helper()
	db, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cl)
	w, closeW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeW)
	zukoId, err = db.CreateUser(w, "zuko@twigg.vc", user.UserState_NoSubscription,
		/*isOrganization*/ false, "zuko", "password-hash",
		user.Subscription_None, 0)
	if err != nil {
		t.Fatal(err)
	}
	return handler{db: db, perms: bugpermissions.NewService(db)}, db, w, zukoId
}

func newReadReq(target string) wrappers.UserWithReadPermissionMuxRequest {
	return wrappers.UserWithReadPermissionMuxRequest{
		Request: httptest.NewRequest("GET", target, nil),
		Repo:    repo.Repo{Id: testRepoId},
		Flags:   featureflags.Flags{ShowBugs: true},
	}
}

func TestGetBugs(t *testing.T) {
	h, db, w, zukoId := newTestHandler(t)
	for range 3 {
		if _, err := db.CreateBug(w, testRepoId, zukoId, "Fix Iroh's tea", "The tea is cold"); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := db.SetBugStatus(w, testRepoId, 1, zukoId, bug.Status_Closed); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.handleGetBugs(rec, newReadReq("/zuko/tea/bugs?status=closed"), w)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}
	var resp GetBugsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	numbers, authors := []uint64{}, []string{}
	for _, b := range resp.Bugs {
		numbers, authors = append(numbers, b.Number), append(authors, b.AuthorUsername)
	}
	if !reflect.DeepEqual(numbers, []uint64{1}) || !reflect.DeepEqual(authors, []string{"zuko"}) || resp.NextCursor != "" {
		t.Fatalf("expected the closed b/1 by zuko and no cursor, got %+v", resp)
	}
	if resp.OpenCount != 2 || resp.ClosedCount != 1 {
		t.Fatalf("expected the counts of the whole repo, got open=%d closed=%d", resp.OpenCount, resp.ClosedCount)
	}
	if resp.CanCreate {
		t.Fatal("expected anonymous users to not be able to create bugs")
	}
}

func TestGetBugsLetsTheRepoOwnerCreate(t *testing.T) {
	h, _, w, zukoId := newTestHandler(t)
	req := newReadReq("/zuko/tea/bugs")
	req.Repo.OwnerId = zukoId
	req.IsLoggedIn = true
	req.MaybeUserWithReadPermission = &user.User{Id: zukoId}

	rec := httptest.NewRecorder()
	h.handleGetBugs(rec, req, w)
	var resp GetBugsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.CanCreate {
		t.Fatalf("expected the owner to be able to create bugs, got %+v", resp)
	}
}

func TestGetBugsFails(t *testing.T) {
	h, _, w, _ := newTestHandler(t)

	req := newReadReq("/zuko/tea/bugs")
	req.Flags.ShowBugs = false
	rec := httptest.NewRecorder()
	h.handleGetBugs(rec, req, w)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("flag off: expected 404, got %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.handleGetBugs(rec, newReadReq("/zuko/tea/bugs?status=resolved"), w)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad status: expected 400, got %d", rec.Code)
	}
}
