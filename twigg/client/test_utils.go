package client

import (
	"fmt"
	"monorepo/twigg/cli/clidb"
	"monorepo/twigg/commit"
	"monorepo/twigg/workdir"
	"testing"
)

// Creates the required instances and commits the root commit
func newTestClient(quotaOwner string, repoId uint64, t *testing.T) (commit.Commit, Client, workdir.TestWorkdir, Write) {
	db, closeDb, err := clidb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)
	l, ul, commitTx, err := db.BeginWrite()
	// When the test ends, commit the lock and check for errors.
	// This ensures that no method is commiting the lock, which should
	// be done only by those who create locks and pass them to silver.
	t.Cleanup(func() {
		err = commitTx()
		if err != nil {
			t.Fatal(err)
		}
		ul()
	})
	if err != nil {
		t.Fatal(err)
	}
	sl := db.Bind(l)

	ag, err := New(quotaOwner, repoId, sl)
	if err != nil {
		t.Fatal(err)
	}
	if ag.IsInit() {
		t.Fatal("expected not to be initted yet")
	}

	wd := workdir.NewTest(fmt.Sprintf("repo-%d_wd", repoId), t)
	root, err := ag.Init(sl)
	if err != nil {
		t.Fatal(err)
	}
	if !ag.IsInit() {
		t.Fatal("expected to be innitted")
	}
	return root, ag, wd, sl
}
