package user

import (
	"bytes"
	"context"
	"monorepo/twigg-web/services/stripeclient"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webdb"
	"reflect"
	"testing"
	"time"
)

const testSalt = "test-salt"

func TestCantFindNonExistingUser(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}

	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, stripeclient.NewMockStripeClient(), db, testSalt)
	if err != nil {
		t.Fatal(err)
	}
	_, isNotFoundErr, err := s.Get(w, 0)
	if err == nil {
		t.Fatal("should err")
	}
	if !isNotFoundErr {
		t.Fatal("err should be ErrNotFound")
	}
}

func TestCreateAndGetUsernameWithPassword(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, stripeclient.NewMockStripeClient(), db, testSalt)
	if err != nil {
		t.Fatal(err)
	}
	createdUser, err := s.RegisterNewUser(w, "my@email.com", "bilbo", "strong-password")
	if err != nil {
		t.Fatal(err)
	}
	expectedUser := User{
		State:                        user.UserState_NoSubscription,
		Id:                           1,
		Email:                        "my@email.com",
		IsOrganization:               false,
		Username:                     "bilbo",
		PasswordHash:                 HashWithSalt("strong-password", testSalt),
		SelfPaidSubscription:         Subscription_None,
		SelfPaidSubscriptionQuantity: 0,
		CliKeyHash:                   "",
		StripeId:                     "",
		StripeSessionId:              "",
		StripeSessionUrl:             "",
		StripeSessionPriceId:         "",
		StripeSessionQuantity:        0,
		StripeSubscriptionID:         "",
	}
	if createdUser != expectedUser {
		t.Fatal("got unexpected created user")
	}

	gotUser, _, err := s.GetByEmail(w, "my@email.com")
	if err != nil {
		t.Fatal(err)
	}
	if gotUser != expectedUser {
		t.Fatal("unexpected user by email")
	}
	gotUser, _, err = s.GetByUsername(w, "bilbo")
	if err != nil {
		t.Fatal(err)
	}
	if gotUser != expectedUser {
		t.Fatal("unexpected user by email")
	}
	jobLimit.checkLimits(gotUser.Id, 0, 0)

	uname, err := s.GetUsername(gotUser.Id, w)
	if err != nil {
		t.Fatal(err)
	}
	if uname != "bilbo" {
		t.Fatalf("bad username %q", uname)
	}
}

func TestCountAll(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, stripeclient.NewMockStripeClient(), db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	n, err := s.CountAll(w)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("got %d users", n)
	}
	s.RegisterNewUser(w, "first@email.com", "first", "first-password")
	s.RegisterNewUser(w, "second@email.com", "second", "second-password")
	n, err = s.CountAll(w)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("got %d users", n)
	}
}

func TestGetAll(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, stripeclient.NewMockStripeClient(), db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	s.RegisterNewUser(w, "first@email.com", "first", "first-password")
	s.RegisterNewUser(w, "second@email.com", "second", "second-password")
	s.RegisterNewUser(w, "third@email.com", "third", "third-password")

	// Get all
	users, err := s.GetAll(w)
	if err != nil {
		t.Fatal(err)
	}
	gotUsers := []User{}
	for users.Next() {
		gotUser, err := users.Get()
		if err != nil {
			t.Fatal(err)
		}
		gotUsers = append(gotUsers, gotUser)
	}
	err = users.Err()
	if err != nil {
		t.Fatal(err)
	}
	if len(gotUsers) != 3 {
		t.Fatalf("expected 3 users got %d", len(gotUsers))
	}

	// Expects descending order
	expectedUsers := []User{
		{
			State:                        user.UserState_NoSubscription,
			Id:                           3,
			Email:                        "third@email.com",
			IsOrganization:               false,
			Username:                     "third",
			PasswordHash:                 HashWithSalt("third-password", testSalt),
			SelfPaidSubscription:         Subscription_None,
			SelfPaidSubscriptionQuantity: 0,
			CliKeyHash:                   "",
			StripeId:                     "",
			StripeSessionId:              "",
			StripeSessionUrl:             "",
			StripeSessionPriceId:         "",
			StripeSessionQuantity:        0,
			StripeSubscriptionID:         "",
		},
		{
			State:                        user.UserState_NoSubscription,
			Id:                           2,
			Email:                        "second@email.com",
			Username:                     "second",
			PasswordHash:                 HashWithSalt("second-password", testSalt),
			SelfPaidSubscription:         Subscription_None,
			SelfPaidSubscriptionQuantity: 0,
			CliKeyHash:                   "",
			StripeId:                     "",
			StripeSessionId:              "",
			StripeSessionUrl:             "",
			StripeSessionPriceId:         "",
			StripeSessionQuantity:        0,
			StripeSubscriptionID:         "",
		},
		{
			State:                        user.UserState_NoSubscription,
			Id:                           1,
			Email:                        "first@email.com",
			Username:                     "first",
			PasswordHash:                 HashWithSalt("first-password", testSalt),
			SelfPaidSubscription:         Subscription_None,
			SelfPaidSubscriptionQuantity: 0,
			CliKeyHash:                   "",
			StripeId:                     "",
			StripeSessionId:              "",
			StripeSessionUrl:             "",
			StripeSessionPriceId:         "",
			StripeSessionQuantity:        0,
			StripeSubscriptionID:         "",
		},
	}
	if !reflect.DeepEqual(gotUsers, expectedUsers) {
		t.Fatal("got unexpected users")
	}
	jobLimit.checkLimits(3, 0, 0)
	jobLimit.checkLimits(2, 0, 0)
	jobLimit.checkLimits(1, 0, 0)
}

