package secrets

import (
	"bytes"
	"fmt"
	"monorepo/twigg-web/secrets"
	"monorepo/twigg-web/webdb"
	"reflect"
	"testing"
)

func TestGetAndSetRepoIdSecret_EmptyName_NegativeRepoId_EmptySecret(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	// deterministic master key
	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty secret name
	_, _, err = svc.GetRepoIdSecret(wl, 10, "")
	if err == nil {
		t.Fatalf("expected error for empty secret name, got nil")
	}
	_, err = svc.SetRepoIdSecret(wl, 10, "", "key-to-the-city")
	if err == nil {
		t.Fatalf("expected error for empty secret name, got nil")
	}

	// Empty Secret
	_, err = svc.SetRepoIdSecret(wl, 10, "my-secret", "")
	if err == nil {
		t.Fatalf("expected error for empty secret, got nil")
	}
}

func TestSetRepoIdSecret_MaxSecretSize(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	// deterministic master key
	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Large Secret
	secret := bytes.Repeat([]byte{1}, maxSecretBytes+1)
	_, err = svc.SetRepoIdSecret(wl, 10, "my-insanely-large-secret", string(secret))
	if err == nil {
		t.Fatalf("expected error for large secret, got nil")
	}
	// Large Secret Name
	secretName := bytes.Repeat([]byte{1}, secretNameMaxLen+1)
	_, err = svc.SetRepoIdSecret(wl, 10, string(secretName), "valid-secret")
	if err == nil {
		t.Fatalf("expected error for large secret name, got nil")
	}
}

func TestSetRepoIdSecret_RepoReachedSecretsLimit(t *testing.T) {
	originalMaxNumOfSecretsPerRepo := maxNumOfSecretsPerRepo
	maxNumOfSecretsPerRepo = 5
	t.Cleanup(func() { maxNumOfSecretsPerRepo = originalMaxNumOfSecretsPerRepo })
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	// deterministic master key
	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repoID := uint64(77)
	// Fill repo with maxNumOfSecretsPerRepo secrets
	for i := 0; i < maxNumOfSecretsPerRepo; i++ {
		name := fmt.Sprintf("secret-%d", i)
		_, err := svc.SetRepoIdSecret(wl, repoID, name, "value")
		if err != nil {
			t.Fatalf("unexpected error inserting %q: %v", name, err)
		}
	}

	// Next insert should fail
	_, err = svc.SetRepoIdSecret(wl, repoID, "overflow-secret", "value")
	if err == nil {
		t.Fatalf("expected error when repo reaches secret limit")
	}

	// Updating an existing secret should still work
	_, err = svc.SetRepoIdSecret(wl, repoID, "secret-0", "new-value")
	if err != nil {
		t.Fatalf("update should succeed even if repo is full: %v", err)
	}

	// Delete first
	err = svc.DeleteRepoIdSecretIfExists(wl, repoID, "secret-0")
	if err != nil {
		t.Fatalf("DeleteRepoIdSecret failed unexpectedly: %v", err)
	}
	// Next insert should work
	_, err = svc.SetRepoIdSecret(wl, repoID, "i-can-keep-another-secret", "value")
	if err != nil {
		t.Fatalf("unexpected error inserting after delete: %q", err)
	}
}

