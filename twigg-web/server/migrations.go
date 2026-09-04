package server

import (
	"fmt"
	"monorepo/twigg-web/webdb"
)

// runGoMigrations is used to run manual non-sql migrations (i.e. "migrations
// that we actually run in go code")
func runGoMigrations(db webdb.WebDb) {
	_, closeDbWrite, commitDbWrite, err := db.BeginWrite()
	defer closeDbWrite()
	if err != nil {
		panic("unable to get tx to run migrations: " + err.Error())
	}

	// TODO:
	// This place is reserved for running non sql migrations - i.e. "migrations"
	// that we run in go. Currently there are none bc the ones that existed were
	// already removed, but this is where we'll put next ones when we need.
	// See https://twigg.vc/andre/monorepo/c/1461 for ref

	err = commitDbWrite()
	if err != nil {
		panic(fmt.Sprintf("failed to commit migrations tx :%s", err))
	}
}
