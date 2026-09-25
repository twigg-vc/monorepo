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

func TestCreateAndGetBug(t *testing.T) {
	b, w := newBugsDb(t)

	created, err := b.CreateBug(w, bugsRepoId, bugsAuthorId, "Fix Iroh's tea", "Uncle Iroh's tea is cold and must be warmed")
	if err != nil {
		t.Fatal(err)
	}
	// reflect.DeepEqual doesn't work well with dates
	if created.CreatedOn.IsZero() || !created.UpdatedOn.Equal(created.CreatedOn) {
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
