package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"monorepo/base/iterator"
	"monorepo/twigg-web/secrets"
)

type service struct {
	db   Db
	aead cipher.AEAD
}

const maxSecretBytes = 64 * 1024 // 64 KiB

const secretNameMaxLen = 100

var maxNumOfSecretsPerRepo = 200

func newService(db Db, masterKey []byte) (Service, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("masterKey must be 32 bytes for AES-256")
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create Cipher. got err=%s", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM. err=%s", err)
	}

	return &service{
		db:   db,
		aead: aead,
	}, nil
}

func (s *service) RepoIdHasSecret(rl context.Context,
	repoId uint64,
	secretName string,
) (bool, error) {
	return s.db.HasRepoSecret(rl, repoId, secretName)
}

func (s *service) SetRepoIdSecret(
	wl context.Context,
	repoId uint64,
	secretName string,
	secret string,
) (secrets.SecretRef, error) {
	return s.setRepoIdSecret(true, wl, repoId, secretName, secret)
}

func (s *service) UpdateRepoIdSecret(
	wl context.Context,
	repoId uint64,
	secretName string,
	secret string,
) (secrets.SecretRef, error) {
	return s.setRepoIdSecret(false, wl, repoId, secretName, secret)
}

func (s *service) setRepoIdSecret(
	createIfNotExists bool,
	wl context.Context,
	repoId uint64,
	secretName string,
	secret string,
) (secrets.SecretRef, error) {
	isValidSecretName, err := isValidSecretName(secretName)
	if !isValidSecretName {
		return secrets.SecretRef{}, fmt.Errorf("invalid secret name in setRepoIdSecret, err=%q", err)
	}
	if secret == "" {
		return secrets.SecretRef{}, errors.New("can not set a repo secret with empty secret")
	}
	isValidSecret, err := isValidSecret(secret)
	if !isValidSecret {
		return secrets.SecretRef{}, err
	}
	alreadyHasSecret, err := s.db.HasRepoSecret(wl, repoId, secretName)
	if err != nil {
		return secrets.SecretRef{}, fmt.Errorf("in SetRepoIdSecret got err=%s checking if repo has secret", err)
	}
	if !alreadyHasSecret && !createIfNotExists {
		return secrets.SecretRef{}, fmt.Errorf(
			"can not update secret because it does not exist (repoId=%d, secretName=%s)",
			repoId, secretName)
	}

	nonce := make([]byte, s.aead.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return secrets.SecretRef{}, fmt.Errorf("error reding nonce err=%s", err)
	}
	ciphertext := s.aead.Seal(nil, nonce, []byte(secret), nil)
	var secretId uint64
	if alreadyHasSecret {
		secretId, _, err = s.db.UpdateRepoSecret(wl, repoId, secretName, nonce, ciphertext)
		if err != nil {
			return secrets.SecretRef{}, fmt.Errorf("failed updating secret (repoId=%v). err=%s", repoId, err)
		}
	} else {
		hasReachedLimit, err := s.repoReachedSecretsLimit(wl, repoId)
		if err != nil {
			return secrets.SecretRef{}, err
		}
		if hasReachedLimit {
			return secrets.SecretRef{}, fmt.Errorf("repo reached maximum number of secrets (%d)", maxNumOfSecretsPerRepo)
		}
		secretId, err = s.db.InsertRepoSecret(wl, repoId, secretName, nonce, ciphertext)
		if err != nil {
			return secrets.SecretRef{}, fmt.Errorf("failed inserting secret (repoId=%v). err=%s", repoId, err)
		}
	}

	return secrets.SecretRef{Id: secretId, Name: secretName}, nil
}

func (s *service) GetRepoIdSecret(
	rl context.Context,
	repoId uint64,
	secretName string,
) (string, bool, error) {
	nonce, encrypted, isNotFoundErr, err := s.db.GetRepoSecretEncrypted(rl, repoId, secretName)
	if isNotFoundErr {
		return "", true, err
	}
	if err != nil {
		return "", false, fmt.Errorf("failed to query secrets. err=%s", err)
	}

	plaintext, err := s.aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", false, fmt.Errorf("failed to decrypt secret. err=%s", err)
	}

	return string(plaintext), false, nil
}

func (s *service) DeleteRepoIdSecretIfExists(wl context.Context,
	repoId uint64,
	secretName string,
) error {
	if secretName == "" {
		return errors.New("can not delete a repo secret with empty secretName")
	}

	hasSecret, err := s.db.HasRepoSecret(wl, repoId, secretName)
	if err != nil {
		return err
	}
	if !hasSecret {
		return nil
	}

	return s.db.DeleteRepoSecret(wl, repoId, secretName)
}

func (s *service) GetRepoIdSecretsPage(
	rl context.Context,
	repoId uint64,
	afterSecretId uint64, // 0 == first page
) ([]secrets.SecretRef, error) {
	it, err := s.db.GetRepoSecretsPage(rl, repoId, afterSecretId, PageSize)
	if err != nil {
		return nil, fmt.Errorf("GetRepoIdSecretsPage: query failed: %w", err)
	}
	result, err := iterator.GetFirstN(int(PageSize), it)
	if err != nil {
		return nil, fmt.Errorf("GetRepoIdSecretsPage: iteration failed: %w", err)
	}
	return result, nil
}

// if false then error != nil
func isValidSecret(secret string) (bool, error) {
	if secret == "" {
		return false, errors.New("empty secret")
	}
	if len(secret) > maxSecretBytes {
		return false, fmt.Errorf("secret size=(%d bytes) exceeds maximum allowed size=(%d bytes)", len(secret), maxSecretBytes)
	}
	return true, nil
}

// if false then error != nil
func isValidSecretName(secretName string) (bool, error) {
	if secretName == "" {
		return false, errors.New("empty secretName")
	}
	if len(secretName) > secretNameMaxLen {
		return false, fmt.Errorf("secretName len=%d exceeds maximum allowed len of %d)", len(secretName), secretNameMaxLen)
	}
	return true, nil
}

func (s *service) repoReachedSecretsLimit(rl context.Context, repoId uint64) (bool, error) {
	count, err := s.db.CountRepoSecrets(rl, repoId)
	if err != nil {
		return false, fmt.Errorf("failed checking secret limit: %w", err)
	}
	return count >= int64(maxNumOfSecretsPerRepo), nil
}