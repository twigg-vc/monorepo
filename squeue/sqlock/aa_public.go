package sqlock

import (
	"database/sql"
)

// Wrapper around a sql database to ensure single-writer multi-reader.
// The use case is to avoid "DB is locked" errors with SQLite
type Db struct {
	db db
}

// Returns a new instance
func NewDb(sqlDb *sql.DB) Db {
	return Db{db: newDb(sqlDb)}
}

// Get a transaction for reading.
// Unlock must ALWAYS be called if a non-nil err is returned,
// and can safelly be called many times.
func (d Db) Read() (tx ReadTx, unlock func(), err error) {
	return d.db.Read()
}

// Get a transaction for writing.
// Unlock must ALWAYS be called if a non-nil err is returned,
// and can safelly be called many times.
func (d Db) Write() (tx WriteTx, unlock func(), err error) {
	return d.db.Write()
}

type ReadTx struct {
	tx sqlockTx
}

func (tx ReadTx) Query(query string, args ...any) (*sql.Rows, error) {
	return tx.tx.Query(query, args...)
}
func (tx ReadTx) QueryRow(query string, args ...any) *sql.Row {
	return tx.tx.QueryRow(query, args...)
}

type WriteTx struct {
	ReadTx
}

func (tx WriteTx) Exec(query string, args ...any) (sql.Result, error) {
	return tx.ReadTx.tx.Exec(query, args...)
}
func (tx WriteTx) Commit() error {
	return tx.ReadTx.tx.Commit()
}
