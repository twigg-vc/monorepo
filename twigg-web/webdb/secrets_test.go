package webdb_test

import (
	"bytes"
	"errors"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/secrets"
	"monorepo/twigg-web/webdb"
	"reflect"
	"testing"
)

func TestInsertAndGetRepoSecret(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const repoId = uint64(10)
	nonce := []byte("nonce-1")
	encrypted := []byte("ciphertext-1")

	id, err := b.InsertRepoSecret(w, repoId, "my-secret", nonce, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Fatalf("expected first secret id 1, got %d", id)
	}

	gotNonce, gotEncrypted, isNotFoundErr, err := b.GetRepoSecretEncrypted(w, repoId, "my-secret")
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("expected isNotFoundErr=false, got true")
	}
	if !bytes.Equal(gotNonce, nonce) {
		t.Fatalf("expected nonce %q, got %q", nonce, gotNonce)
	}
	if !bytes.Equal(gotEncrypted, encrypted) {
		t.Fatalf("expected encrypted %q, got %q", encrypted, gotEncrypted)
	}

	// Missing secret
	_, _, isNotFoundErr, err = b.GetRepoSecretEncrypted(w, repoId, "missing")
	if !isNotFoundErr {
		t.Fatal("expected isNotFoundErr=true, got false")
	}
	if !errors.Is(err, webdb.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestInsertRepoSecretDuplicateName(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	_, err = b.InsertRepoSecret(w, 10, "dup", []byte("n"), []byte("e"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.InsertRepoSecret(w, 10, "dup", []byte("n2"), []byte("e2"))
	if err == nil {
		t.Fatal("expected error inserting duplicate secret name, got nil")
	}
	// Same name on another repo is fine
	_, err = b.InsertRepoSecret(w, 11, "dup", []byte("n3"), []byte("e3"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateRepoSecret(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const repoId = uint64(10)
	insertedId, err := b.InsertRepoSecret(w, repoId, "my-secret", []byte("n1"), []byte("e1"))
	if err != nil {
		t.Fatal(err)
	}

	updatedId, isNotFoundErr, err := b.UpdateRepoSecret(w, repoId, "my-secret", []byte("n2"), []byte("e2"))
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("expected isNotFoundErr=false, got true")
	}
	if updatedId != insertedId {
		t.Fatalf("update should keep the secret id, expected %d, got %d", insertedId, updatedId)
	}

	gotNonce, gotEncrypted, _, err := b.GetRepoSecretEncrypted(w, repoId, "my-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotNonce, []byte("n2")) || !bytes.Equal(gotEncrypted, []byte("e2")) {
		t.Fatalf("expected updated nonce/encrypted, got %q/%q", gotNonce, gotEncrypted)
	}

	// Updating a missing secret
	_, isNotFoundErr, err = b.UpdateRepoSecret(w, repoId, "missing", []byte("n"), []byte("e"))
	if !isNotFoundErr {
		t.Fatal("expected isNotFoundErr=true, got false")
	}
	if !errors.Is(err, webdb.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestHasAndDeleteRepoSecret(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const repoId = uint64(10)
	_, err = b.InsertRepoSecret(w, repoId, "my-secret", []byte("n"), []byte("e"))
	if err != nil {
		t.Fatal(err)
	}

	has, err := b.HasRepoSecret(w, repoId, "my-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("expected has=true, got false")
	}
	// Wrong repo and wrong name
	has, err = b.HasRepoSecret(w, repoId+1, "my-secret")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("other repo should not have the secret")
	}
	has, err = b.HasRepoSecret(w, repoId, "other-secret")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("other name should not exist")
	}

	if err := b.DeleteRepoSecret(w, repoId, "my-secret"); err != nil {
		t.Fatal(err)
	}
	has, err = b.HasRepoSecret(w, repoId, "my-secret")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("secret still exists after deletion")
	}
	// Deleting again is a no-op
	if err := b.DeleteRepoSecret(w, repoId, "my-secret"); err != nil {
		t.Fatal(err)
	}
}

func TestCountRepoSecrets(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	count, err := b.CountRepoSecrets(w, 10)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 secrets, got %d", count)
	}

	for i := 0; i < 3; i++ {
		_, err = b.InsertRepoSecret(w, 10, fmt.Sprintf("secret-%d", i), []byte("n"), []byte("e"))
		if err != nil {
			t.Fatal(err)
		}
	}
	// Another repo's secret must not be counted
	_, err = b.InsertRepoSecret(w, 11, "other-repo-secret", []byte("n"), []byte("e"))
	if err != nil {
		t.Fatal(err)
	}

	count, err = b.CountRepoSecrets(w, 10)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected 3 secrets, got %d", count)
	}
}

func TestGetRepoSecretsPage(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	// Interleave two repos so ids are not contiguous per repo
	_, err = b.InsertRepoSecret(w, 100, "alpha", []byte("n"), []byte("e")) // id 1
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.InsertRepoSecret(w, 200, "other", []byte("n"), []byte("e")) // id 2
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.InsertRepoSecret(w, 100, "beta", []byte("n"), []byte("e")) // id 3
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.InsertRepoSecret(w, 100, "gamma", []byte("n"), []byte("e")) // id 4
	if err != nil {
		t.Fatal(err)
	}

	getPage := func(afterSecretId uint64, limit int64) []secrets.SecretRef {
		t.Helper()
		it, err := b.GetRepoSecretsPage(w, 100, afterSecretId, limit)
		if err != nil {
			t.Fatal(err)
		}
		got, err := iterator.GetFirstN(int(limit), it)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	expected := []secrets.SecretRef{
		{Id: 1, Name: "alpha"},
		{Id: 3, Name: "beta"},
		{Id: 4, Name: "gamma"},
	}
	if got := getPage(0, 10); !reflect.DeepEqual(expected, got) {
		t.Fatalf("expected %#v, got %#v", expected, got)
	}

	// After id 1, limited to 1 result
	expected = []secrets.SecretRef{{Id: 3, Name: "beta"}}
	if got := getPage(1, 1); !reflect.DeepEqual(expected, got) {
		t.Fatalf("expected %#v, got %#v", expected, got)
	}
}
