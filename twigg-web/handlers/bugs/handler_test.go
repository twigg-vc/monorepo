package bugs

import (
	"context"
	"encoding/json"
	"html"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/bugpermissions"
	"monorepo/twigg-web/user"
	twiggwc "monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/webdb"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
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
		// What the mux passes for anonymous users
		MaybeUserWithReadPermission: &user.User{},
		Repo:                        repo.Repo{Id: testRepoId},
		Flags:                       featureflags.Flags{ShowBugs: true},
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

func newBugPageReq(number string, viewer *user.User) wrappers.UserWithReadPermissionMuxRequest {
	req := newReadReq("/zuko/tea/b/" + number)
	req.SetPathValue(routes.BugNumberParamName, number)
	if viewer != nil {
		req.IsLoggedIn = true
		req.MaybeUserWithReadPermission = viewer
	}
	return req
}

// Returns the unescaped value of the <bug-page> attribute, or "" without it.
func bugPageAttr(t *testing.T, page, name string) string {
	t.Helper()
	m := regexp.MustCompile(`<bug-page[^>]* ` + name + `(?:="([^"]*)")?[ >]`).FindStringSubmatch(page)
	if m == nil {
		return ""
	}
	return html.UnescapeString(m[1])
}

func TestGetBug(t *testing.T) {
	h, db, w, zukoId := newTestHandler(t)
	db.SetNower(mockNow{now: time.UnixMilli(200)}, t)
	if _, err := db.CreateBug(w, testRepoId, zukoId, "Fix Iroh's tea", "The tea is cold"); err != nil {
		t.Fatal(err)
	}
	db.SetNower(mockNow{now: time.UnixMilli(201)}, t)
	if _, _, err := db.AddBugComment(w, testRepoId, 1, zukoId, "Try jasmine"); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.handleGetBug(rec, newBugPageReq("1", nil), w)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}
	var b twiggwc.FrontendBug
	if err := json.Unmarshal([]byte(bugPageAttr(t, rec.Body.String(), "Bug")), &b); err != nil {
		t.Fatal(err)
	}
	// reflect.DeepEqual doesn't work well with dates
	if b.CreatedOn.UnixMilli() != 200 || b.UpdatedOn.UnixMilli() != 201 {
		t.Fatalf("unexpected timestamps: %+v", b)
	}
	b.CreatedOn, b.UpdatedOn = time.Time{}, time.Time{}
	wantBug := twiggwc.FrontendBug{
		Number:           1,
		Title:            "Fix Iroh's tea",
		Body:             "The tea is cold",
		Status:           bug.Status_Open,
		AuthorUsername:   "zuko",
		AssigneeUsername: "",
		CommentCount:     1,
		CreatedOn:        time.Time{},
		UpdatedOn:        time.Time{},
	}
	if !reflect.DeepEqual(b, wantBug) {
		t.Fatalf("expected %+v, got %+v", wantBug, b)
	}

	var events []twiggwc.FrontendBugEvent
	if err := json.Unmarshal([]byte(bugPageAttr(t, rec.Body.String(), "Events")), &events); err != nil {
		t.Fatal(err)
	}
	// reflect.DeepEqual doesn't work well with dates
	if len(events) != 1 || events[0].CreatedOn.UnixMilli() != 201 {
		t.Fatalf("unexpected events %+v", events)
	}
	events[0].CreatedOn = time.Time{}
	wantEvents := []twiggwc.FrontendBugEvent{{
		Id:             1,
		Kind:           twiggwc.FrontendBugEventKind_Comment,
		AuthorUsername: "zuko",
		CreatedOn:      time.Time{},
		Comment:        &bug.Comment{Body: "Try jasmine"},
	}}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("expected %+v, got %+v", wantEvents, events)
	}
}

func bugPageHasAttr(page, name string) bool {
	return regexp.MustCompile(`<bug-page[^>]* ` + name + `[ =>]`).MatchString(page)
}

