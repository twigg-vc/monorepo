package bugs

import (
	"context"
	"encoding/json"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/repo"
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
	return handler{db: db}, db, w, zukoId
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
	for range 2 {
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
}
