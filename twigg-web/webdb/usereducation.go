package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"monorepo/twigg-web/education"
)

func (db webDb) GetUserEducation(ctx context.Context, userId int64) (education.UserEducation, error) {
	if userId == 0 {
		return education.UserEducation{}, fmt.Errorf("missing userId")
	}
	ed := education.NewUserEducation(userId)
	err := db.s.QueryRow(ctx, `
		SELECT welcomeWasShown FROM user_education WHERE userId = ?
	`, userId).Scan(&ed.WelcomeWasShown)
	if errors.Is(err, sql.ErrNoRows) {
		// Users start with no row: nothing was shown to them yet.
		return ed, nil
	}
	if err != nil {
		return education.UserEducation{}, fmt.Errorf("failed to get user education: %w", err)
	}
	return ed, nil
}

func (db webDb) SetWelcomeWasShown(writeCtx context.Context, userId int64, welcomeWasShown bool) error {
	ed, err := db.GetUserEducation(writeCtx, userId)
	if err != nil {
		return err
	}
	ed.WelcomeWasShown = welcomeWasShown
	return db.SetUserEducation(writeCtx, ed)
}

func (db webDb) SetUserEducation(writeCtx context.Context, ed education.UserEducation) error {
	_, err := db.s.Exec(writeCtx, `
		INSERT INTO user_education (userId, welcomeWasShown)
		VALUES (?, ?)
		ON CONFLICT (userId) DO UPDATE SET
			welcomeWasShown = EXCLUDED.welcomeWasShown
	`, ed.UserId, ed.WelcomeWasShown)
	if err != nil {
		return fmt.Errorf("failed to set user education: %w", err)
	}
	return nil
}