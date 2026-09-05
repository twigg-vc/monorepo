package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"monorepo/base/iterator"
	"monorepo/twigg-web/services/stripeclient"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webdb"
	"net/mail"
	"regexp"
	"strconv"
	"time"
)

type service struct {
	db           webdb.WebDb
	js           JobLimitSetter
	stripeClient stripeclient.StripeClient
	salt         string
}

func newService(js JobLimitSetter, stripeClient stripeclient.StripeClient,
	db webdb.WebDb, salt string) (service, error) {
	return service{
		db:           db,
		js:           js,
		stripeClient: stripeClient,
		salt:         salt,
	}, nil
}

const plainPasswordMinLength = 5
const plainPasswordMaxLength = 30

func (s service) Get(r context.Context, id int64) (u user.User, isNotFoundErr bool, err error) {
	return s.db.GetUser(r, id)
}
func (s service) GetByEmail(r context.Context, email string) (u user.User, isNotFoundErr bool, err error) {
	return s.db.GetUserByEmail(r, email)
}
func (s service) GetByUsername(r context.Context, username string) (u user.User, isNotFoundErr bool, err error) {
	return s.db.GetUserByUsername(r, username)
}
func (s service) GetByStripeId(r context.Context, stripeId string) (u user.User, isNotFoundErr bool, err error) {
	return s.db.GetUserByStripeId(r, stripeId)
}
func (s service) GetByCliKey(r context.Context, plainCliKey string) (u user.User, isNotFoundErr bool, err error) {
	return s.db.GetUserByCliKeyHash(r, s.hashWithSalt(plainCliKey))
}
func (s service) GetUsername(userId int64, tx context.Context) (string, error) {
	username, _, err := s.db.GetUsername(tx, userId)
	return username, err
}

func (s service) CountAll(r context.Context) (int64, error) {
	return s.db.CountUsers(r)
}

func (s service) GetAll(r context.Context) (iterator.I[user.User], error) {
	return s.db.GetAllUsers(r)
}

func quotaOwnerName(u user.User) string {
	return strconv.FormatInt(u.Id, 10)
}

func (s service) updateUser(u user.User, w context.Context) error {
	// Check if is updating isOrganization field
	currentIsOrg, _, err := s.db.GetUserIsOrganization(w, u.Id)
	if err != nil {
		return fmt.Errorf("failed to load current isOrganization: %w", err)
	}
	if u.IsOrganization != currentIsOrg {
		return fmt.Errorf("isOrganization is immutable and cannot be updated")
	}
	return s.db.UpdateUser(w, u)
}

func (s service) RegisterNewUser(w context.Context,
	email, username, plainPassword string) (user.User, error) {
	// Check if email is valid and not taken
	if !isEmail(email) {
		return user.User{}, fmt.Errorf("%s is not a valid email", email)
	}
	_, notFoundErr, err := s.GetByEmail(w, email)
	if err != nil && !notFoundErr {
		return user.User{}, err
	}
	if !notFoundErr {
		return user.User{}, errors.New("email taken")
	}
	_, notFoundErr, err = s.GetByUsername(w, username)
	if err != nil && !notFoundErr {
		return user.User{}, err
	}
	if !notFoundErr {
		return user.User{}, errors.New("username taken")
	}

	// Check if the password is good
	if plainPassword == "" {
		return user.User{}, errors.New("password is required")
	}
	if len(plainPassword) < plainPasswordMinLength {
		return user.User{}, fmt.Errorf(
			"min of %v characters in password", plainPasswordMinLength)
	}
	if len(plainPassword) > plainPasswordMaxLength {
		return user.User{}, fmt.Errorf(
			"max of %v characters in password", plainPasswordMaxLength)
	}
	passwordHash := s.hashWithSalt(plainPassword)

	// Check if username is good
	if !UsernameIsValid(username) {
		return user.User{}, errInvalidUsername
	}

	u := SignupUserWithPassword(email, username, passwordHash)
	u.Id, err = s.db.CreateUser(w, u.Email, u.State, false, u.Username,
		u.PasswordHash, u.SelfPaidSubscription, u.SelfPaidSubscriptionQuantity)
	if err != nil {
		return user.User{}, fmt.Errorf("failed to insert new user: %s", err)
	}
	err = s.putJobLimits(u.Id, user.Subscription_None, w)
	if err != nil {
		return user.User{}, err
	}
	return u, nil
}

