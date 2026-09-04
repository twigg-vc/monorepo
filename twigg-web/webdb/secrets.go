package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/secrets"
)

// Secrets are stored encrypted: the rows hold only the nonce and the
// ciphertext, never the plaintext.

func (db webDb) HasRepoSecret(ctx context.Context, repoId uint64, secretName string) (bool, error) {
	var exists int
	err := db.s.QueryRow(ctx, `
		SELECT 1
		FROM secrets2
		WHERE repo_id = ? AND secret_name = ?
		LIMIT 1
	`, repoId, secretName).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed checking secret existence: %w", err)
	}
	return true, nil
}

func (db webDb) InsertRepoSecret(writeCtx context.Context, repoId uint64,
	secretName string, nonce, encrypted []byte) (secretId uint64, err error) {
	if secretName == "" {
		return 0, fmt.Errorf("missing secretName")
	}
	if len(nonce) == 0 {
		return 0, fmt.Errorf("missing nonce")
	}
	if len(encrypted) == 0 {
		return 0, fmt.Errorf("missing encrypted")
	}
	err = db.s.QueryRow(writeCtx, `
		INSERT INTO secrets2 (repo_id, secret_name, nonce, encrypted)
		VALUES (?, ?, ?, ?)
		RETURNING secret_id;
	`, repoId, secretName, nonce, encrypted).Scan(&secretId)
	if err != nil {
		return 0, fmt.Errorf("failed inserting secret (repoId=%v): %w", repoId, err)
	}
	return secretId, nil
}

func (db webDb) UpdateRepoSecret(writeCtx context.Context, repoId uint64,
	secretName string, nonce, encrypted []byte) (secretId uint64, isNotFoundErr bool, err error) {
	if secretName == "" {
		return 0, false, fmt.Errorf("missing secretName")
	}
	if len(nonce) == 0 {
		return 0, false, fmt.Errorf("missing nonce")
	}
	if len(encrypted) == 0 {
		return 0, false, fmt.Errorf("missing encrypted")
	}
	err = db.s.QueryRow(writeCtx, `
		UPDATE secrets2
		SET nonce = ?, encrypted = ?
		WHERE repo_id = ? AND secret_name = ?
		RETURNING secret_id;
	`, nonce, encrypted, repoId, secretName).Scan(&secretId)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, true, ErrNotFound
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed updating secret (repoId=%v): %w", repoId, err)
	}
	return secretId, false, nil
}

func (db webDb) GetRepoSecretEncrypted(ctx context.Context, repoId uint64,
	secretName string) (nonce, encrypted []byte, isNotFoundErr bool, err error) {
	err = db.s.QueryRow(ctx, `
		SELECT nonce, encrypted
		FROM secrets2
		WHERE repo_id = ? AND secret_name = ?
	`, repoId, secretName).Scan(&nonce, &encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, true, ErrNotFound
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to query secrets: %w", err)
	}
	return nonce, encrypted, false, nil
}

func (db webDb) DeleteRepoSecret(writeCtx context.Context, repoId uint64, secretName string) error {
	if secretName == "" {
		return fmt.Errorf("missing secretName")
	}
	_, err := db.s.Exec(writeCtx, `
		DELETE FROM secrets2
		WHERE repo_id = ? AND secret_name = ?
	`, repoId, secretName)
	if err != nil {
		return fmt.Errorf("failed deleting secret: %w", err)
	}
	return nil
}

func (db webDb) CountRepoSecrets(ctx context.Context, repoId uint64) (int64, error) {
	var count int64
	err := db.s.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM secrets2
		WHERE repo_id = ?
	`, repoId).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed counting secrets: %w", err)
	}
	return count, nil
}

func (db webDb) GetRepoSecretsPage(ctx context.Context, repoId uint64,
	afterSecretId uint64, limit int64) (iterator.I[secrets.SecretRef], error) {
	rows, err := db.s.Query(ctx, `
		SELECT secret_id, secret_name
		FROM secrets2
		WHERE repo_id = ? AND secret_id > ?
		ORDER BY secret_id
		LIMIT ?
	`, repoId, afterSecretId, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo secrets page: %w", err)
	}
	return secretIterWrapper{rows}, nil
}

type secretIterWrapper struct {
	rows *sql.Rows
}

func (it secretIterWrapper) Get() (secrets.SecretRef, error) {
	var s secrets.SecretRef
	err := it.rows.Scan(&s.Id, &s.Name)
	if err != nil {
		return secrets.SecretRef{}, fmt.Errorf("failed to get secret from iter: %s", err)
	}
	return s, nil
}
func (it secretIterWrapper) Next() bool { return it.rows.Next() }
func (it secretIterWrapper) Err() error { return it.rows.Err() }
