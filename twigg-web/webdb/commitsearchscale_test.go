package webdb

// Throwaway: shows how a commit search scales with the number of commits.
// Run with:
//   go test ./webdb/ -run Test_SearchScale -v -timeout 30m

import (
	"context"
	"fmt"
	"monorepo/twigg-web/commitsearch"
	"monorepo/twigg/commit"
	"testing"
	"time"
)

const scaleRepoId = 1

func seedScaleCommits(t *testing.T, db WebDb, w context.Context, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		err := db.SetCommit(w, "owner", scaleRepoId, commit.Commit{
			L:            uint64(i),
			AuthorUserId: 1,
			Message:      fmt.Sprintf("fix the handler number %d", i),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func timeSearch(t *testing.T, db WebDb, w context.Context,
	f commitsearch.Filter, cursor string) (time.Duration, int, string) {
	t.Helper()
	start := time.Now()
	commits, nextCursor, err := db.SearchCommits(w, f, cursor, 10)
	if err != nil {
		t.Fatal(err)
	}
	return time.Since(start), len(commits), nextCursor
}

// Disabled bc it's only used for ad-hoc verification
func DISABLED_Test_SearchScale(t *testing.T) {
	sizes := []int{10, 20, 50, 100, 200, 400, 1000, 2000, 4000, 8000}
	t.Logf("%-10s %-12s %-14s %-14s",
		"commits", "seed", "search", "per commit")
	for _, n := range sizes {
		db, closeDb, err := NewMem()
		if err != nil {
			t.Fatal(err)
		}
		w, closeW, _, err := db.BeginWrite()
		if err != nil {
			t.Fatal(err)
		}

		seedStart := time.Now()
		seedScaleCommits(t, db, w, n)
		seed := time.Since(seedStart)

		// Read all results using a text filter
		textFilter := commitsearch.NewFilter(scaleRepoId)
		textFilter.Message = "fix"
		textFilterStart := time.Now()
		textFilterCursor := ""
		for {
			var nResults int
			_, nResults, textFilterCursor = timeSearch(t, db, w, textFilter, textFilterCursor)
			if nResults == 0 {
				break
			}
		}
		textFilterTotal := time.Since(textFilterStart)

		// Log the results
		t.Logf("%-10d %-12s %-14s %-14s", n,
			seed.Round(time.Millisecond),
			textFilterTotal.Round(time.Microsecond),
			(textFilterTotal / time.Duration(n)).Round(time.Microsecond))
		closeW()
		closeDb()
	}
}