func TestGetBugLetsWritersWrite(t *testing.T) {
	h, db, w, zukoId := newTestHandler(t)
	if _, err := db.CreateBug(w, testRepoId, zukoId, "Fix Iroh's tea", ""); err != nil {
		t.Fatal(err)
	}
	canWrite := func(viewer *user.User) bool {
		t.Helper()
		req := newBugPageReq("1", viewer)
		req.Repo.OwnerId = zukoId
		rec := httptest.NewRecorder()
		h.handleGetBug(rec, req, w)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
		}
		return bugPageHasAttr(rec.Body.String(), "CanWrite")
	}

	if !canWrite(&user.User{Id: zukoId}) {
		t.Fatal("expected the owner to be able to write")
	}
	if canWrite(&user.User{Id: zukoId + 1}) {
		t.Fatal("expected a stranger to not be able to write")
	}
	if canWrite(nil) {
		t.Fatal("expected anonymous users to not be able to write")
	}
}

func TestGetBugFails(t *testing.T) {
	h, db, w, zukoId := newTestHandler(t)
	if _, err := db.CreateBug(w, testRepoId, zukoId, "Fix Iroh's tea", ""); err != nil {
		t.Fatal(err)
	}
	get := func(req wrappers.UserWithReadPermissionMuxRequest) int {
		t.Helper()
		rec := httptest.NewRecorder()
		h.handleGetBug(rec, req, w)
		return rec.Code
	}

	req := newBugPageReq("1", nil)
	req.Flags.ShowBugs = false
	if code := get(req); code != http.StatusNotFound {
		t.Fatalf("flag off: expected 404, got %d", code)
	}
	if code := get(newBugPageReq("tea", nil)); code != http.StatusBadRequest {
		t.Fatalf("bad number: expected 400, got %d", code)
	}
	if code := get(newBugPageReq("2", nil)); code != http.StatusNotFound {
		t.Fatalf("missing bug: expected 404, got %d", code)
	}
}

func newBugWriteReq(u user.User, repoOwnerId int64, number string, body string) wrappers.UserRepoMuxRequest {
	req := newWriteReq(u, repoOwnerId, body)
	req.SetPathValue(routes.BugNumberParamName, number)
	return req
}

