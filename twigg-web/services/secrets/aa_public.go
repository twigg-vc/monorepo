package secrets

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/secrets"
)

type Service interface {
	// Set secret for a specific repoId and name
	SetRepoIdSecret(wl context.Context, repoId uint64, secretName string, secret string) (secrets.SecretRef, error)
	// Get a specific secret by repoId and name.
	GetRepoIdSecret(rl context.Context, repoId uint64, secretName string) (secret string, isNotFoundErr bool, err error)
	// Update specific secret of a repoId.
	UpdateRepoIdSecret(wl context.Context, repoId uint64, secretName string, secret string) (secrets.SecretRef, error)

	// Delete a specific secret by repoId and name if exist. Does nothing and
	// returns nil otherwise.
	DeleteRepoIdSecretIfExists(wl context.Context, repoId uint64, secretName string) error
	// Get page of secrets of repoId. If afterSecretId == 0 -> first page
	GetRepoIdSecretsPage(rl context.Context, repoId uint64, afterSecretId uint64) ([]secrets.SecretRef, error)
	RepoIdHasSecret(rl context.Context, repoId uint64, secretName string) (bool, error)
}

// Database used by the service. It stores only the nonce and the ciphertext,
// never the plaintext secret.
type Db interface {
	HasRepoSecret(ctx context.Context, repoId uint64, secretName string) (bool, error)
	InsertRepoSecret(writeCtx context.Context, repoId uint64,
		secretName string, nonce, encrypted []byte) (secretId uint64, err error)
	UpdateRepoSecret(writeCtx context.Context, repoId uint64,
		secretName string, nonce, encrypted []byte) (secretId uint64, isNotFoundErr bool, err error)
	GetRepoSecretEncrypted(ctx context.Context, repoId uint64,
		secretName string) (nonce, encrypted []byte, isNotFoundErr bool, err error)
	DeleteRepoSecret(writeCtx context.Context, repoId uint64, secretName string) error
	CountRepoSecrets(ctx context.Context, repoId uint64) (int64, error)
	GetRepoSecretsPage(ctx context.Context, repoId uint64,
		afterSecretId uint64, limit int64) (iterator.I[secrets.SecretRef], error)
}

// Constructs a Service instance.
// masterKey must be a 32 byte, base64-encoded value (see ParseMasterKey)
func NewService(db Db, masterKey []byte) (Service, error) {
	return newService(db, masterKey)
}

// Parses and validates the master key.
// The key must be a 32 byte, base64-encoded value
func ParseMasterKey(key string) ([]byte, error) {
	return parseMasterKey(key)
}

const PageSize int64 = 100