func TestGetUserForPaymentWithStripe(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}

	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	// Create
	u, err := s.RegisterNewUser(w, "my@email.com", "user_1", "12345")
	if err != nil {
		t.Fatal(err)
	}
	readyToPerformPaymentUser, _, err := s.GetUserForPaymentWithStripe(
		w, u.Id, strCli.GetLatestTeamPriceId() /*quantity*/, 14,
		/*forceCurrency*/ "")
	if err != nil {
		t.Fatal(err)
	}
	jobLimit.checkLimits(u.Id, 0, 0)

	// Check if stripe fields are correct
	if len(readyToPerformPaymentUser.StripeId) == 0 {
		t.Fatal("stripe id is empty")
	}
	if len(readyToPerformPaymentUser.StripeSessionId) == 0 {
		t.Fatal("stripe session id is empty")
	}
	if len(readyToPerformPaymentUser.StripeSessionUrl) == 0 {
		t.Fatal("stripe session url is empty")
	}
	if readyToPerformPaymentUser.StripeSessionPriceId !=
		strCli.GetLatestTeamPriceId() {
		t.Fatal("wrong stripe session prince id")
	}

	// Check with the stripe client if the session was created correctly
	strCli.CheckSessionIsUnpaid(readyToPerformPaymentUser.StripeSessionId, t)
	if strCli.SessionUserID(readyToPerformPaymentUser.StripeSessionId) != u.Id {
		t.Fatal("wrong user id in session")
	}
	if strCli.SessionQuantity(
		readyToPerformPaymentUser.StripeSessionId) != 14 {
		t.Fatal("wrong quantity in session")
	}
}

func TestGetUserForPaymentWithStripeTwice(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	jobLimit := newFakeJobLimitSetter(t)
	strCli := stripeclient.NewMockStripeClient()
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	// Create a test user
	user1, _ := s.RegisterNewUser(w, "my@email.com", "user_1", "12345")
	// Call GetUserForPaymentWithStripe so that a session is created with stripe
	user1, _, _ = s.GetUserForPaymentWithStripe(
		w, user1.Id, strCli.GetLatestTeamPriceId() /*quantity*/, 99 /*forceCurrency*/, "")
	jobLimit.checkLimits(user1.Id, 0, 0)

	// Now start a new session on stripe for the solo plan.
	// The previous one must be deleted
	sessionThatMustBeDeleted := user1.StripeSessionId
	user2, _, _ := s.GetUserForPaymentWithStripe(
		w, user1.Id, strCli.GetLatestSoloPriceId() /*quantity*/, 1 /*forceCurrency*/, "")
	jobLimit.checkLimits(user2.Id, 0, 0)

	// Check if the session was deleted
	strCli.CheckSessionWasExpired(sessionThatMustBeDeleted, t)

	// Verify that the new user has the proper fields
	if user2.StripeId != user1.StripeId {
		t.Fatal("stripe id changed")
	}
	if len(user2.StripeSessionId) == 0 {
		t.Fatal("stripe session id is empty")
	}
	if len(user2.StripeSessionUrl) == 0 {
		t.Fatal("stripe session url is empty")
	}
	if user2.StripeSessionPriceId !=
		strCli.GetLatestSoloPriceId() {
		t.Fatal("wrong stripe session prince id")
	}
	if user2.StripeSessionQuantity != 1 {
		t.Fatal("wrong stripe session quantity")
	}
	// Check with stripe that the new session was created
	strCli.CheckSessionIsUnpaid(user2.StripeSessionId, t)
	if strCli.SessionUserID(user2.StripeSessionId) != user2.Id {
		t.Fatal("wrong user id in session")
	}
	if strCli.SessionQuantity(user2.StripeSessionId) != 1 {
		t.Fatal("wrong quantity in session")
	}
}