func TestPostComment(t *testing.T) {
	h, db, w, zukoId := newTestHandler(t)
	if _, err := db.CreateBug(w, testRepoId, zukoId, "Fix Iroh's tea", ""); err != nil {
		t.Fatal(err)
	}
	db.SetNower(mockNow{now: time.UnixMilli(200)}, t)

	rec := httptest.NewRecorder()
	shouldCommit := h.handlePostComment(rec, newBugWriteReq(user.User{Id: zukoId, Username: "zuko"},
		zukoId, "1", `{"Body": "Try jasmine"}`), w)
	if rec.Code != http.StatusOK || !shouldCommit {
		t.Fatalf("expected 200 and a commit, got %d shouldCommit=%v: %s", rec.Code, shouldCommit, rec.Body)
	}
	var got twiggwc.FrontendBugEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	// reflect.DeepEqual doesn't work well with dates
	if got.CreatedOn.UnixMilli() != 200 {
		t.Fatalf("unexpected timestamp: %+v", got)
	}
	got.CreatedOn = time.Time{}
	want := twiggwc.FrontendBugEvent{
		Id:             1,
		Kind:           twiggwc.FrontendBugEventKind_Comment,
		AuthorUsername: "zuko",
		CreatedOn:      time.Time{},
		Comment:        &bug.Comment{Body: "Try jasmine"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
	events, _, err := db.GetBugEvents(w, testRepoId, 1, "", 10)
	if err != nil || len(events) != 1 || events[0].Comment.Body != "Try jasmine" {
		t.Fatalf("expected the comment to be stored, got %+v (err=%v)", events, err)
	}
}

func TestPostCommentFails(t *testing.T) {
	h, db, w, zukoId := newTestHandler(t)
	if _, err := db.CreateBug(w, testRepoId, zukoId, "Fix Iroh's tea", ""); err != nil {
		t.Fatal(err)
	}
	zuko, stranger := user.User{Id: zukoId, Username: "zuko"}, user.User{Id: zukoId + 1}
	post := func(u user.User, number string, flag bool, body string) int {
		t.Helper()
		req := newBugWriteReq(u, zukoId, number, body)
		req.Flags.ShowBugs = flag
		rec := httptest.NewRecorder()
		if h.handlePostComment(rec, req, w) {
			t.Fatalf("expected no commit, got %d: %s", rec.Code, rec.Body)
		}
		return rec.Code
	}
	const comment = `{"Body": "Try jasmine"}`

	if code := post(stranger, "1", true, comment); code != http.StatusForbidden {
		t.Fatalf("stranger: expected 403, got %d", code)
	}
	if events, _, _ := db.GetBugEvents(w, testRepoId, 1, "", 10); len(events) != 0 {
		t.Fatalf("expected the stranger's comment to not be stored, got %+v", events)
	}
	if code := post(zuko, "1", false, comment); code != http.StatusNotFound {
		t.Fatalf("flag off: expected 404, got %d", code)
	}
	if code := post(zuko, "tea", true, comment); code != http.StatusBadRequest {
		t.Fatalf("bad number: expected 400, got %d", code)
	}
	if code := post(zuko, "2", true, comment); code != http.StatusNotFound {
		t.Fatalf("missing bug: expected 404, got %d", code)
	}
	if code := post(zuko, "1", true, `{"Body": "   "}`); code != http.StatusBadRequest {
		t.Fatalf("blank comment: expected 400, got %d", code)
	}
	tooLong, err := json.Marshal(PostCommentRequest{Body: strings.Repeat("a", bug.MaxBodyLen+1)})
	if err != nil {
		t.Fatal(err)
	}
	if code := post(zuko, "1", true, string(tooLong)); code != http.StatusBadRequest {
		t.Fatalf("comment too long: expected 400, got %d", code)
	}
}

func TestPostStatus(t *testing.T) {
	h, db, w, zukoId := newTestHandler(t)
	db.SetNower(mockNow{now: time.UnixMilli(199)}, t)
	if _, err := db.CreateBug(w, testRepoId, zukoId, "Fix Iroh's tea", ""); err != nil {
		t.Fatal(err)
	}
	db.SetNower(mockNow{now: time.UnixMilli(200)}, t)

	rec := httptest.NewRecorder()
	shouldCommit := h.handlePostStatus(rec, newBugWriteReq(user.User{Id: zukoId, Username: "zuko"},
		zukoId, "1", `{"Status": "closed"}`), w)
	if rec.Code != http.StatusOK || !shouldCommit {
		t.Fatalf("expected 200 and a commit, got %d shouldCommit=%v: %s", rec.Code, shouldCommit, rec.Body)
	}
	var got PostStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	// reflect.DeepEqual doesn't work well with dates
	if got.Bug.CreatedOn.UnixMilli() != 199 || got.Bug.UpdatedOn.UnixMilli() != 200 ||
		len(got.Events) != 1 || got.Events[0].CreatedOn.UnixMilli() != 200 {
		t.Fatalf("unexpected timestamps: %+v", got)
	}
	got.Bug.CreatedOn, got.Bug.UpdatedOn, got.Events[0].CreatedOn = time.Time{}, time.Time{}, time.Time{}
	want := PostStatusResponse{
		Bug: twiggwc.FrontendBug{
			Number:           1,
			Title:            "Fix Iroh's tea",
			Body:             "",
			Status:           bug.Status_Closed,
			AuthorUsername:   "zuko",
			AssigneeUsername: "",
			CommentCount:     0,
			CreatedOn:        time.Time{},
			UpdatedOn:        time.Time{},
		},
		Events: []twiggwc.FrontendBugEvent{{
			Id:              1,
			Kind:            twiggwc.FrontendBugEventKind_StatusChange,
			AuthorUsername:  "zuko",
			CreatedOn:       time.Time{},
			Comment:         nil,
			StatusChange:    &bug.StatusChange{NewStatus: bug.Status_Closed},
			DescriptionEdit: nil,
			TitleEdit:       nil,
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}