func (s service) RegisterNewUserFromOAuth(w context.Context, email string) (user.User, error) {
	//Check if email already exist
	_, notFoundErr, err := s.GetByEmail(w, email)
	if err != nil && !notFoundErr {
		return user.User{}, err
	}
	if !notFoundErr {
		return user.User{}, errors.New("email already used by another account")
	}

	u := SignupWithOAuth(email)
	u.Id, err = s.db.CreateUser(w, u.Email, u.State, false, u.Username,
		u.PasswordHash, u.SelfPaidSubscription, u.SelfPaidSubscriptionQuantity)
	if err != nil {
		return user.User{}, fmt.Errorf("failed to insert new oauth user: %s", err)
	}
	err = s.putJobLimits(u.Id, user.Subscription_None, w)
	if err != nil {
		return user.User{}, err
	}
	return u, nil
}

func (s service) CreateNewOrganizationUser(w context.Context, organizationUsername string) (user.User, error) {
	if !UsernameIsValid(organizationUsername) {
		return user.User{}, errInvalidUsername
	}
	_, notFoundErr, err := s.GetByUsername(w, organizationUsername)
	if err != nil && !notFoundErr {
		return user.User{}, err
	}
	if !notFoundErr {
		return user.User{}, errors.New("username already taken")
	}

	u := user.User{
		Email:          "",
		IsOrganization: true,
		State:          user.UserState_NoSubscription,
		Username:       organizationUsername,
	}
	u.Id, err = s.db.CreateUser(w, u.Email, u.State, u.IsOrganization,
		u.Username, u.PasswordHash, u.SelfPaidSubscription,
		u.SelfPaidSubscriptionQuantity)
	if err != nil {
		return user.User{}, err
	}
	return s.HandleManualPaymentSuccess(w, u.Id, user.Subscription_Trial, 1)
}

var errInvalidUsername = errors.New("invalid username")

func (s service) UpdateUsername(w context.Context, id int64, username string) (user.User, error) {
	if !UsernameIsValid(username) {
		return user.User{}, errInvalidUsername
	}

	_, isNotFoundErr, err := s.GetByUsername(w, username)
	if err != nil && !isNotFoundErr {
		return user.User{}, err
	}
	if !isNotFoundErr {
		return user.User{}, fmt.Errorf("username already taken")
	}
	u, _, err := s.Get(w, id)
	if err != nil {
		return user.User{}, err
	}
	if u.Username != "" {
		return user.User{}, errors.New("changing username is not allowed")
	}
	SetUsername(&u, username)

	return u, s.updateUser(u, w)
}
func (s service) ChooseUsernameAndStartTrial(w context.Context, id int64, username string) (user.User, error) {
	if !UsernameIsValid(username) {
		return user.User{}, errInvalidUsername
	}

	_, isNotFoundErr, err := s.GetByUsername(w, username)
	if err != nil && !isNotFoundErr {
		return user.User{}, err
	}
	if !isNotFoundErr {
		return user.User{}, fmt.Errorf("username already taken")
	}
	u, _, err := s.Get(w, id)
	if err != nil {
		return user.User{}, err
	}
	if u.Username != "" {
		return user.User{}, errors.New("changing username is not allowed")
	}
	SetUsername(&u, username)
	err = s.updateUser(u, w)
	if err != nil {
		return user.User{}, err
	}
	return s.HandleManualPaymentSuccess(w, id, user.Subscription_Trial, 1)
}
func (s service) UpdateCliKey(w context.Context, id int64, plainCliKey string) error {
	u, _, err := s.Get(w, id)
	if err != nil {
		return err
	}
	u.CliKeyHash = s.hashWithSalt(plainCliKey)
	return s.updateUser(u, w)
}
func (s service) DeleteCliKey(w context.Context, id int64) error {
	u, _, err := s.Get(w, id)
	if err != nil {
		return err
	}
	u.CliKeyHash = ""
	return s.updateUser(u, w)
}

