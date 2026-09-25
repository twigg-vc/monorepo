package webdb_test

import (
	"context"
	"errors"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/webdb"
	"reflect"
	"testing"
	"time"
)

const (
	bugsRepoId   = uint64(7)
	bugsAuthorId = int64(3)
)

func newBugsDb(t *testing.T) (b webdb.WebDb, w context.Context) {
	t.Helper()
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cl)
	w, closeW, _, err := b.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeW)
	return b, w
}

type mockNow struct {
	now time.Time
}

func (m mockNow) Now() time.Time {
	return m.now
}

func TestCreateAndGetBug(t *testing.T) {
	b, w := newBugsDb(t)
	now := mockNow{
		now: time.UnixMilli(199),
	}
	b.SetNower(now, t)

	created, err := b.CreateBug(w, bugsRepoId, bugsAuthorId, "Fix Iroh's tea", "Uncle Iroh's tea is cold and must be warmed")
	if err != nil {
		t.Fatal(err)
	}
	// reflect.DeepEqual doesn't work well with dates
	if created.CreatedOn.UnixMilli() != 199 || created.UpdatedOn.UnixMilli() != 199 {
		t.Fatalf("unexpected timestamps: created %+v", created)
	}
	originalCreatedOn := created.CreatedOn
	originalUpdatedOn := created.UpdatedOn
	created.CreatedOn, created.UpdatedOn = time.Time{}, time.Time{}
	want := bug.Bug{
		Number:         1,
		Title:          "Fix Iroh's tea",
		Body:           "Uncle Iroh's tea is cold and must be warmed",
		Status:         bug.Status_Open,
		AuthorUserId:   bugsAuthorId,
		AssigneeUserId: 0,
		CreatedOn:      time.Time{},
		UpdatedOn:      time.Time{},
	}
	if !reflect.DeepEqual(created, want) {
		t.Fatalf("CreateBug: expected %+v, got %+v", want, created)
	}

	got, isNotFoundErr, err := b.GetBug(w, bugsRepoId, created.Number)
	if err != nil || isNotFoundErr {
		t.Fatalf("GetBug: isNotFoundErr=%v err=%v", isNotFoundErr, err)
	}
	// reflect.DeepEqual doesn't work well with dates
	if !got.CreatedOn.Equal(originalCreatedOn) ||
		!got.UpdatedOn.Equal(originalUpdatedOn) {
		t.Fatalf("unexpected timestamps: got %+v", got)
	}
	got.CreatedOn, got.UpdatedOn = time.Time{}, time.Time{}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetBug: expected %+v, got %+v", want, got)
	}
}

func TestGetBugNotFound(t *testing.T) {
	b, w := newBugsDb(t)
	_, err := b.CreateBug(w, bugsRepoId, bugsAuthorId, "Fix Iroh's tea", "")
	if err != nil {
		t.Fatal(err)
	}

	_, isNotFoundErr, err := b.GetBug(w, bugsRepoId, 2)
	if !isNotFoundErr || !errors.Is(err, webdb.ErrNotFound) {
		t.Fatalf("expected not found, got isNotFoundErr=%v err=%v", isNotFoundErr, err)
	}
	_, isNotFoundErr, err = b.GetBug(w, bugsRepoId+1, 1)
	if !isNotFoundErr || !errors.Is(err, webdb.ErrNotFound) {
		t.Fatalf("expected bug of another repo to not be found, got isNotFoundErr=%v err=%v",
			isNotFoundErr, err)
	}
}

func TestBugNumbersAreSequentialPerRepo(t *testing.T) {
	b, w := newBugsDb(t)

	for _, want := range []struct {
		repoId uint64
		number uint64
	}{{bugsRepoId, 1}, {bugsRepoId, 2}, {bugsRepoId + 1, 1}, {bugsRepoId, 3}} {
		created, err := b.CreateBug(w, want.repoId, bugsAuthorId, "Fix Iroh's tea", "")
		if err != nil {
			t.Fatal(err)
		}
		if created.Number != want.number {
			t.Fatalf("repo %d: expected number %d, got %d", want.repoId, want.number, created.Number)
		}
	}
}