func TestMockUserWithDeletedStripeSession(t *testing.T) {
	// Deleting a session involves 2 steps: first we ask stripe to delete it;
	// then we mark it as deleted in our side. Let's test a case in which
	// stripe successfully deleted the session, but we failed to save that
	// information on our side.
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, commit, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}

	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	// Create a new user. Save the user. Commit the lock.
	// Release (unlock/close) the lock
	u, err := s.RegisterNewUser(w, "aang@mail.com", "user_1", "12345")
	if err != nil {
		t.Fatal(err)
	}
	// instantiate every stripe field
	u, _, err = s.GetUserForPaymentWithStripe(
		w, u.Id, strCli.GetLatestSoloPriceId() /*quantity*/, 1 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}

	strCli.CheckSessionIsUnpaid(u.StripeSessionId, t)
	commit()
	clW()

	// Get a new lock.
	w, clW, commit, err = db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	// Get a user for payment. Check with stripe client that the session for
	// this user is ok.
	notCommittedUser, _, err := s.GetUserForPaymentWithStripe(
		w, u.Id, strCli.GetLatestSoloPriceId() /*quantity*/, 1 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}
	strCli.CheckSessionIsUnpaid(notCommittedUser.StripeSessionId, t)
	if strCli.SessionUserID(notCommittedUser.StripeSessionId) !=
		notCommittedUser.Id {
		t.Fatal("wrong user id in session")
	}
	if strCli.SessionQuantity(notCommittedUser.StripeSessionId) !=
		notCommittedUser.StripeSessionQuantity {
		t.Fatal("wrong quantity in session")
	}
	// Now, just close the lock without committing it. This mimics a DB failure
	clW()

	// Get a new lock.
	w, clW, commit, err = db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	// Get a user for payment. Now check 3 things with stripe: the old session
	// was deleted, the not committed one and this new one are ok
	userValidSession, _, err := s.GetUserForPaymentWithStripe(
		w, u.Id, strCli.GetLatestSoloPriceId() /*quantity*/, 1 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}
	strCli.CheckSessionWasExpired(u.StripeSessionId, t)
	strCli.CheckSessionIsUnpaid(notCommittedUser.StripeSessionId, t)
	strCli.CheckSessionIsUnpaid(userValidSession.StripeSessionId, t)
	if strCli.SessionUserID(userValidSession.StripeSessionId) !=
		userValidSession.Id {
		t.Fatal("wrong user id in session")
	}
	if strCli.SessionQuantity(userValidSession.StripeSessionId) !=
		userValidSession.StripeSessionQuantity {
		t.Fatal("wrong user quantity")
	}
}
func TestDontCommitUserWithSessionAndGetNewSessionFromStripe(t *testing.T) {
	// Fist time we call GetUserForPaymentWithStripe() for a user we crete
	// its stripe id and stripe session and then update these values in user.
	// Let's test a case in which stripe successfully creates stripe id and
	// stripe session, but we failed to save that information on our side.
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, commit, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}

	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	// Create a new user. Save the user. Commit the lock.
	// Release (unlock/close) the lock
	u, err := s.RegisterNewUser(w, "katara@mail.com", "user_1", "12345")
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, _, err = strCli.GetSessionStatus(u.StripeSessionId)
	if err == nil {
		t.Fatal("session should not exist yet")
	}
	commit()
	clW()

	// Get a new lock.
	w, clW, commit, err = db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	// Get a user for payment. Check with stripe client that the session for
	// this user is ok.
	notCommittedUser, _, err := s.GetUserForPaymentWithStripe(
		w, u.Id, strCli.GetLatestSoloPriceId() /*quantity*/, 1 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}
	strCli.CheckSessionIsUnpaid(notCommittedUser.StripeSessionId, t)
	if strCli.SessionUserID(notCommittedUser.StripeSessionId) !=
		notCommittedUser.Id {
		t.Fatal("wrong user id in session")
	}
	if strCli.SessionQuantity(notCommittedUser.StripeSessionId) != 1 {
		t.Fatal("wrong quantity of seats in session")
	}
	// Now, just close the lock without committing it. This mimics a DB failure
	clW()

	// Get a new lock.
	w, clW, commit, err = db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	// Get a user for payment. Now check with stripe that both not committed and
	// current session exist.
	currentUser, _, err := s.GetUserForPaymentWithStripe(
		w, u.Id, strCli.GetLatestSoloPriceId() /*quantity*/, 1 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}
	strCli.CheckSessionIsUnpaid(notCommittedUser.StripeSessionId, t)
	strCli.CheckSessionIsUnpaid(currentUser.StripeSessionId, t)
	if strCli.SessionUserID(currentUser.StripeSessionId) != currentUser.Id {
		t.Fatal("wrong user id in session")
	}
	if strCli.SessionQuantity(currentUser.StripeSessionId) != 1 {
		t.Fatal("wrong quantity of seats in session")
	}

}

