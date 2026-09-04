package sqlitehelper_test

import (
	"testing"
)

func TestShouldCommit(t *testing.T) {
	s := newInMemoryDb(t)

	w, closeTx, _, err := s.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()

	if s.ShouldCommit(w) {
		t.Fatal("should not commit before any write happened")
	}

	_, err = s.Exec(w, `INSERT INTO migrations_test_dummy1 (id) VALUES (99);`)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ShouldCommit(w) {
		t.Fatal("should commit after a successful Exec")
	}

	_, err = s.Exec(w, `INSERT INTO does_not_exist (id) VALUES (1);`)
	if err == nil {
		t.Fatal("expected an error inserting into a nonexistent table")
	}
	if s.ShouldCommit(w) {
		t.Fatal("should not commit after a failed Exec")
	}

	// A later successful Exec must not undo the earlier failure.
	_, err = s.Exec(w, `INSERT INTO migrations_test_dummy1 (id) VALUES (100);`)
	if err != nil {
		t.Fatal(err)
	}
	if s.ShouldCommit(w) {
		t.Fatal("should not commit even if a later Exec succeeds")
	}
}

func TestPreventCommit(t *testing.T) {
	s := newInMemoryDb(t)

	w, closeTx, _, err := s.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()

	_, err = s.Exec(w, `INSERT INTO migrations_test_dummy1 (id) VALUES (99);`)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ShouldCommit(w) {
		t.Fatal("should commit after a successful Exec")
	}

	s.PreventCommit(w)
	if s.ShouldCommit(w) {
		t.Fatal("should not commit after PreventCommit was called")
	}

	// A later successful Exec must not undo PreventCommit.
	_, err = s.Exec(w, `INSERT INTO migrations_test_dummy1 (id) VALUES (100);`)
	if err != nil {
		t.Fatal(err)
	}
	if s.ShouldCommit(w) {
		t.Fatal("should not commit after PreventCommit, even after a later successful Exec")
	}
}

func TestShouldCommitAndPreventCommit_PanicOnReadOnlyContext(t *testing.T) {
	s := newInMemoryDb(t)

	r, closeTx, err := s.BeginRead()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()

	expectPanic := func(name string, fn func()) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s: expected panic when called with a read-only context", name)
				}
			}()
			fn()
		})
	}
	expectPanic("ShouldCommit", func() { s.ShouldCommit(r) })
	expectPanic("PreventCommit", func() { s.PreventCommit(r) })
}
