package webdb_test

import (
	"errors"
	"monorepo/base/iterator"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webdb"
	"testing"
)

func TestUpdateUser(t *testing.T) {
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

	userId, err := b.CreateUser(w, "someone@twigg.vc",
		user.UserState_NoSubscription, false, "someone", "password-hash",
		user.Subscription_None, 0)
	if err != nil {
		t.Fatal(err)
	}
	const quota = 4096
	if err := b.SetQuota(b.UserQuotaOwnerName(userId), quota); err != nil {
		t.Fatal(err)
	}
	created, _, err := b.GetUser(w, userId)
	if err != nil {
		t.Fatal(err)
	}

	// Every mutable column gets a new value.
	updated := created
	updated.Email = "renamed@twigg.vc"
	updated.State = user.UserState_StripeSubscription
	updated.IsOrganization = true
	updated.StripeId = "cus_1"
	updated.CliKeyHash = "cli-key-hash"
	updated.Username = "renamed"
	updated.PasswordHash = "new-password-hash"
	updated.SelfPaidSubscription = user.Subscription_Solo
	updated.SelfPaidSubscriptionQuantity = 1
	updated.StripeSessionId = "session-1"
	updated.StripeSessionUrl = "https://stripe.test/session-1"
	updated.StripeSessionPriceId = "price-1"
	updated.StripeSessionQuantity = 1
	updated.StripeSubscriptionID = "sub-1"
	if err := b.UpdateUser(w, updated); err != nil {
		t.Fatal(err)
	}

	got, isNotFoundErr, err := b.GetUser(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("updated user was not found")
	}
	if got != updated {
		t.Fatalf("got %+v, want %+v", got, updated)
	}
	if got.TotalQuota != quota {
		t.Fatalf("update lost the quota: got %d, want %d", got.TotalQuota, quota)
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

	userId, err := b.CreateUser(w, "someone@twigg.vc",
		user.UserState_NoSubscription, false, "", "", user.Subscription_None, 0)
	if err != nil {
		t.Fatal(err)
	}

	got, _, err := b.GetUser(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalQuota != 0 {
		t.Fatalf("expected no quota before it is set, got %d", got.TotalQuota)
	}

	const quota = 4096
	if err := b.SetQuota(b.UserQuotaOwnerName(userId), quota); err != nil {
		t.Fatal(err)
	}

	got, _, err = b.GetUser(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalQuota != quota {
		t.Fatalf("GetUser got TotalQuota %d, want %d", got.TotalQuota, quota)
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

	userId, err := b.CreateUser(w, "someone@twigg.vc",
		user.UserState_NoSubscription, false, "someone", "",
		user.Subscription_None, 0)
	if err != nil {
		t.Fatal(err)
	}
	created, _, err := b.GetUser(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	created.StripeId = "cus_1"
	created.CliKeyHash = "cli-key-hash"
	if err := b.UpdateUser(w, created); err != nil {
		t.Fatal(err)
	}
	stored, _, err := b.GetUser(w, userId)
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

	firstId, err := b.CreateUser(w, "first@twigg.vc",
		user.UserState_NoSubscription, false, "", "", user.Subscription_None, 0)
	if err != nil {
		t.Fatal(err)
	}
	secondId, err := b.CreateUser(w, "second@twigg.vc",
		user.UserState_NoSubscription, false, "", "", user.Subscription_None, 0)
	if err != nil {
		t.Fatal(err)
	}
	first, _, err := b.GetUser(w, firstId)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := b.GetUser(w, secondId)
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

func TestGetUsername(t *testing.T) {
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

	userId, err := b.CreateUser(w, "someone@twigg.vc",
		user.UserState_NoSubscription, false, "someone", "",
		user.Subscription_None, 0)
	if err != nil {
		t.Fatal(err)
	}

	username, isNotFoundErr, err := b.GetUsername(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("user was not found")
	}
	if username != "someone" {
		t.Fatalf("got username %q, want %q", username, "someone")
	}

	_, isNotFoundErr, err = b.GetUsername(w, 404)
	if !isNotFoundErr {
		t.Fatalf("expected isNotFoundErr, got err %v", err)
	}
	if !errors.Is(err, webdb.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetUserIsOrganization(t *testing.T) {
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

	personId, err := b.CreateUser(w, "someone@twigg.vc",
		user.UserState_NoSubscription, false, "", "", user.Subscription_None, 0)
	if err != nil {
		t.Fatal(err)
	}
	organizationId, err := b.CreateUser(w, "",
		user.UserState_NoSubscription, true, "acme", "",
		user.Subscription_None, 0)
	if err != nil {
		t.Fatal(err)
	}

	isOrganization, isNotFoundErr, err := b.GetUserIsOrganization(w, personId)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("user was not found")
	}
	if isOrganization {
		t.Fatal("expected the user to not be an organization")
	}

	isOrganization, _, err = b.GetUserIsOrganization(w, organizationId)
	if err != nil {
		t.Fatal(err)
	}
	if !isOrganization {
		t.Fatal("expected the user to be an organization")
	}

	_, isNotFoundErr, err = b.GetUserIsOrganization(w, 404)
	if !isNotFoundErr {
		t.Fatalf("expected isNotFoundErr, got err %v", err)
	}
	if !errors.Is(err, webdb.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateUser(t *testing.T) {
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

	userId, err := b.CreateUser(w, "someone@twigg.vc",
		user.UserState_NoSubscription, false, "someone", "password-hash",
		user.Subscription_Solo, 1)
	if err != nil {
		t.Fatal(err)
	}
	if userId != 1 {
		t.Fatalf("expected first user id 1, got %d", userId)
	}

	got, isNotFoundErr, err := b.GetUser(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("created user was not found")
	}
	want := user.User{
		Id:                           userId,
		Email:                        "someone@twigg.vc",
		State:                        user.UserState_NoSubscription,
		IsOrganization:               false,
		Username:                     "someone",
		PasswordHash:                 "password-hash",
		SelfPaidSubscription:         user.Subscription_Solo,
		SelfPaidSubscriptionQuantity: 1,
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSetUserStripeId(t *testing.T) {
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

	userId, err := b.CreateUser(w, "someone@twigg.vc",
		user.UserState_NoSubscription, false, "someone", "", user.Subscription_None, 0)
	if err != nil {
		t.Fatal(err)
	}
	created, _, err := b.GetUser(w, userId)
	if err != nil {
		t.Fatal(err)
	}

	if err := b.SetUserStripeId(w, userId, "cus_1"); err != nil {
		t.Fatal(err)
	}

	got, _, err := b.GetUser(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	want := created
	want.StripeId = "cus_1"
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
