package webdb

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"monorepo/data/sqlitehelper"
)

//go:embed quotamigrations/*.sql
var embeddedQuotaMigrations embed.FS

// Stores the bytes each quota owner is allowed to write and has used.
// Uses its own db, separate from the sqlite db used for everything else.
// No ctx on purpose: quota mirrors physical bytes written to the blobstore,
// which survive rollbacks, so every method commits its own transaction
// immediately instead of joining the caller's. This does open the risk for
// quota to be accounted for but the main transaction (such as creating some
// business data) not being commited.
// E.g.:
// 1 - create a tx at the main db
// 2 - write some data to the blobstore -> quota is accounted for
// 3 - write some data to the main db
// 4 - fail to commit the tx to the main db
//
// -> If these happen, the quota was accounted for but the data ended up being
// discarded.
//
// The above scenario is unlikelly but possible and it kinda sucks. However,
// it prioritizes keeping track of used quota and thus increases the safety
// of our system. If we made the quota tracking part of the same db, we could
// end up in scenarios in which we wrote to the blobstore but failed to commit.
// Since we don't ever delete from the blobstore (it's append only for max
// durability), some user would have spent our disk data but we wouldn't account
// for it.
//
// We can always add more quota for uses to fix that scenario described above (
// in which we "charged" quota but failed to commit the business data), so this
// solution is good for now. We could implement another solution that makes
// quota tracking part of the same tx, but then we'd need some cleanup process
// to delete unused bytes - which is not only complicated but decreases the
// durability of the blobstore bc now we'll be deleting some bytes from it
//
// I'm not too sold on the idea we need this as part of the main db;
// but for now this is good enough so I'll leave it.
type quotaDb struct {
	s sqlitehelper.SqliteHelper
}

func newQuotaDb(pathToDir, dbFileName string) (quotaDb, error) {
	s, err := sqlitehelper.NewSqliteHelper(pathToDir, dbFileName)
	if err != nil {
		return quotaDb{}, err
	}
	return setupQuotaDb(s)
}

func newMemQuotaDb() (quotaDb, error) {
	s, err := sqlitehelper.NewSqliteHelper(sqlitehelper.InMemoryPathToDir, "")
	if err != nil {
		return quotaDb{}, err
	}
	return setupQuotaDb(s)
}

func setupQuotaDb(s sqlitehelper.SqliteHelper) (quotaDb, error) {
	err := s.Init(embeddedQuotaMigrations)
	if err != nil {
		s.Close()
		return quotaDb{}, err
	}
	return quotaDb{s: s}, nil
}

func (q quotaDb) close() {
	q.s.Close()
}

// Returns the total number of bytes the quota owner owns
func (q quotaDb) GetQuota(quotaOwner string) (int64, error) {
	r, closeR, err := q.s.BeginRead()
	if err != nil {
		return 0, err
	}
	defer closeR()
	var nBytes int64
	err = q.s.QueryRow(r, `
		SELECT Bytes FROM quota WHERE QuotaOwner = ?
	`, quotaOwner).Scan(&nBytes)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return nBytes, err
}

// Returns the total bytes used and bytes tried but quota-limited
func (q quotaDb) GetQuotaUsed(quotaOwner string) (success int64, quotaLimitted int64, err error) {
	r, closeR, err := q.s.BeginRead()
	if err != nil {
		return 0, 0, err
	}
	defer closeR()
	err = q.s.QueryRow(r, `
		SELECT SuccessfullBytes, QuotaLimittedBytes FROM usage WHERE QuotaOwner = ?
	`, quotaOwner).Scan(&success, &quotaLimitted)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, nil
	}
	return success, quotaLimitted, err
}

// Returns the number of bytes the quota owner can still write
func (q quotaDb) GetQuotaLeft(quotaOwner string) (int64, error) {
	r, closeR, err := q.s.BeginRead()
	if err != nil {
		return 0, err
	}
	defer closeR()
	var n int64
	err = q.s.QueryRow(r, `
		SELECT
			q.Bytes - IFNULL(u.SuccessfullBytes, 0)
		FROM
			quota AS q
		LEFT JOIN
			usage AS u
		ON
			q.QuotaOwner = u.QuotaOwner
		WHERE
			q.QuotaOwner = ?;
	`, quotaOwner).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return n, err
}

// Adds nBytes to the number of bytes the specified quota owner can write
func (q quotaDb) AddQuota(quotaOwner string, nBytes int64) error {
	if nBytes == 0 {
		return nil
	}
	return q.exec(`
		INSERT INTO quota (QuotaOwner, Bytes)
		VALUES (?, ?)
		ON CONFLICT(QuotaOwner)
		DO UPDATE SET Bytes = quota.Bytes + excluded.Bytes;
	`, quotaOwner, nBytes)
}

// Sets the total number of bytes the specified quota owner can write
func (q quotaDb) SetQuota(quotaOwner string, nBytes int64) error {
	used, _, err := q.GetQuotaUsed(quotaOwner)
	if err != nil {
		return err
	}
	if used > nBytes {
		return fmt.Errorf("tried to set quota bellow usage of %s", quotaOwner)
	}
	current, err := q.GetQuota(quotaOwner)
	if err != nil {
		return err
	}
	return q.AddQuota(quotaOwner, nBytes-current)
}

// Sets the quota to the current usage
func (q quotaDb) FreezeQuota(quotaOwner string) error {
	used, _, err := q.GetQuotaUsed(quotaOwner)
	if err != nil {
		return err
	}
	return q.SetQuota(quotaOwner, used)
}

// Sets the quotaLimitted to zero
func (q quotaDb) ClearQuotaLimitted(quotaOwner string) error {
	return q.exec(`
		UPDATE usage
		SET QuotaLimittedBytes = 0
		WHERE QuotaOwner = ?;
	`, quotaOwner)
}

// IncreaseQuotaLimittedBytes implements blobdb.QuotaDb.
func (q quotaDb) IncreaseQuotaLimittedBytes(quotaOwner string, n int64) error {
	return q.exec(`
		INSERT INTO usage (QuotaOwner, SuccessfullBytes, QuotaLimittedBytes)
		VALUES (?, 0, ?)
		ON CONFLICT(QuotaOwner) DO UPDATE SET
			QuotaLimittedBytes = usage.QuotaLimittedBytes + excluded.QuotaLimittedBytes`,
		quotaOwner, n)
}

// IncreaseSuccessfullBytes implements blobdb.QuotaDb.
func (q quotaDb) IncreaseSuccessfullBytes(quotaOwner string, n int64) error {
	return q.exec(`
		INSERT INTO usage (QuotaOwner, SuccessfullBytes, QuotaLimittedBytes)
		VALUES (?, ?, 0)
		ON CONFLICT(QuotaOwner) DO UPDATE SET
			SuccessfullBytes = usage.SuccessfullBytes + excluded.SuccessfullBytes`,
		quotaOwner, n)
}

// Runs query in its own write transaction, committed immediately, so it
// never depends on (or is undone by) the caller's transaction.
func (q quotaDb) exec(query string, args ...any) error {
	w, closeW, commitW, err := q.s.BeginWrite()
	defer closeW()
	if err != nil {
		return err
	}
	_, err = q.s.Exec(w, query, args...)
	if err != nil {
		return err
	}
	return commitW()
}