func (s service) PasswordIsCorrect(u user.User, plainPassword string) bool {
	return u.PasswordHash == s.hashWithSalt(plainPassword)
}
func (s service) GetUserForPaymentWithStripe(
	w context.Context, id int64, priceId stripeclient.PriceId, quantity int64, forceCurrency string) (
	u user.User, isNotFoundErr bool, err error) {

	// Get user by id. If user.PaymentPlanIsActive, return an error
	u, isNotFoundErr, err = s.Get(w, id)
	if err != nil {
		return
	}
	if u.HasSub() {
		return u, false, errors.New("user already has sub")
	}

	// Deletes current session if needed.
	err = s.deleteCurrentStripeSessionIfItExists(&u)
	if err != nil {
		return u, false, err
	}

	// Get a stripe id if the user doesnt yet have one
	if u.StripeId == "" {
		stripeId, err := s.stripeClient.GetNewStripeCustomer()
		if err != nil {
			return u, false, err
		}
		u.StripeId = stripeId
		_, err = s.db.Bind(w).Exec(`
			UPDATE users2
			SET stripeId = ?
			WHERE id = ?
		`, stripeId, u.Id)
		if err != nil {
			return u, false, fmt.Errorf("failed to set stripe id: %s", err)
		}
	}

	// Creates a new session with stripe.
	stripeClientReferenceId, stripeSessionId, stripeSessionUrl, err :=
		s.stripeClient.GetNewStripeSession(u.Id, u.StripeId, priceId,
			quantity, forceCurrency)
	if err != nil {
		return
	}
	if stripeClientReferenceId != strconv.FormatInt(u.Id, 10) {
		panic(fmt.Sprintf(
			"stripe client created clientRefId %s for userId %d",
			stripeClientReferenceId, u.Id))
	}

	// Update and save the user
	StartStripePayment(&u, stripeSessionId, stripeSessionUrl, priceId, quantity)
	err = s.updateUser(u, w)
	if err != nil {
		return
	}
	return
}

