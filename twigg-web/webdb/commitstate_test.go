package webdb_test

import (
	"errors"
	"io"
	"testing"
)

// Always fails, to exercise a blob write that never reaches SQL (unlike a
// failed Exec, which sqlitehelper tracks on its own).
type failingWriterTo struct{}

func (failingWriterTo) WriteTo(w io.Writer) (int64, error) {
	return 0, errors.New("boom")
}

func Test_ShouldCommit(t *testing.T) {
	cliDb := getNewDb(t)

	w, closeW, _, err := cliDb.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	if cliDb.ShouldCommit(w) {
		t.Fatal("should not commit before any write happened")
	}

	err = cliDb.SetRepoNextLocalId(w, 99, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !cliDb.ShouldCommit(w) {
		t.Fatal("should commit after a successful write")
	}
}

func Test_ShouldCommit_AfterFailedSqlWrite(t *testing.T) {
	cliDb := getNewDb(t)

	w, closeW, _, err := cliDb.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	err = cliDb.SetRepoNextLocalId(w, 99, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !cliDb.ShouldCommit(w) {
		t.Fatal("should commit after a successful write")
	}

	// Force a write to fail by closing the underlying transaction early.
	closeW()
	err = cliDb.SetRepoNextLocalId(w, 99, 4)
	if err == nil {
		t.Fatal("expected an error writing on a closed transaction")
	}
	if cliDb.ShouldCommit(w) {
		t.Fatal("should not commit after a failed write")
	}
}

// A blob write can fail before ever issuing SQL (e.g. while writing the blob
// bytes themselves), so it needs its own tracking distinct from Exec's.
func Test_ShouldCommit_AfterFailedBlobWrite(t *testing.T) {
	cliDb := getNewDb(t)

	w, closeW, _, err := cliDb.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	err = cliDb.SetRepoNextLocalId(w, 99, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !cliDb.ShouldCommit(w) {
		t.Fatal("should commit after a successful write")
	}

	_, err = cliDb.SetBlob(w, "owner", "prefix", "id", failingWriterTo{})
	if err == nil {
		t.Fatal("expected an error from the failing WriterTo")
	}
	if cliDb.ShouldCommit(w) {
		t.Fatal("should not commit after a failed blob write")
	}

	// A later successful write must not undo the earlier failure.
	err = cliDb.SetRepoNextLocalId(w, 99, 4)
	if err != nil {
		t.Fatal(err)
	}
	if cliDb.ShouldCommit(w) {
		t.Fatal("should not commit even if a later write succeeds")
	}
}

func Test_PreventCommit(t *testing.T) {
	cliDb := getNewDb(t)

	w, closeW, _, err := cliDb.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	err = cliDb.SetRepoNextLocalId(w, 99, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !cliDb.ShouldCommit(w) {
		t.Fatal("should commit after a successful write")
	}

	cliDb.PreventCommit(w)
	if cliDb.ShouldCommit(w) {
		t.Fatal("should not commit after PreventCommit was called")
	}

	// Even further successful writes must not undo PreventCommit.
	err = cliDb.SetRepoNextLocalId(w, 99, 4)
	if err != nil {
		t.Fatal(err)
	}
	if cliDb.ShouldCommit(w) {
		t.Fatal("should not commit after PreventCommit was called, even after a later successful write")
	}
}

func Test_ShouldCommit_PanicsOnNonWriteContext(t *testing.T) {
	cliDb := getNewDb(t)

	r, closeR, err := cliDb.BeginRead()
	if err != nil {
		t.Fatal(err)
	}
	defer closeR()

	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic when calling ShouldCommit with a non-write context")
		}
	}()
	cliDb.ShouldCommit(r)
}
