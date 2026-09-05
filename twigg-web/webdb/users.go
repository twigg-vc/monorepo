package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/user"
	"strconv"
)

// Writes every column of the user row and returns the stored user. A zero id
// inserts a new row. The quota fields are not written: they live in the quota
// db, so they are read back into the returned user.
func (db webDb) UpsertUser(writeCtx context.Context,
	u user.User) (stored user.User, err error) {
	// The column is NOT NULL in the table but no longer used.
	const deprecatedSeatsInUse = 0
	err = db.s.QueryRow(writeCtx, `
		INSERT INTO users2 (
			id,
			email,
			state,
			isOrganization,
			stripeId,
			cliKeyHash,
			username,
			passwordHash,
			selfPaidSubscription,
			selfPaidSubscriptionQuantity,
			selfPaidSubscriptionSeatsInUse,
			stripeSessionId,
			stripeSessionUrl,
			stripeSessionPriceId,
			stripeSessionQuantity,
			stripeSubscriptionID
		) VALUES (NULLIF(?, 0), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			email = excluded.email,
			state = excluded.state,
			isOrganization = excluded.isOrganization,
			stripeId = excluded.stripeId,
			cliKeyHash = excluded.cliKeyHash,
			username = excluded.username,
			passwordHash = excluded.passwordHash,
			selfPaidSubscription = excluded.selfPaidSubscription,
			selfPaidSubscriptionQuantity = excluded.selfPaidSubscriptionQuantity,
			stripeSessionId = excluded.stripeSessionId,
			stripeSessionUrl = excluded.stripeSessionUrl,
			stripeSessionPriceId = excluded.stripeSessionPriceId,
			stripeSessionQuantity = excluded.stripeSessionQuantity,
			stripeSubscriptionID = excluded.stripeSubscriptionID
		RETURNING id;
	`,
		u.Id, u.Email, u.State, u.IsOrganization, u.StripeId, u.CliKeyHash,
		u.Username, u.PasswordHash, u.SelfPaidSubscription,
		u.SelfPaidSubscriptionQuantity, deprecatedSeatsInUse,
		u.StripeSessionId, u.StripeSessionUrl, u.StripeSessionPriceId,
		u.StripeSessionQuantity, u.StripeSubscriptionID,
	).Scan(&u.Id)
	if err != nil {
		return user.User{}, fmt.Errorf("failed to upsert user: %w", err)
	}
	err = db.readUserQuota(&u)
	if err != nil {
		return user.User{}, err
	}
	return u, nil
}

const selectUserColumns = `
	SELECT
		id,
		email,
		state,
		isOrganization,
		stripeId,
		cliKeyHash,
		username,
		passwordHash,
		selfPaidSubscription,
		selfPaidSubscriptionQuantity,
		stripeSessionId,
		stripeSessionUrl,
		stripeSessionPriceId,
		stripeSessionQuantity,
		stripeSubscriptionID
	FROM users2`

// Satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (u user.User, err error) {
	err = row.Scan(
		&u.Id,
		&u.Email,
		&u.State,
		&u.IsOrganization,
		&u.StripeId,
		&u.CliKeyHash,
		&u.Username,
		&u.PasswordHash,
		&u.SelfPaidSubscription,
		&u.SelfPaidSubscriptionQuantity,
		&u.StripeSessionId,
		&u.StripeSessionUrl,
		&u.StripeSessionPriceId,
		&u.StripeSessionQuantity,
		&u.StripeSubscriptionID,
	)
	return u, err
}

func (db webDb) GetUser(ctx context.Context,
	userId int64) (u user.User, isNotFoundErr bool, err error) {
	return db.getUserWhere(ctx, "id = ?", userId)
}

func (db webDb) GetUserByEmail(ctx context.Context,
	email string) (u user.User, isNotFoundErr bool, err error) {
	return db.getUserWhere(ctx, "email = ?", email)
}

func (db webDb) GetUserByUsername(ctx context.Context,
	username string) (u user.User, isNotFoundErr bool, err error) {
	return db.getUserWhere(ctx, "username = ?", username)
}

func (db webDb) GetUserByStripeId(ctx context.Context,
	stripeId string) (u user.User, isNotFoundErr bool, err error) {
	return db.getUserWhere(ctx, "stripeId = ?", stripeId)
}

func (db webDb) GetUserByCliKeyHash(ctx context.Context,
	cliKeyHash string) (u user.User, isNotFoundErr bool, err error) {
	return db.getUserWhere(ctx, "cliKeyHash = ?", cliKeyHash)
}

// Reads only the username, so the callers that just label a user id don't pay
// for the rest of the row.
func (db webDb) GetUsername(ctx context.Context,
	userId int64) (username string, isNotFoundErr bool, err error) {
	err = db.s.QueryRow(ctx, `
		SELECT username
		FROM users2
		WHERE id = ?;
	`, userId).Scan(&username)
	if errors.Is(err, sql.ErrNoRows) {
		return "", true, ErrNotFound
	}
	if err != nil {
		return "", false, fmt.Errorf("failed to query username: %w", err)
	}
	return username, false, nil
}

func (db webDb) getUserWhere(ctx context.Context, whereClause string,
	arg any) (u user.User, isNotFoundErr bool, err error) {
	u, err = scanUser(db.s.QueryRow(ctx, fmt.Sprintf(`
		%s
		WHERE %s;
	`, selectUserColumns, whereClause), arg))
	if errors.Is(err, sql.ErrNoRows) {
		return user.User{}, true, ErrNotFound
	}
	if err != nil {
		return user.User{}, false, fmt.Errorf("failed to query user: %w", err)
	}
	err = db.readUserQuota(&u)
	if err != nil {
		return user.User{}, false, err
	}
	return u, false, nil
}

// Fills in the fields kept in the quota db instead of the users table.
func (db webDb) readUserQuota(u *user.User) error {
	owner := db.UserQuotaOwnerName(u.Id)
	total, err := db.quota.GetQuota(owner)
	if err != nil {
		return fmt.Errorf("failed to get user quota: %w", err)
	}
	used, limitted, err := db.quota.GetQuotaUsed(owner)
	if err != nil {
		return fmt.Errorf("failed to get used user quota: %w", err)
	}
	u.TotalQuota = total
	u.QuotaUsed = used
	u.QuotaLimmitted = limitted
	return nil
}

func (db webDb) UserQuotaOwnerName(userId int64) string {
	return strconv.FormatInt(userId, 10)
}

func (db webDb) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := db.s.QueryRow(ctx, `
		SELECT COUNT(*) FROM users2;
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

func (db webDb) GetAllUsers(ctx context.Context) (iterator.I[user.User], error) {
	rows, err := db.s.Query(ctx, fmt.Sprintf(`
		%s
		ORDER BY id DESC;
	`, selectUserColumns))
	if err != nil {
		return nil, fmt.Errorf("failed to query all users: %w", err)
	}
	return userIterWrapper{db, rows}, nil
}

type userIterWrapper struct {
	db   webDb
	rows *sql.Rows
}

func (it userIterWrapper) Get() (user.User, error) {
	u, err := scanUser(it.rows)
	if err != nil {
		return user.User{}, fmt.Errorf("failed to get user from iter: %w", err)
	}
	err = it.db.readUserQuota(&u)
	if err != nil {
		return user.User{}, err
	}
	return u, nil
}
func (it userIterWrapper) Next() bool { return it.rows.Next() }
func (it userIterWrapper) Err() error { return it.rows.Err() }