func (s service) HandleStripeCheckoutSessionSuccess(w context.Context,
	id int64,
	stripeSubscriptionID string,
	stripeSessionId string,
	priceId stripeclient.PriceId,
	quantity int64) (user.User, error) {
	// Get user by id, if user not found return an error
	u, isNotFoundErr, err := s.Get(w, id)
	if isNotFoundErr {
		panic("stripe send payment of a unknown user")
	}
	if err != nil {
		return user.User{}, err
	}

	// The session was already paid. This is probably stripe replaying a
	// message for us. Lets just check and panic if not.
	if u.State == user.UserState_StripeSubscription {
		if u.SelfPaidSubscription != stripePriceIdToPaymentPlan(
			priceId, s.stripeClient) {
			panic("stripe send a invalid price id for paying user")
		}
		if u.SelfPaidSubscriptionQuantity != quantity {
			panic("stripe send a invalid quantity for paying user")
		}
		if u.StripeSubscriptionID != stripeSubscriptionID {
			panic("stripe send a invalid stripeSubscriptionID for paying user")
		}
		return u, nil
	}
	// Check quantity and priceId. They must be provided bc the user can
	// change them in the stripe portal
	if quantity < 1 {
		panic("invalid quantity")
	}
	plan := stripePriceIdToPaymentPlan(priceId, s.stripeClient)
	if plan == user.Subscription_Solo && quantity != 1 {
		panic("invalid quantity for solo plan")
	}

	_, err = s.db.Bind(w).Exec(`
		INSERT INTO stripe_subscriptions2 (
			stripeSubscriptionId, userId, isActive
		) VALUES (?, ?, TRUE) ;
	`, stripeSubscriptionID, u.Id)
	if err != nil {
		return user.User{}, fmt.Errorf("failed to insert stripe sub: %s", err)
	}

	var quota int64
	switch plan {
	case user.Subscription_Solo:
		quota = SoloStorageQuota
	case user.Subscription_Team:
		quota = TeamStorageQuota
	default:
		return user.User{}, fmt.Errorf("%d is not a valid plan", plan)
	}
	err = s.db.SetQuota(quotaOwnerName(u), quota)
	if err != nil {
		return user.User{}, fmt.Errorf("failed to set quota: %s", err)
	}
	err = s.putJobLimits(u.Id, plan, w)
	if err != nil {
		return user.User{}, err
	}

	// Update and save the user
	HandleStripePaymentCompleted(&u, plan, quantity, stripeSubscriptionID)
	err = s.updateUser(u, w)
	if err != nil {
		return user.User{}, fmt.Errorf("failed to update User: %s", err)
	}
	updatedUser, _, err := s.Get(w, u.Id)
	if err != nil {
		return user.User{}, fmt.Errorf("failed get updated user: %s", err)
	}
	return updatedUser, nil
}
func (s service) HandlesSubscriptionDeleted(
	w context.Context, stripeId, subscriptionId string) (user.User, error) {
	// Get user
	u, isNotFoundErr, err := s.GetByStripeId(w, stripeId)
	if isNotFoundErr {
		log.Printf("stripe send delete subscription of a unknown user. stripe id: %q", stripeId)
	}
	if err != nil {
		return user.User{}, err
	}
	// If the user has sub, it must mean we're deleting again (
	// replaying the subscription deleted msg). Let's just verify that.
	if !u.HasSub() {
		var isActive bool
		err = s.db.Bind(w).QueryRow(`
			SELECT
				isActive
			FROM
				stripe_subscriptions2
			WHERE
				stripeSubscriptionId = ?
		`, subscriptionId).Scan(&isActive)
		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, fmt.Errorf("stripe sub not found")
		}
		if err != nil {
			return user.User{}, fmt.Errorf("failed to query stripe sub: %s", err)
		}
		if isActive {
			panicF("stripeSub %s is active on user of inactive plan on db",
				subscriptionId)
		}
		return user.User{}, nil

	}
	if u.StripeSubscriptionID != subscriptionId {
		return user.User{}, fmt.Errorf("user with id %s and subId %s not found", stripeId, subscriptionId)
	}
	_, err = s.db.Bind(w).Exec(`
		UPDATE stripe_subscriptions2
		SET
			isActive = FALSE
		WHERE
			stripeSubscriptionId = ?
	`, subscriptionId)
	if err != nil {
		return user.User{}, fmt.Errorf("failed to inactivate stripe sub: %s", err)
	}
	err = s.db.FreezeQuota(quotaOwnerName(u))
	if err != nil {
		return user.User{}, fmt.Errorf("failed to freeze quota: %s", err)
	}
	err = s.putJobLimits(u.Id, user.Subscription_None, w)
	if err != nil {
		return user.User{}, err
	}

	DeleteStripeSubscription(&u)
	err = s.updateUser(u, w)
	if err != nil {
		return user.User{}, fmt.Errorf("failed to update User: %s", err)
	}
	updatedUser, _, err := s.Get(w, u.Id)
	if err != nil {
		return user.User{}, fmt.Errorf("failed get updated user: %s", err)
	}
	return updatedUser, nil
}