func TestGetRepoIdSecret_NotFound(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	// deterministic master key
	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, isNotFoundErr, err := svc.GetRepoIdSecret(wl, 10, "missing")

	if err == nil {
		t.Fatalf("expected not found error. got nil")
	}
	if !isNotFoundErr {
		t.Fatalf("expected isNotFoundErr=true, got false")
	}
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestSetAndGetRepoIdSecret(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	// deterministic master key
	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repoID := uint64(10)
	secretName := "door-on-the-third-floor"
	secret := "Alohomora"

	// Set
	s, err := svc.SetRepoIdSecret(wl, repoID, secretName, secret)
	if err != nil {
		t.Fatalf("failed inserting secret: %v", err)
	}
	if s.Id != 1 {
		t.Fatalf("expected secret.Id in setRepoIdSecret 1, got %d", s.Id)
	}
	if s.Name != secretName {
		t.Fatalf("expected secret.Name in setRepoIdSecret %q, got %q", secretName, s.Name)
	}

	// Get
	got, isNotFoundErr, err := svc.GetRepoIdSecret(wl, repoID, secretName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isNotFoundErr {
		t.Fatalf("expected isNotFoundErr=false, got true")
	}
	if got != secret {
		t.Fatalf("expected %q, got %q", secret, got)
	}
}

func TestSetAndHasRepoIdSecret(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	// deterministic master key
	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repoID := uint64(10)
	secretName := "Erebor"
	secret := "key-to-the-side-door"

	// Set
	_, err = svc.SetRepoIdSecret(wl, repoID, secretName, secret)
	if err != nil {
		t.Fatalf("failed inserting secret: %v", err)
	}

	// Has
	has, err := svc.RepoIdHasSecret(wl, repoID, secretName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !has {
		t.Fatalf("expected has=true, got false")
	}

	// Does not have
	// wrong repoId
	has, err = svc.RepoIdHasSecret(wl, 100, secretName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if has {
		t.Fatalf("expected has=false, got true")
	}
	// wrong name
	has, err = svc.RepoIdHasSecret(wl, repoID, "side-door")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if has {
		t.Fatalf("expected has=false, got true")
	}
}

func TestDeleteRepoIdSecret(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	// deterministic master key
	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repoID := uint64(15)
	secretName := "Atlantis-Key"
	secretValue := "Trident-of-Poseidon"

	// Set secret
	_, err = svc.SetRepoIdSecret(wl, repoID, secretName, secretValue)
	if err != nil {
		t.Fatalf("Failed to set secret err: %v", err)
	}

	// Deletion secret
	err = svc.DeleteRepoIdSecretIfExists(wl, repoID, secretName)
	if err != nil {
		t.Fatalf("DeleteRepoIdSecret failed unexpectedly: %v", err)
	}

	// Check secret (has should be false)
	has, err := svc.RepoIdHasSecret(wl, repoID, secretName)
	if err != nil {
		t.Fatalf("RepoIdHasSecret failed: %v", err)
	}
	if has {
		t.Fatal("secret still exists after deletion")
	}

	// Deleting the same secret again should not return an error
	err = svc.DeleteRepoIdSecretIfExists(wl, repoID, secretName)
	if err != nil {
		t.Fatalf("Expected nil error when deleting non-existent secret, got: %v", err)
	}

	// Delete a secret that never existed
	err = svc.DeleteRepoIdSecretIfExists(wl, 999, "ImaginarySecret")
	if err != nil {
		t.Fatalf("Expected nil error when deleting imaginary secret, got: %v", err)
	}
}

func TestSetRepoIdSecret_UpdateExisting(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	// deterministic master key
	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repoID := uint64(42)
	secretName := "SecretOfLife"

	// First value
	secret1 := "CircleOfLife"
	_, err = svc.SetRepoIdSecret(wl, repoID, secretName, secret1)
	if err != nil {
		t.Fatalf("failed inserting secret: %v", err)
	}

	// Overwrite with a second value
	secret2 := "Hakuna Matata"
	s2, err := svc.SetRepoIdSecret(wl, repoID, secretName, secret2)
	if err != nil {
		t.Fatalf("failed updating secret: %v", err)
	}
	if s2.Name != secretName {
		t.Fatalf("got s2.Name: %v, expected: %v", s2.Name, secretName)
	}

	// Get and verify updated value
	got, isNotFoundErr, err := svc.GetRepoIdSecret(wl, repoID, secretName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isNotFoundErr {
		t.Fatalf("expected isNotFoundErr=false, got true")
	}

	if got != secret2 {
		t.Fatalf("expected updated value %q, got %q", secret2, got)
	}
}
func TestUpdateRepoIdSecret_Success(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	// deterministic master key
	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repoID := uint64(7)
	secretName := "ancient-scroll"

	// initial secret
	sInitial, err := svc.SetRepoIdSecret(wl, repoID, secretName, "first-value")
	if err != nil {
		t.Fatalf("failed to set secret: %v", err)
	}

	// update
	sUpdated, err := svc.UpdateRepoIdSecret(wl, repoID, secretName, "updated-value")
	if err != nil {
		t.Fatalf("failed to update secret: %v", err)
	}
	if !reflect.DeepEqual(sInitial, sUpdated) {
		t.Fatalf("secret struct should satay the same, only secret value must change. sInitial=%#v sUpdated=%#v", sInitial, sUpdated)
	}

	// verify
	got, isNotFoundErr, err := svc.GetRepoIdSecret(wl, repoID, secretName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isNotFoundErr {
		t.Fatalf("expected isNotFoundErr=false, got true")
	}
	if got != "updated-value" {
		t.Fatalf("expected updated-value, got %q", got)
	}
}
func TestUpdateRepoIdSecret_NotFound(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	// deterministic master keys
	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.UpdateRepoIdSecret(wl, 99, "missing-secret", "value")
	if err == nil {
		t.Fatalf("expected error when updating non-existent secret, got nil")
	}
}
func TestUpdateRepoIdSecret_InvalidInputs(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	// deterministic master keys
	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// empty name
	_, err = svc.UpdateRepoIdSecret(wl, 1, "", "y")
	if err == nil {
		t.Fatalf("expected error for empty secretName")
	}

	// empty secret
	_, err = svc.UpdateRepoIdSecret(wl, 1, "x", "")
	if err == nil {
		t.Fatalf("expected error for empty secret")
	}

	// Large Secret
	tooLarge := bytes.Repeat([]byte{1}, maxSecretBytes+1)
	_, err = svc.UpdateRepoIdSecret(wl, 1, "big-secret", string(tooLarge))
	if err == nil {
		t.Fatalf("expected error for oversized secret")
	}
	// Large Secret Name
	secretName := bytes.Repeat([]byte{1}, secretNameMaxLen+1)
	_, err = svc.UpdateRepoIdSecret(wl, 10, string(secretName), "valid-secret")
	if err == nil {
		t.Fatalf("expected error for large secret name, got nil")
	}
}

func TestGetRepoIdSecretsPage_EmptyRepo(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secrets, err := svc.GetRepoIdSecretsPage(wl, 42, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(secrets) != 0 {
		t.Fatalf("expected empty result, got %v", secrets)
	}
	if secrets == nil {
		t.Fatalf("expected not nil result, got nil")
	}
}

func TestGetRepoIdSecretsPage_FirstPage(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repoID := uint64(10)

	expectedSecrets := []secrets.SecretRef{
		{
			Id:   1,
			Name: "alpha",
		},
		{
			Id:   2,
			Name: "beta",
		},
		{
			Id:   3,
			Name: "gamma",
		},
	}
	for _, s := range expectedSecrets {
		_, err = svc.SetRepoIdSecret(wl, repoID, s.Name, "value")
		if err != nil {
			t.Fatalf("failed inserting %q: %v", s.Name, err)
		}
	}

	// Get fist page.
	gotSecrets, err := svc.GetRepoIdSecretsPage(wl, repoID, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(expectedSecrets, gotSecrets) {
		t.Fatalf("expected %#v, got %#v", expectedSecrets, gotSecrets)
	}
	// Get after fist secret
	expectedSecrets = []secrets.SecretRef{
		{
			Id:   2,
			Name: "beta",
		},
		{
			Id:   3,
			Name: "gamma",
		},
	}
	gotSecrets, err = svc.GetRepoIdSecretsPage(wl, repoID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(expectedSecrets, gotSecrets) {
		t.Fatalf("expected %#v, got %#v", expectedSecrets, gotSecrets)
	}
}

func TestGetRepoIdSecretsPage_RepoIsolation(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	wl, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	masterKey := bytes.Repeat([]byte{1}, 32)
	svc, err := NewService(b, masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add 2 secret to 2 repo different repos
	_, err = svc.SetRepoIdSecret(wl, 100, "repo100-secret-1", "x") // Id 1
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.SetRepoIdSecret(wl, 200, "repo200-secret-1", "y") // Id 2
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.SetRepoIdSecret(wl, 100, "repo100-secret-2", "z") // Id 3
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.SetRepoIdSecret(wl, 200, "repo200-secret-2", "h") // Id 4
	if err != nil {
		t.Fatal(err)
	}

	// Repo 100
	expectedRepo100Secrets := []secrets.SecretRef{
		{Id: 1, Name: "repo100-secret-1"},
		{Id: 3, Name: "repo100-secret-2"},
	}
	gotRepo100Secrets, err := svc.GetRepoIdSecretsPage(wl, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(expectedRepo100Secrets, gotRepo100Secrets) {
		t.Fatalf("unexpected secrets for repo 1: %#v", gotRepo100Secrets)
	}

	// Repo 200
	expectedRepo200Secrets := []secrets.SecretRef{
		{Id: 2, Name: "repo200-secret-1"},
		{Id: 4, Name: "repo200-secret-2"},
	}
	gotRepo200Secrets, err := svc.GetRepoIdSecretsPage(wl, 200, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(expectedRepo200Secrets, gotRepo200Secrets) {
		t.Fatalf("unexpected secrets for repo 1: %#v", gotRepo200Secrets)
	}
}