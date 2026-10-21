package webdb

import "testing"

func Test_CommitSearchMigration_RemovesHistoricalText(t *testing.T) {
	db, w := newIndexTestDb(t)
	// Recreate the old index: three versions in repo 7 and one in repo 8.
	statements := []string{
		`INSERT INTO twigg_commit_search
			(repoId, commitId, commitVersion, authorId, isSubmitted,
			createdOnUnixMilli, isWip, isArchived)
			VALUES (7, 1, 2, 3, 1, 0, 0, 0), (8, 1, 0, 3, 0, 0, 0, 0)`,
		`INSERT INTO commit_search_text
			(rowid, message, repoId, commitId, commitVersion) VALUES
			(1, 'flag obsolete', 7, 1, 0),
			(2, 'flag obsolete', 7, 1, 1),
			(3, 'flag latest', 7, 1, 2),
			(4, 'flag otherrepo', 8, 1, 0)`,
	}
	for _, statement := range statements {
		if _, err := db.db.s.Exec(w, statement); err != nil {
			t.Fatal(err)
		}
	}

	migration, err := embeddedMigrations.ReadFile("migrations/0000026_commit_search_latest_text.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.s.Exec(w, string(migration)); err != nil {
		t.Fatal(err)
	}

	// Only the latest messages remain searchable, with their original rowids.
	var got string
	err = db.db.s.QueryRow(w, `SELECT group_concat(rowid) FROM (
		SELECT rowid FROM commit_search_text
		WHERE commit_search_text MATCH 'flag' ORDER BY rowid
	)`).Scan(&got)
	if err != nil {
		t.Fatal(err)
	}
	if got != "3,4" {
		t.Fatalf("matching rowids are %q, want 3,4", got)
	}
}
