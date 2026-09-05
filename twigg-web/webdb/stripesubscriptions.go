package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Writes the stripe subscription row, replacing it if the subscription id is
// already stored.
func (db webDb) UpsertStripeSubscription(writeCtx context.Context,
	stripeSubscriptionId string, userId int64, isActive bool) error {
	if stripeSubscriptionId == "" {
		return fmt.Errorf("missing stripeSubscriptionId")
	}
	_, err := db.s.Exec(writeCtx, `
		INSERT INTO stripe_subscriptions2 (
			stripeSubscriptionId,
			userId,
			isActive
		) VALUES (?, ?, ?)
		ON CONFLICT (stripeSubscriptionId) DO UPDATE SET
			userId = excluded.userId,
			isActive = excluded.isActive;
	`, stripeSubscriptionId, userId, isActive)
	if err != nil {
		return fmt.Errorf("failed to upsert stripe subscription: %w", err)
	}
	return nil
}

// Returns whether the stripe subscription is active. Returns ErrNotFound if
// the subscription is not stored.
func (db webDb) GetStripeSubscriptionIsActive(ctx context.Context,
	stripeSubscriptionId string) (isActive bool, isNotFoundErr bool, err error) {
	if stripeSubscriptionId == "" {
		return false, false, fmt.Errorf("missing stripeSubscriptionId")
	}
	err = db.s.QueryRow(ctx, `
		SELECT isActive
		FROM stripe_subscriptions2
		WHERE stripeSubscriptionId = ?;
	`, stripeSubscriptionId).Scan(&isActive)
	if errors.Is(err, sql.ErrNoRows) {
		return false, true, ErrNotFound
	}
	if err != nil {
		return false, false, fmt.Errorf("failed to query stripe subscription: %w", err)
	}
	return isActive, false, nil
}
