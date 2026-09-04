package sqlock

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSingleWriterAndReader(t *testing.T) {
	db := setupTest(t)

	// Create a test table
	createTableTx, unlockCreateTableTx, err := db.Write()
	if err != nil {
		t.Fatal(err)
	}
	defer unlockCreateTableTx()
	_, err = createTableTx.Exec(`CREATE TABLE my_table (Name TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	err = createTableTx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	unlockCreateTableTx()

	// Insert into the table but dont commit the tx
	insertButDontCommitTx, unlockInsertButDontCommitTx, err := db.Write()
	if err != nil {
		t.Fatal(err)
	}
	defer unlockInsertButDontCommitTx()
	_, err = insertButDontCommitTx.Exec(`INSERT INTO my_table (Name) VALUES ("aang")`)
	if err != nil {
		t.Fatal(err)
	}
	// Querying in the tx should see the values
	rowCount := -1
	err = insertButDontCommitTx.QueryRow(`SELECT COUNT(*) FROM my_table`).Scan(&rowCount)
	if err != nil {
		t.Fatal(err)
	}
	if rowCount != 1 {
		t.Fatalf("expected count 1 got %d", rowCount)
	}
	unlockInsertButDontCommitTx() // unlock without commiting

	// Helper to count rows
	countTableRows := func() int {
		countTx, unlockCountTx, err := db.Read()
		if err != nil {
			t.Fatal(err)
		}
		defer unlockCountTx()
		rowCount := -1
		err = countTx.QueryRow(`SELECT COUNT(*) FROM my_table`).Scan(&rowCount)
		if err != nil {
			t.Fatal(err)
		}
		return rowCount
	}

	// When reading, expect 0 rows bc insertButDontCommitTx was not commited
	rowCount = countTableRows()
	if rowCount != 0 {
		t.Fatalf("expected count 0 got %d", rowCount)
	}

	// Insert into the table and commit the tx
	insertTx, unlockInsertTx, err := db.Write()
	if err != nil {
		t.Fatal(err)
	}
	defer unlockInsertTx()
	_, err = insertTx.Exec(`INSERT INTO my_table (Name) VALUES ("appa")`)
	if err != nil {
		t.Fatal(err)
	}
	err = insertTx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	unlockInsertTx()

	// When reading, expect 1 rows because the insertTx was commited
	rowCount = countTableRows()
	if rowCount != 1 {
		t.Fatalf("expected count 0 got %d", rowCount)
	}
}

func TestManyReaders(t *testing.T) {
	db := setupTest(t)

	// Create a test table
	createTableTx, unlockCreateTableTx, err := db.Write()
	if err != nil {
		t.Fatal(err)
	}
	defer unlockCreateTableTx()
	_, err = createTableTx.Exec(`CREATE TABLE my_table (Name TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	err = createTableTx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	unlockCreateTableTx()

	// Insert one row
	insertTx, unlockInsertTx, err := db.Write()
	if err != nil {
		t.Fatal(err)
	}
	defer unlockInsertTx()
	_, err = insertTx.Exec(`INSERT INTO my_table (Name) VALUES ("appa")`)
	if err != nil {
		t.Fatal(err)
	}
	err = insertTx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	unlockInsertTx()

	// Helper to count rows using a ReadTx
	checkRowCount := func(tx ReadTx, expectedCount int) {
		rowCount := expectedCount + 1
		err = tx.QueryRow(`SELECT COUNT(*) FROM my_table`).Scan(&rowCount)
		if err != nil {
			t.Fatal(err)
		}
		if rowCount != expectedCount {
			t.Fatalf("expected count %d got %d", expectedCount, rowCount)
		}
	}

	// Start many readers
	r1Tx, unlockr1Tx, err := db.Read()
	if err != nil {
		t.Fatal(err)
	}
	defer unlockr1Tx()
	r2Tx, unlockr2Tx, err := db.Read()
	if err != nil {
		t.Fatal(err)
	}
	defer unlockr2Tx()
	r3Tx, unlockr3Tx, err := db.Read()
	if err != nil {
		t.Fatal(err)
	}
	defer unlockr3Tx()
	checkRowCount(r1Tx, 1)
	checkRowCount(r2Tx, 1)
	checkRowCount(r3Tx, 1)
}

func TestWriterBlocks(t *testing.T) {
	db := setupTest(t)

	// Start a writeTx
	w, unlockW, err := db.Write()
	if err != nil {
		t.Fatal(err)
	}
	defer unlockW()
	// In parallel, start another tx that will read the num of rows and
	// put it in a variable. It'll block untill the writeTx is unlocked
	rowCount0 := -1
	startedReading0 := false
	read0ErrCh := make(chan error, 1)
	go func() {
		r, unlockR, err := db.Read()
		if err != nil {
			read0ErrCh <- err
			return
		}
		startedReading0 = true
		defer unlockR()
		err = r.QueryRow(`SELECT COUNT(*) FROM my_table`).Scan(&rowCount0)
		if err != nil {
			read0ErrCh <- err
			return
		}
		read0ErrCh <- err
	}()
	// Also start a writeTx in paralel that will just read.
	// it should also be blocked until the other one unlocks
	rowCount1 := -1
	startedReading1 := false
	read1ErrCh := make(chan error, 1)
	go func() {
		w1, unlockW1, err := db.Write()
		if err != nil {
			read1ErrCh <- err
			return
		}
		startedReading1 = true
		defer unlockW1()
		err = w1.QueryRow(`SELECT COUNT(*) FROM my_table`).Scan(&rowCount1)
		if err != nil {
			read1ErrCh <- err
			return
		}
		read1ErrCh <- err
	}()

	// Create a test table and insert a row
	_, err = w.Exec(`CREATE TABLE my_table (Name TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Exec(`INSERT INTO my_table (Name) VALUES ("appa")`)
	if err != nil {
		t.Fatal(err)
	}
	err = w.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if startedReading0 || startedReading1 {
		t.Fatalf("readers started before writer was unlocked")
	}
	unlockW()

	// Wait for the reads to finish
	err = <-read0ErrCh
	if err != nil {
		t.Fatal(err)
	}
	if rowCount0 != 1 {
		t.Fatalf("expected rowCount0 1 got %d", rowCount0)
	}
	err = <-read1ErrCh
	if err != nil {
		t.Fatal(err)
	}
	if rowCount1 != 1 {
		t.Fatalf("expected rowCount1 1 got %d", rowCount1)
	}
}

func setupTest(t *testing.T) Db {
	t.Helper()
	db, err := sql.Open("sqlite", "file:memdb1?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewDb(db)
}
