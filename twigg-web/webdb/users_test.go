package webdb_test

import (
	"errors"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webdb"
	"strconv"
	"testing"
)

func TestUpsertAndGetUser(t *testing.T) {
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

	inserted := user.User{
		Email:                        "someone@twigg.vc",
		State:                        user.UserState_StripeSubscription,
		IsOrganization:               false,
		StripeId:                     "cus_1",
		CliKeyHash:                   "cli-key-hash",
		Username:                     "someone",
		PasswordHash:                 "password-hash",
		SelfPaidSubscription:         user.Subscription_Solo,
		SelfPaidSubscriptionQuantity: 1,
		StripeSessionId:              "session-1",
		StripeSessionUrl:             "https://stripe.test/session-1",
		StripeSessionPriceId:         "price-1",
		StripeSessionQuantity:        1,
		StripeSubscriptionID:         "sub-1",
	}
	stored, err := b.UpsertUser(w, inserted)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Id != 1 {
		t.Fatalf("expected first user id 1, got %d", stored.Id)
	}
	userId := stored.Id
	inserted.Id = userId
	if stored != inserted {
		t.Fatalf("upsert returned %+v, want %+v", stored, inserted)
	}

	got, isNotFoundErr, err := b.GetUser(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("upserted user was not found")
	}
	if got != inserted {
		t.Fatalf("after insert got %+v, want %+v", got, inserted)
	}

	updated := inserted
	updated.Email = "renamed@twigg.vc"
	updated.Username = "renamed"
	updated.State = user.UserState_NoSubscription
	updated.SelfPaidSubscription = user.Subscription_None
	updated.SelfPaidSubscriptionQuantity = 0
	updated.StripeSessionId = ""
	updated.StripeSessionUrl = ""
	updated.StripeSessionPriceId = ""
	updated.StripeSessionQuantity = 0
	updated.StripeSubscriptionID = ""
	stored, err = b.UpsertUser(w, updated)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Id != userId {
		t.Fatalf("upsert created id %d instead of updating %d", stored.Id, userId)
	}

	got, _, err = b.GetUser(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if got != updated {
		t.Fatalf("after update got %+v, want %+v", got, updated)
	}
}

func TestGetUserNotFound(t *testing.T) {
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

	_, isNotFoundErr, err := b.GetUser(r, 404)
	if !isNotFoundErr {
		t.Fatalf("expected isNotFoundErr, got err %v", err)
	}
	if !errors.Is(err, webdb.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetUserReadsQuotaFields(t *testing.T) {
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

	stored, err := b.UpsertUser(w, user.User{
		Email: "someone@twigg.vc",
		State: user.UserState_NoSubscription,
	})
	if err != nil {
		t.Fatal(err)
	}
	if stored.TotalQuota != 0 {
		t.Fatalf("expected no quota before it is set, got %d", stored.TotalQuota)
	}

	const quota = 4096
	if err := b.SetQuota(strconv.FormatInt(stored.Id, 10), quota); err != nil {
		t.Fatal(err)
	}

	got, _, err := b.GetUser(w, stored.Id)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalQuota != quota {
		t.Fatalf("GetUser got TotalQuota %d, want %d", got.TotalQuota, quota)
	}

	reUpserted, err := b.UpsertUser(w, got)
	if err != nil {
		t.Fatal(err)
	}
	if reUpserted.TotalQuota != quota {
		t.Fatalf("UpsertUser got TotalQuota %d, want %d", reUpserted.TotalQuota, quota)
	}
}