func (s service) HandleSubscriptionQuantityUpdated(w context.Context,
	stripeId, stripeSubscriptionId string, updatedQuantity int64) (user.User, error) {

	if updatedQuantity <= 0 {
		return user.User{}, fmt.Errorf("invalid quantity updating subscription for stripe id=%q. Quantity should be > 0 got=%v", stripeId, updatedQuantity)
	}
	// Get user
	u, _, err := s.GetByStripeId(w, stripeId)
	if err != nil {
		return user.User{}, fmt.Errorf("got error trying to get user in HandleSubscriptionQuantityUpdated() err=%w", err)
	}
	if u.SelfPaidSubscription == user.Subscription_Solo {
		return user.User{}, fmt.Errorf("can not update quantity of Solo plan. tried to for userId=%v", u.Id)
	}
	if u.StripeSubscriptionID != stripeSubscriptionId {
		return user.User{}, fmt.Errorf("user with id=%v and subId=%s not found", u.Id, stripeSubscriptionId)
	}
	if !u.HasSub() {
		return user.User{}, fmt.Errorf("user stripe id=%q does not have subscription to update", stripeId)
	}

	if updatedQuantity == u.SelfPaidSubscriptionQuantity {
		return u, nil
	}
	u.SelfPaidSubscriptionQuantity = updatedQuantity
	err = s.updateUser(u, w)
	if err != nil {
		return user.User{}, fmt.Errorf("failed to update User: %s", err)
	}
	updatedUser, _, err := s.Get(w, u.Id)
	if err != nil {
		return user.User{}, fmt.Errorf("failed get updated user: %s", err)
	}
	return updatedUser, nil
}

func (s service) HandleManualSubscriptionDeleted(w context.Context, userId int64) error {
	u, _, err := s.Get(w, userId)
	if err != nil {
		return err
	}
	if u.State != user.UserState_ManualSubscription {
		return fmt.Errorf("%d is not on a manual subscription", userId)
	}
	err = s.db.FreezeQuota(quotaOwnerName(u))
	if err != nil {
		return fmt.Errorf("failed to freeze quota: %s", err)
	}
	err = s.putJobLimits(u.Id, user.Subscription_None, w)
	if err != nil {
		return err
	}
	DeleteManualSubscription(&u)
	return s.updateUser(u, w)
}

// Calls stripe to cancel the session of the user and updates the relevant
// fieds on the user
func (s service) deleteCurrentStripeSessionIfItExists(u *user.User) error {
	// Nothing to cancel if not paying
	if u.State != user.UserState_PayingWithStripe {
		return nil
	}
	err := s.stripeClient.ExpireStripeSession(u.StripeSessionId)
	if err != nil {
		return err
	}
	StopPayingWithStripe(u)
	return nil
}

func (s service) HandleManualPaymentSuccess(w context.Context, userId int64,
	plan user.SubscriptionPlan, quantity int64) (user.User, error) {
	// Validate parameters
	if quantity < 1 {
		return user.User{}, fmt.Errorf("%d is an invalid quantity", quantity)
	}
	if plan == user.Subscription_Solo && quantity != 1 {
		return user.User{}, fmt.Errorf("%d is an invalid quantity for solo plan", quantity)
	}

	// Get the user
	u, _, err := s.Get(w, userId)
	if err != nil {
		return user.User{}, err
	}
	if u.HasSub() {
		return user.User{}, errors.New("user already has sub")
	}

	// If user is paying via stripe, cancel that session first
	err = s.deleteCurrentStripeSessionIfItExists(&u)
	if err != nil {
		return user.User{}, err
	}

	var quota int64
	switch plan {
	case user.Subscription_Trial:
		quota = TrialStorageQuota
	case user.Subscription_Solo:
		quota = SoloStorageQuota
	case user.Subscription_Team:
		quota = TeamStorageQuota
	default:
		return user.User{}, fmt.Errorf("%d is not a valid plan", plan)
	}
	err = s.db.SetQuota(quotaOwnerName(u), quota)
	if err != nil {
		return user.User{}, fmt.Errorf("failed to set quota: %s", err)
	}
	err = s.putJobLimits(u.Id, plan, w)
	if err != nil {
		return user.User{}, err
	}

	ManuallyPayForPlan(&u, plan, quantity)
	err = s.updateUser(u, w)
	if err != nil {
		return user.User{}, fmt.Errorf("failed to update User: %s", err)
	}
	updatedUser, _, err := s.Get(w, u.Id)
	if err != nil {
		return user.User{}, fmt.Errorf("failed get updated user: %s", err)
	}
	return updatedUser, nil
}

