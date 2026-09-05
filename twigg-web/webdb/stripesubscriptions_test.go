package webdb_test

import (
	"errors"
	"monorepo/twigg-web/webdb"
	"testing"
)

func TestUpsertAndGetStripeSubscription(t *testing.T) {
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

	if err := b.UpsertStripeSubscription(w, "sub-1", 7, true); err != nil {
		t.Fatal(err)
	}
	isActive, isNotFoundErr, err := b.GetStripeSubscriptionIsActive(w, "sub-1")
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("upserted subscription was not found")
	}
	if !isActive {
		t.Fatal("expected the subscription to be active")
	}

	// Upserting the same id deactivates it instead of adding a row.
	if err := b.UpsertStripeSubscription(w, "sub-1", 7, false); err != nil {
		t.Fatal(err)
	}
	isActive, _, err = b.GetStripeSubscriptionIsActive(w, "sub-1")
	if err != nil {
		t.Fatal(err)
	}
	if isActive {
		t.Fatal("expected the subscription to be inactive")
	}
}

func TestGetStripeSubscriptionIsActiveNotFound(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	r, closeR, err := b.BeginRead()
	defer closeR()
	if err != nil {
		t.Fatal(err)
	}

	_, isNotFoundErr, err := b.GetStripeSubscriptionIsActive(r, "sub-404")
	if !isNotFoundErr {
		t.Fatalf("expected isNotFoundErr, got err %v", err)
	}
	if !errors.Is(err, webdb.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