func TestHandleStripeCheckoutSessionSuccess(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	u, err := s.RegisterNewUser(w, "sokka@mail.com", "user_1", "12345")
	if err != nil {
		t.Fatal(err)
	}
	jobLimit.checkLimits(u.Id, 0, 0)

	u, _, err = s.GetUserForPaymentWithStripe(
		w, u.Id, strCli.GetLatestSoloPriceId() /*quantity*/, 1 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}
	if u.State != user.UserState_PayingWithStripe {
		t.Fatal("wrong state")
	}
	jobLimit.checkLimits(u.Id, 0, 0)

	_, err = s.HandleStripeCheckoutSessionSuccess(w, u.Id,
		/*stripeSubscriptionID*/ "sub_12345",
		u.StripeSessionId, strCli.GetLatestSoloPriceId() /*quantity*/, 1)
	if err != nil {
		t.Fatal(err)
	}
	jobLimit.checkLimits(u.Id, SoloMaxParallelJobs, SoloMaxParallelTimeoutSum)
	u, _, err = s.Get(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	if u.State != user.UserState_StripeSubscription {
		t.Fatal("wrong state")
	}
	if u.SelfPaidSubscription != Subscription_Solo {
		t.Fatal("wrong sub")
	}
	if u.SelfPaidSubscriptionQuantity != 1 {
		t.Fatal("wrong sub qty")
	}
	if u.StripeSubscriptionID != "sub_12345" {
		t.Fatal("wrong StripeSubscriptionID")
	}

	// Call again, simulating stripe replaying request
	_, err = s.HandleStripeCheckoutSessionSuccess(w, u.Id,
		/*stripeSubscriptionID*/ "sub_12345",
		u.StripeSessionId, strCli.GetLatestSoloPriceId() /*quantity*/, 1)
	if err != nil {
		t.Fatal(err)
	}
}

func TestStripeCancelAndReactivation(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	// Pay once
	u, err := s.RegisterNewUser(w, "sokka@mail.com", "user_1", "12345")
	if err != nil {
		t.Fatal(err)
	}
	u, _, err = s.GetUserForPaymentWithStripe(
		w, u.Id, strCli.GetLatestSoloPriceId() /*quantity*/, 1 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.HandleStripeCheckoutSessionSuccess(w, u.Id,
		/*stripeSubscriptionID*/ "sub_12345",
		u.StripeSessionId, strCli.GetLatestSoloPriceId() /*quantity*/, 1)
	if err != nil {
		t.Fatal(err)
	}
	jobLimit.checkLimits(u.Id, SoloMaxParallelJobs, SoloMaxParallelTimeoutSum)
	// Now cancel
	_, err = s.HandlesSubscriptionDeleted(w, u.StripeId, "sub_12345")
	if err != nil {
		t.Fatal(err)
	}
	jobLimit.checkLimits(u.Id, 0, 0)
	u, _, err = s.Get(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	if u.State != user.UserState_NoSubscription {
		t.Fatal("wrong state")
	}

	// Now pay again using with a new sub id
	u, _, err = s.GetUserForPaymentWithStripe(
		w, u.Id, strCli.GetLatestSoloPriceId() /*quantity*/, 1 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.HandleStripeCheckoutSessionSuccess(w, u.Id,
		/*stripeSubscriptionID*/ "sub_6789",
		u.StripeSessionId, strCli.GetLatestSoloPriceId() /*quantity*/, 1)
	if err != nil {
		t.Fatal(err)
	}
	jobLimit.checkLimits(u.Id, SoloMaxParallelJobs, SoloMaxParallelTimeoutSum)
	u, _, err = s.Get(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	if u.State != user.UserState_StripeSubscription {
		t.Fatal("wrong state")
	}
}

func TestHandleSubscriptionQuantityUpdated_increaseQuantity(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.RegisterNewUser(w, "obi-wan@email.com", "obi-wan", "force-123")
	if err != nil {
		t.Fatal(err)
	}
	// Create subscription
	u, _, err = s.GetUserForPaymentWithStripe(
		w, u.Id, strCli.GetLatestTeamPriceId() /*quantity*/, 10 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.HandleStripeCheckoutSessionSuccess(w, u.Id,
		/*stripeSubscriptionID*/ "sub_12345",
		u.StripeSessionId, strCli.GetLatestTeamPriceId() /*quantity*/, 10)
	if err != nil {
		t.Fatal(err)
	}
	jobLimit.checkLimits(u.Id, TeamMaxParllelJobs, TeamMaxParallelTimeoutSum)
	// Now update quantity to 100
	u, err = s.HandleSubscriptionQuantityUpdated(w, u.StripeId, "sub_12345", 100)
	if err != nil {
		t.Fatal(err)
	}
	if u.SelfPaidSubscriptionQuantity != 100 {
		t.Fatalf("wrong SelfPaidSubscriptionQuantity got=%v", u.SelfPaidSubscriptionQuantity)
	}

	// Check if it is persistent
	uPersistent, _, err := s.Get(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	if uPersistent.SelfPaidSubscriptionQuantity != 100 {
		t.Fatalf("not persistent SelfPaidSubscriptionQuantity got=%v", uPersistent.SelfPaidSubscriptionQuantity)
	}
	jobLimit.checkLimits(u.Id, TeamMaxParllelJobs, TeamMaxParallelTimeoutSum)
}

func TestHandleSubscriptionQuantityUpdated_decreaseQuantity(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.RegisterNewUser(w, "anakinskywalker@email.com", "anakin-skywalker", "chosen-one")
	if err != nil {
		t.Fatal(err)
	}
	// Create subscription
	u, _, err = s.GetUserForPaymentWithStripe(
		w, u.Id, strCli.GetLatestTeamPriceId() /*quantity*/, 2 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.HandleStripeCheckoutSessionSuccess(w, u.Id,
		/*stripeSubscriptionID*/ "sub_12345",
		u.StripeSessionId, strCli.GetLatestTeamPriceId() /*quantity*/, 2)
	if err != nil {
		t.Fatal(err)
	}
	jobLimit.checkLimits(u.Id, TeamMaxParllelJobs, TeamMaxParallelTimeoutSum)
	// Now update quantity to 1
	u, err = s.HandleSubscriptionQuantityUpdated(w, u.StripeId, "sub_12345", 1)
	if err != nil {
		t.Fatal(err)
	}
	if u.SelfPaidSubscriptionQuantity != 1 {
		t.Fatalf("wrong SelfPaidSubscriptionQuantity got=%v", u.SelfPaidSubscriptionQuantity)
	}

	// Check if it is persistent
	uPersistent, _, err := s.Get(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	if uPersistent.SelfPaidSubscriptionQuantity != 1 {
		t.Fatalf("not persistent SelfPaidSubscriptionQuantity got=%v", uPersistent.SelfPaidSubscriptionQuantity)
	}
}

func TestHandleSubscriptionQuantityUpdated_invalidInputs(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.RegisterNewUser(w, "yoda@email.com", "yoda", "password-you-must-have")
	if err != nil {
		t.Fatal(err)
	}
	// Create subscription
	u, _, err = s.GetUserForPaymentWithStripe(
		w, u.Id, strCli.GetLatestTeamPriceId() /*quantity*/, 10 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.HandleStripeCheckoutSessionSuccess(w, u.Id,
		/*stripeSubscriptionID*/ "sub_12345",
		u.StripeSessionId, strCli.GetLatestTeamPriceId() /*quantity*/, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Get user for latter checking
	u, _, err = s.Get(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	// invalid quantity -1
	_, err = s.HandleSubscriptionQuantityUpdated(w, u.StripeId, "sub_12345", -1)
	if err == nil {
		t.Fatal("expected err for invalid quantity input")
	}
	// invalid stripe id
	_, err = s.HandleSubscriptionQuantityUpdated(w, "fake stripe id", "sub_12345", -1)
	if err == nil {
		t.Fatal("expected err for invalid quantity input")
	}
	// Expect filed to be unchanged
	gotU, _, err := s.Get(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	if gotU != u {
		t.Fatal("user changed with invalid input")
	}
}

func TestGetByStripeId(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	u, err := s.RegisterNewUser(w, "zuko@mail.com", "user_1", "12345")
	if err != nil {
		t.Fatal(err)
	}
	u, _, err = s.GetUserForPaymentWithStripe(w, u.Id, strCli.GetLatestTeamPriceId() /*quantity*/, 5 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}

	userGotByStripeId, isNotFoundErr, err := s.GetByStripeId(w, u.StripeId)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("should not have err")
	}
	if userGotByStripeId != u {
		t.Fatal("they should be equal")
	}

	// Invalid stripe id
	_, isNotFoundErr, err = s.GetByStripeId(w, "invalid_stripe_id")
	if !isNotFoundErr {
		t.Fatal("should have err is nor found")
	}
	if err == nil {
		t.Fatal(err)
	}
}

func TestHandlesSubscriptionDeleted(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	// Create user, get checkout session, pay checkout session, get portal and
	// delete subscription. Check expected fields.
	u, err := s.RegisterNewUser(w, "appa@mail.com", "appa", "12345")
	if err != nil {
		t.Fatal(err)
	}
	u, _, err = s.GetUserForPaymentWithStripe(
		w, u.Id, strCli.GetLatestTeamPriceId() /*quantity*/, 10 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.HandleStripeCheckoutSessionSuccess(w, u.Id,
		/*stripeSubscriptionID*/ "sub_12345",
		u.StripeSessionId, strCli.GetLatestTeamPriceId() /*quantity*/, 10)
	if err != nil {
		t.Fatal(err)
	}
	jobLimit.checkLimits(u.Id, TeamMaxParllelJobs, TeamMaxParallelTimeoutSum)
	u, _, err = s.Get(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	if u.State != user.UserState_StripeSubscription {
		t.Fatal("wrong stripe state")
	}
	subscriptionId := u.StripeSubscriptionID

	_, err = strCli.GetNewCustomerPortalSession(u.Id, u.StripeId)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.HandlesSubscriptionDeleted(w, u.StripeId, subscriptionId)
	if err != nil {
		t.Fatal(err)
	}
	jobLimit.checkLimits(u.Id, 0, 0)

	u, _, err = s.Get(w, u.Id)
	if err != nil {
		t.Fatal(err)
	}

	// Note that the plan fields are not updated bc PaymentPlanIsActive
	// shows the plan is inactive
	expectUser := User{
		State:                        user.UserState_NoSubscription,
		Id:                           1,
		Email:                        "appa@mail.com",
		IsOrganization:               false,
		Username:                     "appa",
		PasswordHash:                 HashWithSalt("12345", testSalt),
		SelfPaidSubscription:         Subscription_None,
		SelfPaidSubscriptionQuantity: 0,
		CliKeyHash:                   "",
		StripeId:                     u.StripeId,
		StripeSessionId:              "",
		StripeSessionUrl:             "",
		StripeSessionPriceId:         "",
		StripeSessionQuantity:        0,
		StripeSubscriptionID:         "",
	}
	if u != expectUser {
		t.Fatal("something is wrong")
	}
	// Call delete again, simulating stripe replaying request
	_, err = s.HandlesSubscriptionDeleted(w, u.StripeId, subscriptionId)
	if err != nil {
		t.Fatal(err)
	}
	if u != expectUser {
		t.Fatal("something is wrong after deleting for second time")
	}
}

func TestHandleSuccessfulManualPayment(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	// Create a user
	s.RegisterNewUser(w, "aang@southern-temple.air", "aang", "password")
	gotUser, err := s.HandleManualPaymentSuccess(w, 1, Subscription_Solo, 1)
	if err != nil {
		t.Fatal(err)
	}

	jobLimit.checkLimits(1, SoloMaxParallelJobs, SoloMaxParallelTimeoutSum)
	// Paying again wont work because the plan is already active
	_, err = s.HandleManualPaymentSuccess(w, 1, Subscription_Solo, 1)
	if err == nil {
		t.Fatal("should not succeed to pay again")
	}

	expectedUser := User{
		State:                        user.UserState_ManualSubscription,
		Id:                           1,
		Email:                        "aang@southern-temple.air",
		IsOrganization:               false,
		Username:                     "aang",
		PasswordHash:                 HashWithSalt("password", testSalt),
		SelfPaidSubscription:         Subscription_Solo,
		SelfPaidSubscriptionQuantity: 1,
		CliKeyHash:                   "",
		StripeId:                     "",
		StripeSessionId:              "",
		StripeSessionUrl:             "",
		StripeSessionPriceId:         "",
		StripeSessionQuantity:        0,
		StripeSubscriptionID:         "",
		TotalQuota:                   SoloStorageQuota,
		QuotaUsed:                    0,
	}
	if gotUser != expectedUser {
		t.Fatal("unexpected user")
	}
	// Check persistence
	u, _, err := s.GetByUsername(w, "aang")
	if err != nil {
		t.Fatal(err)
	}
	if u != expectedUser {
		t.Fatal("changes in user did not persisted")
	}
}

func TestHandleCantPayManually(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	// Create a user and pay manually
	u, _ := s.RegisterNewUser(w, "aang@southern-temple.air", "aang", "password")
	_, err = s.HandleManualPaymentSuccess(w, u.Id, Subscription_Solo, 1)
	if err != nil {
		t.Fatal(err)
	}
	jobLimit.checkLimits(u.Id, SoloMaxParallelJobs, SoloMaxParallelTimeoutSum)

	// Cant pay again manually
	_, err = s.HandleManualPaymentSuccess(w, u.Id, Subscription_Solo, 1)
	if err == nil {
		t.Fatal("should not succeed to pay again")
	}

	// Also cant start to pay with stripe
	_, _, err = s.GetUserForPaymentWithStripe(w, u.Id,
		strCli.GetLatestTeamPriceId(), 1 /*forceCurrency*/, "")
	if err == nil {
		t.Fatal("should not be able to start paying with stripe")
	}
}

func TestHandlePayingManuallyCancelsStripeSession(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	// Create a user
	u, _ := s.RegisterNewUser(w, "aang@southern-temple.air", "aang", "password")

	// Start paying with stripe
	u, _, err = s.GetUserForPaymentWithStripe(w, u.Id,
		strCli.GetLatestSoloPriceId(), 1 /*forceCurrency*/, "")
	if err != nil {
		t.Fatal(err)
	}
	stripeSession := u.StripeSessionId

	// Just pay manually instead, the stripe session should be expired
	_, err = s.HandleManualPaymentSuccess(w, u.Id, Subscription_Solo, 1)
	if err != nil {
		t.Fatal(err)
	}
	strCli.CheckSessionWasExpired(stripeSession, t)
}

func TestChooseUsernameAndStartTrial(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	strCli := stripeclient.NewMockStripeClient()
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, strCli, db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	email := "frodo@shire.me"
	u, err := s.RegisterNewUserFromOAuth(w, email)
	if err != nil {
		t.Fatal(err)
	}
	username := "frodo"
	gotUser, err := s.ChooseUsernameAndStartTrial(w, u.Id, username)
	if err != nil {
		t.Fatal(err)
	}
	jobLimit.checkLimits(u.Id, TrialMaxParallelJobs, TrialMaxParallelTimeoutSum)

	expectedUser := User{
		State:                        user.UserState_ManualSubscription,
		Id:                           1,
		Email:                        email,
		IsOrganization:               false,
		Username:                     username,
		PasswordHash:                 "",
		SelfPaidSubscription:         Subscription_Trial,
		SelfPaidSubscriptionQuantity: 1,
		CliKeyHash:                   "",
		StripeId:                     "",
		StripeSessionId:              "",
		StripeSessionUrl:             "",
		StripeSessionPriceId:         "",
		StripeSessionQuantity:        0,
		StripeSubscriptionID:         "",
		TotalQuota:                   TrialStorageQuota,
		QuotaUsed:                    0,
	}
	if gotUser != expectedUser {
		t.Fatal("unexpected user")
	}
	// Check persistence
	u2, _, err := s.GetByUsername(w, username)
	if err != nil {
		t.Fatal(err)
	}
	if u2 != expectedUser {
		t.Fatal("changes in user did not persisted")
	}
}

func TestUsernameIsValid(t *testing.T) {
	testCases := []struct {
		desc     string
		username string
		expected bool
	}{
		{
			desc:     "true",
			username: "user_1",
			expected: true,
		},
		{
			desc:     "false too short",
			username: "b",
			expected: false,
		},
		{
			desc:     "false start with digit",
			username: "1username",
			expected: false,
		},
		{
			desc:     "true",
			username: "username_valid",
			expected: true,
		},
		{
			desc:     "false uppercase letters",
			username: "User_1",
			expected: false,
		},
		{
			desc:     "false contains with uppercase",
			username: "usErname",
			expected: false,
		},
	}
	for _, tC := range testCases {
		if UsernameIsValid(tC.username) != tC.expected {
			t.Fatalf("fails tets: %q, expected: %v got: %v",
				tC.desc, tC.expected, UsernameIsValid(tC.username))
		}
	}
}

func TestCantCreateUserWithSameUsername(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, stripeclient.NewMockStripeClient(), db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	// Create
	_, err = s.RegisterNewUser(w, "gandalf@email.com", "gandalf", "12345")
	if err != nil {
		t.Fatal(err)
	}

	// Try again
	_, err = s.RegisterNewUser(w, "mithrandir@email.com", "gandalf", "12345")
	if err == nil {
		t.Fatal("expected err username is taken")
	}
}

func TestRegisterNewUserFromOAuth(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, stripeclient.NewMockStripeClient(), db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	emailFromOAuthProvider := "frodo@twigg.vc"
	u, err := s.RegisterNewUserFromOAuth(w, emailFromOAuthProvider)
	if err != nil {
		t.Fatal(err)
	}
	expectedUser := User{
		State:                        user.UserState_NoUsername,
		Id:                           1,
		Email:                        emailFromOAuthProvider,
		IsOrganization:               false,
		Username:                     "",
		PasswordHash:                 "",
		SelfPaidSubscription:         Subscription_None,
		SelfPaidSubscriptionQuantity: 0,
		CliKeyHash:                   "",
		StripeId:                     "",
		StripeSessionId:              "",
		StripeSessionUrl:             "",
		StripeSessionPriceId:         "",
		StripeSessionQuantity:        0,
		StripeSubscriptionID:         "",
	}
	if u != expectedUser {
		t.Fatal("should be equal")
	}

	// Just check that the id increments
	u2, err := s.RegisterNewUserFromOAuth(w, "gandalf@twigg.vc")
	if err != nil {
		t.Fatal(err)
	}
	if u2.Id == 0 {
		t.Fatal("id didn't increase")
	}
	jobLimit.checkLimits(u.Id, 0, 0)
}

func TestUpdateUsername(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, stripeclient.NewMockStripeClient(), db, testSalt)
	if err != nil {
		t.Fatal(err)
	}
	emailFromOAuthProvider := "frodo@twigg.vc"
	u, err := s.RegisterNewUserFromOAuth(w, emailFromOAuthProvider)
	if err != nil {
		t.Fatal(err)
	}

	username := "frodo-baggins"
	updatedUser, err := s.UpdateUsername(w, u.Id, username)
	if err != nil {
		t.Fatal(err)
	}
	expectedUser := User{
		State:                        user.UserState_NoSubscription,
		Id:                           1,
		Email:                        emailFromOAuthProvider,
		IsOrganization:               false,
		Username:                     username,
		PasswordHash:                 "",
		SelfPaidSubscription:         Subscription_None,
		SelfPaidSubscriptionQuantity: 0,
		CliKeyHash:                   "",
		StripeId:                     "",
		StripeSessionId:              "",
		StripeSessionUrl:             "",
		StripeSessionPriceId:         "",
		StripeSessionQuantity:        0,
		StripeSubscriptionID:         "",
	}
	if updatedUser != expectedUser {
		t.Fatal("should be equal")
	}
}

func TestNotAllowedUserTriesToUpdateUsername(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, stripeclient.NewMockStripeClient(), db, testSalt)
	if err != nil {
		t.Fatal(err)
	}

	u, err := s.RegisterNewUser(w, "legolas@email.com", "legolas", "12345")
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.UpdateUsername(w, u.Id, "legolas-elf")
	if err == nil {
		t.Fatal("should err because username is already set")
	}

	u, _, err = s.Get(w, u.Id)
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "legolas" {
		t.Fatal("username should not change")
	}
}
func TestUpdateCliKey(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, stripeclient.NewMockStripeClient(), db, testSalt)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.RegisterNewUser(w, "yoda@twigg.vc", "yoda", "05/04")
	if err != nil {
		t.Fatal(err)
	}

	err = s.UpdateCliKey(w, u.Id, "tk_test_4eC39HqLyjWDarjtT1zdp7dc")
	if err != nil {
		t.Fatal(err)
	}
	expectedUser := User{
		State:                        user.UserState_NoSubscription,
		Id:                           1,
		Email:                        "yoda@twigg.vc",
		IsOrganization:               false,
		Username:                     "yoda",
		PasswordHash:                 u.PasswordHash,
		SelfPaidSubscription:         Subscription_None,
		SelfPaidSubscriptionQuantity: 0,
		CliKeyHash:                   HashWithSalt("tk_test_4eC39HqLyjWDarjtT1zdp7dc", testSalt),
		StripeId:                     "",
		StripeSessionId:              "",
		StripeSessionUrl:             "",
		StripeSessionPriceId:         "",
		StripeSessionQuantity:        0,
		StripeSubscriptionID:         "",
	}
	updatedUser, _, err := s.Get(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	if updatedUser != expectedUser {
		t.Fatal("should be equal")
	}
}
func TestDeleteCliKey(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, stripeclient.NewMockStripeClient(), db, testSalt)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.RegisterNewUser(w, "yoda@twigg.vc", "yoda", "05/04")
	if err != nil {
		t.Fatal(err)
	}
	err = s.UpdateCliKey(w, u.Id, "tk_test_4eC39HqLyjWDarjtT1zdp7dc")
	if err != nil {
		t.Fatal(err)
	}

	err = s.DeleteCliKey(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	deletedCliKeyUser := User{
		State:                        user.UserState_NoSubscription,
		Id:                           1,
		Email:                        "yoda@twigg.vc",
		IsOrganization:               false,
		Username:                     "yoda",
		PasswordHash:                 u.PasswordHash,
		SelfPaidSubscription:         Subscription_None,
		SelfPaidSubscriptionQuantity: 0,
		CliKeyHash:                   "",
		StripeId:                     "",
		StripeSessionId:              "",
		StripeSessionUrl:             "",
		StripeSessionPriceId:         "",
		StripeSessionQuantity:        0,
		StripeSubscriptionID:         "",
	}
	user, _, err := s.Get(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	if user != deletedCliKeyUser {
		t.Fatal("should be equal")
	}
}

func TestGetByCliKey(t *testing.T) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer clDb()
	w, clW, _, err := db.BeginWrite()
	defer clW()
	if err != nil {
		t.Fatal(err)
	}
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, stripeclient.NewMockStripeClient(), db, testSalt)
	if err != nil {
		t.Fatal(err)
	}
	registeredUser, err := s.RegisterNewUser(w, "yoda@twigg.vc", "yoda", "05/04")
	if err != nil {
		t.Fatal(err)
	}
	err = s.UpdateCliKey(w, registeredUser.Id, "fake-cli-key")
	if err != nil {
		t.Fatal(err)
	}
	registeredUser.CliKeyHash = HashWithSalt("fake-cli-key", testSalt)

	gotUser, isNotFoundErr, err := s.GetByCliKey(w, "fake-cli-key")
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if registeredUser != gotUser {
		t.Fatalf("got unexpected user")
	}

	s.DeleteCliKey(w, registeredUser.Id)

	_, isNotFoundErr, err = s.GetByCliKey(w, "fake-cli-key")
	if err == nil || !isNotFoundErr {
		t.Fatal("expected err bc cli key was deleted")
	}
}

func getServiceForTest(t *testing.T) (Service, *fakeJobLimitSetter, webdb.WebDb, context.Context) {
	db, clDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(clDb)
	w, clW, _, err := db.BeginWrite()
	t.Cleanup(clW)
	if err != nil {
		t.Fatal(err)
	}
	jobLimit := newFakeJobLimitSetter(t)
	s, err := NewService(jobLimit, stripeclient.NewMockStripeClient(), db, testSalt)
	if err != nil {
		t.Fatal(err)
	}
	return s, jobLimit, db, w
}

func TestQuota(t *testing.T) {
	s, _, db, w := getServiceForTest(t)

	// Create a user and pay manually
	u, _ := s.RegisterNewUser(w, "aang@southern-temple.air", "aang", "password")
	s.HandleManualPaymentSuccess(w, u.Id, Subscription_Solo, 1)
	u, _, _ = s.Get(w, u.Id)
	if u.TotalQuota != SoloStorageQuota {
		t.Fatal("wrong quota")
	}
	if u.QuotaUsed != 0 {
		t.Fatal("wrong used quota")
	}

	// Mock the usage of quota
	db.SetBlob(w, quotaOwnerName(u), "", "", bytes.NewBufferString("abc"))
	u, _, _ = s.Get(w, u.Id)
	qUsed := u.QuotaUsed
	if qUsed == 0 {
		t.Fatal("no quota used")
	}

	// Cancel the sub. Quota should be frozen.
	s.HandleManualSubscriptionDeleted(w, u.Id)
	u, _, _ = s.Get(w, u.Id)
	if u.TotalQuota != qUsed {
		t.Fatal("wrong quota")
	}
	if u.QuotaUsed != qUsed {
		t.Fatal("no quota used")
	}
}

func TestCreateNewOrganizationUser(t *testing.T) {
	s, jobLimit, _, w := getServiceForTest(t)

	gotUser, err := s.CreateNewOrganizationUser(w, "english-east-india")
	if err != nil {
		t.Fatal(err)
	}

	expectedUser := User{
		State:                        user.UserState_ManualSubscription,
		Id:                           1,
		Email:                        "",
		IsOrganization:               true,
		Username:                     "english-east-india",
		PasswordHash:                 "",
		SelfPaidSubscription:         Subscription_Trial,
		SelfPaidSubscriptionQuantity: 1,
		CliKeyHash:                   "",
		StripeId:                     "",
		StripeSessionId:              "",
		StripeSessionUrl:             "",
		StripeSessionPriceId:         "",
		StripeSessionQuantity:        0,
		StripeSubscriptionID:         "",
		TotalQuota:                   TrialStorageQuota,
		QuotaUsed:                    0,
	}
	if expectedUser != gotUser {
		t.Fatalf("unexpected user: %#v, got: %#v", expectedUser, gotUser)
	}

	jobLimit.checkLimits(
		gotUser.Id,
		TrialMaxParallelJobs,
		TrialMaxParallelTimeoutSum,
	)
}

func TestCreateNewOrganizationUser2times(t *testing.T) {
	s, _, _, w := getServiceForTest(t)

	firstOrgUser, err := s.CreateNewOrganizationUser(w, "english-east-india")
	if err != nil {
		t.Fatal(err)
	}
	secondOrgUser, err := s.CreateNewOrganizationUser(w, "standard-oil")
	if err != nil {
		t.Fatal(err)
	}

	if firstOrgUser.Id == secondOrgUser.Id {
		t.Fatal("got same id for both")
	}
	if !firstOrgUser.IsOrganization {
		t.Fatal("got IsOrganization=false for firstOrgUser")
	}
	if !secondOrgUser.IsOrganization {
		t.Fatal("got IsOrganization=false for secondOrgUser")
	}
}
func TestCreateNewOrganizationUserInvalidUsername(t *testing.T) {
	s, _, _, w := getServiceForTest(t)

	_, err := s.CreateNewOrganizationUser(w, "1invalid")
	if err == nil {
		t.Fatal("expected invalid username err")
	}
}

func TestCreateNewOrganizationUserUsernameAlreadyTaken(t *testing.T) {
	s, _, _, w := getServiceForTest(t)

	_, err := s.RegisterNewUser(
		w,
		"user@email.com",
		"taken-username",
		"password",
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.CreateNewOrganizationUser(w, "taken-username")
	if err == nil {
		t.Fatal("expected username already taken err")
	}
}

type fakeJobLimitSetter struct {
	t                  *testing.T
	userIdToMaxJobs    map[int64]int
	userIdToMaxTimeout map[int64]time.Duration
}

func newFakeJobLimitSetter(t *testing.T) *fakeJobLimitSetter {
	return &fakeJobLimitSetter{
		t:                  t,
		userIdToMaxJobs:    map[int64]int{},
		userIdToMaxTimeout: map[int64]time.Duration{},
	}
}
func (js *fakeJobLimitSetter) PutLimits(ownerId int64, maxJobs int,
	maxTimeout time.Duration, tx context.Context) error {
	js.userIdToMaxJobs[ownerId] = maxJobs
	js.userIdToMaxTimeout[ownerId] = maxTimeout
	return nil
}
func (js *fakeJobLimitSetter) checkLimits(ownerId int64, expectedMaxJobs int,
	expectedMaxTimeout time.Duration) {
	j, ok := js.userIdToMaxJobs[ownerId]
	if !ok {
		js.t.Fatalf("n jobs not found for userId %d", ownerId)
	}
	tm := js.userIdToMaxTimeout[ownerId]
	if j != expectedMaxJobs {
		js.t.Fatalf("expected maxJobs %v got %v for userId %d", expectedMaxJobs, j, ownerId)
	}
	if tm != expectedMaxTimeout {
		js.t.Fatalf("expected timeout %v got %v for userId %d", expectedMaxTimeout, tm, ownerId)
	}
}
