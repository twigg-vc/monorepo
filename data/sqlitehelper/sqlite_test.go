package sqlitehelper_test

import (
	"context"
	"embed"
	"errors"
	"monorepo/data/sqlitehelper"
	"testing"
	"testing/fstest"
	"time"
)

//go:embed dummy-migrations/*.sql
var embeddedMigrations embed.FS

// Returns a database that stores the data in memory.
// In order for it to work properly, it is capped at a single connection; so it
// should not be used to test concurrency.
func newInMemoryDb(t *testing.T) sqlitehelper.SqliteHelper {
	s, err := sqlitehelper.NewSqliteHelper(sqlitehelper.InMemoryPathToDir, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err = s.Init(embeddedMigrations); err != nil {
		t.Fatal(err)
	}
	return s
}

// Unlike newInMemoryDb, this one is not capped at a single connection, so
// readers can run while a write transaction is open.
func newFileDb(t *testing.T) sqlitehelper.SqliteHelper {
	s, err := sqlitehelper.NewSqliteHelper(t.TempDir(), "test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err = s.Init(embeddedMigrations); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestInitInTmpDir(t *testing.T) {
	s, err := sqlitehelper.NewSqliteHelper(t.TempDir(), "test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Init(embeddedMigrations); err != nil {
		t.Fatal(err)
	}
}

func TestInit_Idempotency_AndMigrationTampering(t *testing.T) {
	s, err := sqlitehelper.NewSqliteHelper(t.TempDir(), "test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	fakeFS := fstest.MapFS{
		"0000001_create_foo.sql": {
			Data: []byte("CREATE TABLE foo (id INT);")},
	}

	// Init is idempotent
	if err = s.Init(fakeFS); err != nil {
		t.Fatal(err)
	}
	if err = s.Init(fakeFS); err != nil {
		t.Fatal(err)
	}

	// But it identifies tampered migrations (change INT -> TEXT)
	fakeFS = fstest.MapFS{
		"0000001_create_foo.sql": {
			Data: []byte("CREATE TABLE foo (id TEXT);")},
	}
	if err = s.Init(fakeFS); !errors.Is(err, sqlitehelper.ErrTamperedMigration) {
		t.Fatal("got no error with modified migration")
	}
}

func TestCantPassReadCtxAgain(t *testing.T) {
	s := newInMemoryDb(t)
	readCtx, closeTx, err := s.BeginRead()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()

	if _, _, err = s.BeginReadWithCtx(readCtx); err == nil {
		t.Fatal("expected error passing a context that already has a read tx")
	}
	if _, _, _, err = s.BeginWriteWithCtx(readCtx); err == nil {
		t.Fatal("expected error passing a read ctx to BeginWrite")
	}
}

func TestCantPassWriteCtxAgain(t *testing.T) {
	s := newInMemoryDb(t)
	writeCtx, closeTx, _, err := s.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()

	if _, _, err = s.BeginReadWithCtx(writeCtx); err == nil {
		t.Fatal("expected error passing a write ctx to BeginRead")
	}
	if _, _, _, err = s.BeginWriteWithCtx(writeCtx); err == nil {
		t.Fatal("expected error passing a context that already has a write tx")
	}
}

func TestQueryDummyTables(t *testing.T) {
	s := newInMemoryDb(t)
	readTx, closeTx, err := s.BeginRead()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()
	var count1 int64
	err = s.QueryRow(readTx,
		`SELECT COUNT(*) FROM migrations_test_dummy1;`).Scan(&count1)
	if err != nil {
		t.Fatal(err)
	}
	if count1 != 1 {
		t.Fatalf("count=%d, expected 1", count1)
	}
	var count2 int64
	err = s.QueryRow(readTx,
		`SELECT COUNT(*) FROM migrations_test_dummy2;`).Scan(&count2)
	if err != nil {
		t.Fatal(err)
	}
	if count2 != 1 {
		t.Fatalf("count=%d, expected 1", count2)
	}
}

func TestInsertToDummyTable(t *testing.T) {
	s := newInMemoryDb(t)

	// Add one row. Note that one row already exists bc one migration adds it.
	writeTx, closeWriteTx, commitTx, err := s.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeWriteTx()
	_, err = s.Exec(writeTx,
		`INSERT INTO migrations_test_dummy1 (id) VALUES (99);`)
	if err != nil {
		t.Fatal(err)
	}
	err = commitTx()
	if err != nil {
		t.Fatal(err)
	}
	closeWriteTx()

	// Read back in a separate tx to ensure it worked
	readTx, closeTx, err := s.BeginRead()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()
	var count int64
	err = s.QueryRow(readTx,
		`SELECT COUNT(*) FROM migrations_test_dummy1;`).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count=%d, expected 1", count)
	}
}

func TestSingleWriterMultiReader(t *testing.T) {
	s := newFileDb(t)

	// Helper function that counts the rows of migrations_test_dummy1
	countRows := func(c context.Context) int64 {
		var rowCount int64
		err := s.QueryRow(c,
			"SELECT COUNT(*) from migrations_test_dummy1").Scan(&rowCount)
		if err != nil {
			t.Fatal(err)
		}
		return rowCount
	}

	// Get a write that will insert to the table
	w1, closeW1, commitW1, err := s.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeW1)
	_, err = s.Exec(w1,
		`INSERT INTO migrations_test_dummy1 (id) VALUES (2);`)
	if err != nil {
		t.Fatal(err)
	}
	cw1 := countRows(w1)
	if cw1 != 2 {
		t.Fatalf("count=%d, expected 2", cw1)
	}
	// Start another goroutine that will try to write. This one will block until
	// w1 is closed
	w2IsOpen := false
	w2ErrCh := make(chan error, 1)
	go func() {
		w2, closeW2, commitW2, err := s.BeginWrite()
		if err != nil {
			closeW2()
			w2ErrCh <- err
			return
		}
		w2IsOpen = true
		_, err = s.Exec(w2,
			`INSERT INTO migrations_test_dummy1 (id) VALUES (3);`)
		if err != nil {
			w2ErrCh <- err
			return
		}
		err = commitW2()
		if err != nil {
			w2ErrCh <- err
			return
		}
		closeW2()
		close(w2ErrCh)
	}()
	// Sleep a bit to ensure w2 really is blocking
	time.Sleep(5 * time.Millisecond)
	if w2IsOpen {
		t.Fatalf("two write transactions oppened at the same time")
	}

	// Others can read while the lock is aquired; but the values are isolated
	r1, closeR1, err := s.BeginRead()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeR1)
	r2, closeR2, err := s.BeginRead()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeR2)
	c1 := countRows(r1)
	if c1 != 1 {
		t.Fatalf("c1=%d, expected 1", c1)
	}
	c2 := countRows(r2)
	if c2 != 1 {
		t.Fatalf("c2=%d, expected 1", c2)
	}

	// Commit the tx that started earlier that adds a row
	err = commitW1()
	if err != nil {
		t.Fatal(err)
	}
	closeW1()

	// wait for w2 to complete
	err = <-w2ErrCh
	if err != nil {
		t.Fatal(err)
	}

	// r1 and r2 should still read a single row
	c1 = countRows(r1)
	if c1 != 1 {
		t.Fatalf("c1=%d, expected 1", c1)
	}
	c2 = countRows(r2)
	if c2 != 1 {
		t.Fatalf("c2=%d, expected 1", c2)
	}

	// But a new transaction will read what was committed
	r3, closeR3, err := s.BeginRead()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeR3)
	c3 := countRows(r3)
	if c3 != 3 {
		t.Fatalf("c3=%d, expected 3", c3)
	}
}

