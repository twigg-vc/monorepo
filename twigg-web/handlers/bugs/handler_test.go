package bugs

import (
	"context"
	"encoding/json"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/services/bugpermissions"
	"monorepo/twigg-web/user"
	twiggwc "monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/webdb"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
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

func newWriteReq(u user.User, repoOwnerId int64, body string) wrappers.UserRepoMuxRequest {
	return wrappers.UserRepoMuxRequest{
		Request:                 httptest.NewRequest("POST", "/zuko/tea/bugs", strings.NewReader(body)),
		UserWithWritePermission: u,
		Repo:                    repo.Repo{Id: testRepoId, OwnerId: repoOwnerId},
		Flags:                   featureflags.Flags{ShowBugs: true},
	}
}

func TestPostBug(t *testing.T) {
	h, db, w, zukoId := newTestHandler(t)
	db.SetNower(mockNow{now: time.UnixMilli(200)}, t)

	rec := httptest.NewRecorder()
	shouldCommit := h.handlePostBug(rec, newWriteReq(user.User{Id: zukoId}, zukoId,
		`{"Title": "  Fix Iroh's tea  ", "Body": "The tea is cold"}`), w)
	if rec.Code != http.StatusOK || !shouldCommit {
		t.Fatalf("expected 200 and a commit, got %d shouldCommit=%v: %s", rec.Code, shouldCommit, rec.Body)
	}
	var got twiggwc.FrontendBug
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	// reflect.DeepEqual doesn't work well with dates
	if got.CreatedOn.UnixMilli() != 200 || got.UpdatedOn.UnixMilli() != 200 {
		t.Fatalf("unexpected timestamps: %+v", got)
	}
	got.CreatedOn, got.UpdatedOn = time.Time{}, time.Time{}
	want := twiggwc.FrontendBug{
		Number:           1,
		Title:            "Fix Iroh's tea",
		Body:             "The tea is cold",
		Status:           bug.Status_Open,
		AuthorUsername:   "zuko",
		AssigneeUsername: "",
		CommentCount:     0,
		CreatedOn:        time.Time{},
		UpdatedOn:        time.Time{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
	if _, isNotFoundErr, err := db.GetBug(w, testRepoId, 1); err != nil || isNotFoundErr {
		t.Fatalf("expected b/1 to be created, got isNotFoundErr=%v err=%v", isNotFoundErr, err)
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

type mockNow struct {
	now time.Time
}

func (m mockNow) Now() time.Time {
	return m.now
}

func TestPostBugFails(t *testing.T) {
	h, db, w, zukoId := newTestHandler(t)
	post := func(u user.User, flag bool, body string) int {
		t.Helper()
		req := newWriteReq(u, zukoId, body)
		req.Flags.ShowBugs = flag
		rec := httptest.NewRecorder()
		if h.handlePostBug(rec, req, w) {
			t.Fatalf("expected no commit, got %d: %s", rec.Code, rec.Body)
		}
		return rec.Code
	}

	if code := post(user.User{Id: zukoId + 1}, true, `{"Title": "Fix Iroh's tea"}`); code != http.StatusForbidden {
		t.Fatalf("stranger: expected 403, got %d", code)
	}
	if _, isNotFoundErr, _ := db.GetBug(w, testRepoId, 1); !isNotFoundErr {
		t.Fatal("expected the stranger's bug to not be created")
	}
	if code := post(user.User{Id: zukoId}, false, `{"Title": "Fix Iroh's tea"}`); code != http.StatusNotFound {
		t.Fatalf("flag off: expected 404, got %d", code)
	}
	if code := post(user.User{Id: zukoId}, true, `{"Title": "   "}`); code != http.StatusBadRequest {
		t.Fatalf("blank title: expected 400, got %d", code)
	}
	if code := post(user.User{Id: zukoId}, true, `not json`); code != http.StatusBadRequest {
		t.Fatalf("invalid json: expected 400, got %d", code)
	}
}

func TestPostBugLimitsItsSize(t *testing.T) {
	h, _, w, zukoId := newTestHandler(t)
	post := func(title, body string) int {
		t.Helper()
		reqBody, err := json.Marshal(PostBugRequest{Title: title, Body: body})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		h.handlePostBug(rec, newWriteReq(user.User{Id: zukoId}, zukoId, string(reqBody)), w)
		return rec.Code
	}

	if code := post(strings.Repeat("茶", bug.MaxTitleLen), ""); code != http.StatusOK {
		t.Fatalf("title of %d characters: expected 200, got %d", bug.MaxTitleLen, code)
	}
	if code := post(strings.Repeat("a", bug.MaxTitleLen+1), ""); code != http.StatusBadRequest {
		t.Fatalf("title too long: expected 400, got %d", code)
	}
	if code := post("Fix Iroh's tea", strings.Repeat("a", bug.MaxBodyLen+1)); code != http.StatusBadRequest {
		t.Fatalf("body too long: expected 400, got %d", code)
	}
}