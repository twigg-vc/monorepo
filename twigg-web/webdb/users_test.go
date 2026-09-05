package webdb_test

import (
	"errors"
	"monorepo/base/iterator"
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

func TestGetUserByField(t *testing.T) {
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
		Email:      "someone@twigg.vc",
		State:      user.UserState_NoSubscription,
		Username:   "someone",
		StripeId:   "cus_1",
		CliKeyHash: "cli-key-hash",
	})
	if err != nil {
		t.Fatal(err)
	}

	lookups := map[string]func() (user.User, bool, error){
		"email":      func() (user.User, bool, error) { return b.GetUserByEmail(w, "someone@twigg.vc") },
		"username":   func() (user.User, bool, error) { return b.GetUserByUsername(w, "someone") },
		"stripeId":   func() (user.User, bool, error) { return b.GetUserByStripeId(w, "cus_1") },
		"cliKeyHash": func() (user.User, bool, error) { return b.GetUserByCliKeyHash(w, "cli-key-hash") },
	}
	for field, lookup := range lookups {
		got, isNotFoundErr, err := lookup()
		if err != nil {
			t.Fatalf("by %s: %v", field, err)
		}
		if isNotFoundErr {
			t.Fatalf("by %s: user not found", field)
		}
		if got != stored {
			t.Fatalf("by %s: got %+v, want %+v", field, got, stored)
		}
	}
}

func TestGetUserByFieldNotFound(t *testing.T) {
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

	lookups := map[string]func() (user.User, bool, error){
		"email":      func() (user.User, bool, error) { return b.GetUserByEmail(r, "nobody@twigg.vc") },
		"username":   func() (user.User, bool, error) { return b.GetUserByUsername(r, "nobody") },
		"stripeId":   func() (user.User, bool, error) { return b.GetUserByStripeId(r, "cus_nobody") },
		"cliKeyHash": func() (user.User, bool, error) { return b.GetUserByCliKeyHash(r, "no-hash") },
	}
	for field, lookup := range lookups {
		_, isNotFoundErr, err := lookup()
		if !isNotFoundErr {
			t.Fatalf("by %s: expected isNotFoundErr, got err %v", field, err)
		}
		if !errors.Is(err, webdb.ErrNotFound) {
			t.Fatalf("by %s: expected ErrNotFound, got %v", field, err)
		}
	}
}

func TestCountAndGetAllUsers(t *testing.T) {
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

	count, err := b.CountUsers(w)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no users, got %d", count)
	}

	first, err := b.UpsertUser(w, user.User{
		Email: "first@twigg.vc", State: user.UserState_NoSubscription,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.UpsertUser(w, user.User{
		Email: "second@twigg.vc", State: user.UserState_NoSubscription,
	})
	if err != nil {
		t.Fatal(err)
	}

	count, err = b.CountUsers(w)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 users, got %d", count)
	}

	it, err := b.GetAllUsers(w)
	if err != nil {
		t.Fatal(err)
	}
	got, err := iterator.GetFirstN(10, it)
	if err != nil {
		t.Fatal(err)
	}
	// Newest first.
	want := []user.User{second, first}
	if len(got) != len(want) {
		t.Fatalf("got %d users, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("user %d got %+v, want %+v", i, got[i], want[i])
		}
	}
}