func TestCreateBugWithoutTitleFails(t *testing.T) {
	b, w := newBugsDb(t)
	_, err := b.CreateBug(w, bugsRepoId, bugsAuthorId, "", "Uncle Iroh's tea is cold")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAddBugComment(t *testing.T) {
	b, w := newBugsDb(t)
	b.SetNower(mockNow{now: time.UnixMilli(199)}, t)
	created, err := b.CreateBug(w, bugsRepoId, bugsAuthorId, "Fix Iroh's tea", "")
	if err != nil {
		t.Fatal(err)
	}
	const commenterId = bugsAuthorId + 1
	b.SetNower(mockNow{now: time.UnixMilli(200)}, t)

	e, isNotFoundErr, err := b.AddBugComment(w, bugsRepoId, created.Number, commenterId, "Try jasmine")
	if err != nil || isNotFoundErr {
		t.Fatalf("AddBugComment: isNotFoundErr=%v err=%v", isNotFoundErr, err)
	}
	// reflect.DeepEqual doesn't work well with dates
	if e.CreatedOn.UnixMilli() != 200 {
		t.Fatalf("unexpected timestamp: event %+v", e)
	}
	e.CreatedOn = time.Time{}
	want := bug.Event{
		Id:           1,
		Kind:         bug.EventKind_Comment,
		AuthorUserId: commenterId,
		CreatedOn:    time.Time{},
		Comment:      bug.Comment{Body: "Try jasmine"},
	}
	if !reflect.DeepEqual(e, want) {
		t.Fatalf("AddBugComment: expected %+v, got %+v", want, e)
	}

	got, _, err := b.GetBug(w, bugsRepoId, created.Number)
	if err != nil {
		t.Fatal(err)
	}
	if got.UpdatedOn.UnixMilli() != 200 {
		t.Fatalf("expected the comment to bump UpdatedOn to 200, got %v", got.UpdatedOn.UnixMilli())
	}
}

func TestAddBugCommentFails(t *testing.T) {
	b, w := newBugsDb(t)
	created, err := b.CreateBug(w, bugsRepoId, bugsAuthorId, "Fix Iroh's tea", "")
	if err != nil {
		t.Fatal(err)
	}

	_, isNotFoundErr, err := b.AddBugComment(w, bugsRepoId, created.Number+1, bugsAuthorId, "Try jasmine")
	if !isNotFoundErr || !errors.Is(err, webdb.ErrNotFound) {
		t.Fatalf("expected not found, got isNotFoundErr=%v err=%v", isNotFoundErr, err)
	}
	_, isNotFoundErr, err = b.AddBugComment(w, bugsRepoId, created.Number, bugsAuthorId, "")
	if err == nil || isNotFoundErr {
		t.Fatalf("expected an error for an empty comment, got isNotFoundErr=%v err=%v", isNotFoundErr, err)
	}
}
func TestGetBugEvents(t *testing.T) {
	b, w := newBugsDb(t)
	for range 2 {
		if _, err := b.CreateBug(w, bugsRepoId, bugsAuthorId, "Fix Iroh's tea", ""); err != nil {
			t.Fatal(err)
		}
	}
	for i, c := range []struct {
		number uint64
		body   string
	}{{1, "Try jasmine"}, {2, "Not this bug"}, {1, "Try ginseng"}} {
		b.SetNower(mockNow{now: time.UnixMilli(200 + int64(i))}, t)
		if _, _, err := b.AddBugComment(w, bugsRepoId, c.number, bugsAuthorId, c.body); err != nil {
			t.Fatal(err)
		}
	}

	events, nextCursor, err := b.GetBugEvents(w, bugsRepoId, 1, "", 10)
	if err != nil || nextCursor != "" {
		t.Fatalf("GetBugEvents: nextCursor=%q err=%v", nextCursor, err)
	}
	// reflect.DeepEqual doesn't work well with dates
	if len(events) != 2 || events[0].CreatedOn.UnixMilli() != 200 || events[1].CreatedOn.UnixMilli() != 202 {
		t.Fatalf("unexpected timestamps: %+v", events)
	}
	events[0].CreatedOn, events[1].CreatedOn = time.Time{}, time.Time{}
	want := []bug.Event{
		{Id: 1, Kind: bug.EventKind_Comment, AuthorUserId: bugsAuthorId, Comment: bug.Comment{Body: "Try jasmine"}},
		{Id: 3, Kind: bug.EventKind_Comment, AuthorUserId: bugsAuthorId, Comment: bug.Comment{Body: "Try ginseng"}},
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("expected %+v, got %+v", want, events)
	}
}
func TestGetBugEventsPages(t *testing.T) {
	b, w := newBugsDb(t)
	for range 2 {
		if _, err := b.CreateBug(w, bugsRepoId, bugsAuthorId, "Fix Iroh's tea", ""); err != nil {
			t.Fatal(err)
		}
	}
	// b/1 gets the events 1, 2, 4, 5 and 6; b/2 gets 3.
	for _, number := range []uint64{1, 1, 2, 1, 1, 1} {
		if _, _, err := b.AddBugComment(w, bugsRepoId, number, bugsAuthorId, "Try jasmine"); err != nil {
			t.Fatal(err)
		}
	}
	getPage := func(cursor string, limit int) (ids []uint64, nextCursor string) {
		t.Helper()
		events, nextCursor, err := b.GetBugEvents(w, bugsRepoId, 1, cursor, limit)
		if err != nil {
			t.Fatal(err)
		}
		ids = []uint64{}
		for _, e := range events {
			ids = append(ids, e.Id)
		}
		return ids, nextCursor
	}

	ids, cursor := getPage("", 2)
	if !reflect.DeepEqual(ids, []uint64{5, 6}) || cursor == "" {
		t.Fatalf("page 1: expected [5 6] and a cursor, got %v cursor=%q", ids, cursor)
	}
	ids, cursor = getPage(cursor, 2)
	if !reflect.DeepEqual(ids, []uint64{2, 4}) || cursor == "" {
		t.Fatalf("page 2: expected [2 4] and a cursor, got %v cursor=%q", ids, cursor)
	}
	ids, cursor = getPage(cursor, 2)
	if !reflect.DeepEqual(ids, []uint64{1}) || cursor != "" {
		t.Fatalf("page 3: expected [1] and no cursor, got %v cursor=%q", ids, cursor)
	}

	ids, cursor = getPage("", 5)
	if !reflect.DeepEqual(ids, []uint64{1, 2, 4, 5, 6}) || cursor != "" {
		t.Fatalf("exact page: expected [1 2 4 5 6] and no cursor, got %v cursor=%q", ids, cursor)
	}
}

func TestGetBugEventsFails(t *testing.T) {
	b, w := newBugsDb(t)
	if _, err := b.CreateBug(w, bugsRepoId, bugsAuthorId, "Fix Iroh's tea", ""); err != nil {
		t.Fatal(err)
	}

	if _, _, err := b.GetBugEvents(w, bugsRepoId, 1, "not a cursor!", 2); err == nil {
		t.Fatal("expected an error for a malformed cursor")
	}
	if _, _, err := b.GetBugEvents(w, bugsRepoId, 1, "", 0); err == nil {
		t.Fatal("expected an error for a zero limit")
	}
}