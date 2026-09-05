package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

func (db webDb) GetUser(ctx context.Context,
	userId int64) (u user.User, isNotFoundErr bool, err error) {
	err = db.s.QueryRow(ctx, `
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
		FROM users2
		WHERE id = ?;
	`, userId).Scan(
		&u.Id, &u.Email, &u.State, &u.IsOrganization, &u.StripeId,
		&u.CliKeyHash, &u.Username, &u.PasswordHash, &u.SelfPaidSubscription,
		&u.SelfPaidSubscriptionQuantity, &u.StripeSessionId,
		&u.StripeSessionUrl, &u.StripeSessionPriceId, &u.StripeSessionQuantity,
		&u.StripeSubscriptionID,
	)
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
	owner := userQuotaOwner(u.Id)
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

func userQuotaOwner(userId int64) string {
	return strconv.FormatInt(userId, 10)
}