func (s service) ManualReset(w context.Context, userId int64) error {
	u, _, err := s.Get(w, userId)
	if err != nil {
		return err
	}

	if u.StripeSessionId != "" {
		err = s.stripeClient.ExpireStripeSession(u.StripeSessionId)
		if err != nil {
			return fmt.Errorf("failed to expire stripe session %s: %s",
				u.StripeSessionId, err)
		}
	}
	if u.StripeSubscriptionID != "" {
		_, err = s.db.Bind(w).Exec(`
		UPDATE stripe_subscriptions2
		SET
			isActive = FALSE
		WHERE
			stripeSubscriptionId = ?
	`, u.StripeSubscriptionID)
		if err != nil {
			return fmt.Errorf("failed to inactivate stripe sub: %s", err)
		}
	}
	err = s.db.FreezeQuota(quotaOwnerName(u))
	if err != nil {
		return fmt.Errorf("failed to freeze quota: %s", err)
	}
	err = s.putJobLimits(u.Id, user.Subscription_None, w)
	if err != nil {
		return err
	}

	ResetManually(&u)
	return s.updateUser(u, w)
}

func isEmail(s string) bool {
	_, err := mail.ParseAddress(s)
	return err == nil
}

var minLenOfUsername = 2
var maxLenOfUsername = 20

// 1- ^[a-z] → must start with a lowercase letter.
// 2- [a-z0-9_-] → allowed characters after the first letter: lowercase
// letters, digits, underscore, dash.
// 3- {%d,%d} → at least `minLenOfUsername` characters and max maxLenOfUsername.
// 4- ^…$ → ensures the whole string follows the pattern.
var usernameRegex = regexp.MustCompile(fmt.Sprintf(
	`^[a-z][a-z0-9_-]{%d,%d}$`,
	minLenOfUsername-1,
	maxLenOfUsername-1,
))

func (s *service) hashWithSalt(plainText string) string {
	return HashWithSalt(plainText, s.salt)
}

// Panics if price id is not PriceId_Team or PriceId_Solo
func stripePriceIdToPaymentPlan(pId stripeclient.PriceId, sCLient stripeclient.StripeClient) user.SubscriptionPlan {
	product, isOk := sCLient.ResolvePriceId(pId)
	if !isOk {
		panic("could not resolve price id")
	}
	switch product {
	case stripeclient.Product_Subscription_Team:
		return user.Subscription_Team
	case stripeclient.Product_Subscription_Solo:
		return user.Subscription_Solo
	default:
		panic("invalid price id")
	}
}

func (s service) putJobLimits(userId int64, plan user.SubscriptionPlan, w context.Context) error {
	var maxParallelJobs int
	var maxParallelJobsTimeout time.Duration
	switch plan {
	case user.Subscription_None:
		maxParallelJobs = 0
		maxParallelJobsTimeout = 0
	case user.Subscription_Trial:
		maxParallelJobs = TrialMaxParallelJobs
		maxParallelJobsTimeout = TrialMaxParallelTimeoutSum
	case user.Subscription_Solo:
		maxParallelJobs = SoloMaxParallelJobs
		maxParallelJobsTimeout = SoloMaxParallelTimeoutSum
	case user.Subscription_Team:
		maxParallelJobs = TeamMaxParllelJobs
		maxParallelJobsTimeout = TeamMaxParallelTimeoutSum
	default:
		panic(fmt.Sprintf("unexpected plan %d", plan))
	}
	err := s.js.PutLimits(userId, maxParallelJobs, maxParallelJobsTimeout, w)
	if err != nil {
		return fmt.Errorf("failed to job limits: %s", err)
	}
	return nil
}
