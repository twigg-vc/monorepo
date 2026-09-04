package sqlock

import (
	"database/sql"
	"sync"
)

type db struct {
	mu  *sync.RWMutex
	sdb *sql.DB
}

func newDb(sdb *sql.DB) db {
	return db{sdb: sdb, mu: &sync.RWMutex{}}
}

func (d db) Read() (tx ReadTx, unlock func(), err error) {
	d.mu.RLock()
	defer func() {
		if err != nil {
			d.mu.RUnlock()
		}
	}()
	unlock = func() {}
	sqlTx, err := d.sdb.Begin()
	if err != nil {
		return
	}
	callback := &onceCallback{
		f: func() {
			sqlTx.Rollback()
			d.mu.RUnlock()
		},
	}
	unlock = callback.run
	tx = ReadTx{sqlockTx{sqlTx}}
	return
}

func (d db) Write() (tx WriteTx, unlock func(), err error) {
	d.mu.Lock()
	defer func() {
		if err != nil {
			d.mu.Unlock()
		}
	}()
	unlock = func() {}
	sqlTx, err := d.sdb.Begin()
	if err != nil {
		return
	}
	callback := &onceCallback{
		f: func() {
			sqlTx.Rollback()
			d.mu.Unlock()
		},
	}
	unlock = callback.run
	tx = WriteTx{ReadTx{sqlockTx{sqlTx}}}
	return
}

type sqlockTx struct {
	sqlTx *sql.Tx
}

func (tx sqlockTx) Query(query string, args ...any) (*sql.Rows, error) {
	return tx.sqlTx.Query(query, args...)
}
func (tx sqlockTx) QueryRow(query string, args ...any) *sql.Row {
	return tx.sqlTx.QueryRow(query, args...)
}
func (tx sqlockTx) Exec(query string, args ...any) (sql.Result, error) {
	return tx.sqlTx.Exec(query, args...)
}
func (tx sqlockTx) Commit() error {
	return tx.sqlTx.Commit()
}

type onceCallback struct {
	once sync.Once
	f    func()
}

func (o *onceCallback) run() {
	o.once.Do(o.f)
}