func TestBeginWriteTimesOutOnStuckWriteTx(t *testing.T) {
	s := newFileDb(t)
	s.SetMaxWaitForWriteTx(50*time.Millisecond, t)

	// Simulate a caller that is stuck holding a write tx: it's never closed
	_, closeStuckW, _, err := s.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}

	// Others give up instead of blocking forever
	start := time.Now()
	_, closeW, _, err := s.BeginWrite()
	closeW()
	if !errors.Is(err, sqlitehelper.ErrBeginWriteTimeout) {
		t.Fatalf("err=%v, expected ErrBeginWriteTimeout", err)
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("gave up before waiting for maxWaitForWriteTx")
	}

	// Reads are unaffected
	_, closeR, err := s.BeginRead()
	if err != nil {
		t.Fatal(err)
	}
	closeR()

	// Once the stuck writer goes away, writes work again
	closeStuckW()
	w, closeW, _, err := s.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()
	if _, err = s.Exec(w,
		`INSERT INTO migrations_test_dummy1 (id) VALUES (4);`); err != nil {
		t.Fatal(err)
	}
}

func TestMultipleBeginWriteTimeout(t *testing.T) {
	s := newFileDb(t)
	s.SetMaxWaitForWriteTx(10*time.Millisecond, t)

	// Ensure that a failed BeginWrite doesn't take the semaphore
	_, closeStuckW, _, err := s.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		_, closeW, _, err := s.BeginWrite()
		closeW()
		if !errors.Is(err, sqlitehelper.ErrBeginWriteTimeout) {
			t.Fatalf("err=%v, expected ErrBeginWriteTimeout", err)
		}
	}
	closeStuckW()

	_, closeW, _, err := s.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	closeW()
}
