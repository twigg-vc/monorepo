package user

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"monorepo/base/iterator"
	"monorepo/twigg-web/services/stripeclient"
	"monorepo/twigg-web/user"
	"time"
)

type Service interface {
	Get(r context.Context, id int64) (u user.User, isNotFoundErr bool, err error)
	GetByEmail(r context.Context, email string) (u user.User, isNotFoundErr bool, err error)
	GetByUsername(r context.Context, username string) (u user.User, isNotFoundErr bool, err error)
	GetByStripeId(r context.Context, stripeId string) (u user.User, isNotFoundErr bool, err error)
	GetUsername(userId int64, tx context.Context) (string, error)

	// Used to create a new user with email, username and password
	RegisterNewUser(w context.Context,
		email, username, plainPassword string) (user.User, error)
	// Use when creating a user from a OAuth, where we only have email.
	RegisterNewUserFromOAuth(w context.Context, email string) (user.User, error)

	CreateNewOrganizationUser(w context.Context, organizationUsername string) (user.User, error)

	// Returns updated User
	UpdateUsername(w context.Context, id int64, username string) (user.User, error)

	// Returns updated User
	ChooseUsernameAndStartTrial(w context.Context, id int64, username string) (user.User, error)

	// Returns the total num of users
	CountAll(r context.Context) (int64, error)
	// Returns all users, one by one
	GetAll(r context.Context) (it iterator.I[user.User], err error)
	// Returns true if the provided plain password (not hash) is correct
	PasswordIsCorrect(u user.User, plainPassword string) bool

	// Update User cliKeyHash.
	UpdateCliKey(w context.Context, id int64, plainCliKey string) error
	// Update User.CliKeyHash to ""
	DeleteCliKey(w context.Context, id int64) error
	// Returns a user by its cliKey
	GetByCliKey(r context.Context, plainCliKey string) (
		u user.User, isNotFoundErr bool, err error)

	// Does all the necessary setup (if any) and returns a user who is ready
	// to perform payment with stripe. Check user.StripeSessionUrl for the
	// redirect URL. If user.PaymentPlanIsActive returns error.
	// forceCurrency is used to force a currency (usd/brl); it's ignored if invalid
	GetUserForPaymentWithStripe(w context.Context, id int64,
		priceId stripeclient.PriceId, quantity int64, forceCurrency string) (u user.User, isNotFoundErr bool, err error)

	// It updates the user's record to mark the payment plan as active and
	// stores the relevant Stripe subscription information. Only use it when
	// handling stripe checkout session success. We expect that stripe send us
	// values in a expected manner, otherwise it panics.
	HandleStripeCheckoutSessionSuccess(w context.Context,
		id int64, stripeSubscriptionID, stripeSessionId string,
		priceId stripeclient.PriceId, quantity int64) (user.User, error)

	// It updates the user's record to mark their payment plan as inactive and
	// change the relevant stripe subscription information. Only use it when
	// handling stripe webhook customer.subscription.deleted event. We expect
	//  that stripe send us values in a expected manner, otherwise it panics.
	HandlesSubscriptionDeleted(w context.Context,
		stripeId, subscriptionId string) (user.User, error)

	// It updates the user's record to mark their plan quantity updated version
	HandleSubscriptionQuantityUpdated(w context.Context,
		stripeId, subscriptionId string, updatedQuantity int64) (user.User, error)

	// Handles a "manual payment" success, i.e. someone "manually" paying us
	// with cash or something like that.
	HandleManualPaymentSuccess(w context.Context, userId int64,
		sub user.SubscriptionPlan, subQuantity int64) (user.User, error)

	// Handles the deletion of a manual subscription
	HandleManualSubscriptionDeleted(w context.Context, userId int64) error

	// "Manually" reset a user to be on a no subscription
	ManualReset(w context.Context, userId int64) error
}

func UsernameIsValid(username string) bool {
	return usernameRegex.MatchString(username)
}
func IsEmail(s string) bool {
	return isEmail(s)
}

// Constructor of the service
func NewService(js JobLimitSetter,
	stripeClient stripeclient.StripeClient, db Db, salt string) (Service, error) {
	return newService(js, stripeClient, db, salt)
}

// The storage the service needs.
type Db interface {
	CreateUser(writeCtx context.Context, email string, state user.UserState,
		isOrganization bool, username, passwordHash string,
		selfPaidSubscription user.SubscriptionPlan,
		selfPaidSubscriptionQuantity int64) (userId int64, err error)
	UpdateUser(writeCtx context.Context, u user.User) error
	SetUserStripeId(writeCtx context.Context, userId int64, stripeId string) error

	GetUser(ctx context.Context, userId int64) (u user.User, isNotFoundErr bool, err error)
	GetUserByEmail(ctx context.Context, email string) (u user.User, isNotFoundErr bool, err error)
	GetUserByUsername(ctx context.Context, username string) (u user.User, isNotFoundErr bool, err error)
	GetUserByStripeId(ctx context.Context, stripeId string) (u user.User, isNotFoundErr bool, err error)
	GetUserByCliKeyHash(ctx context.Context, cliKeyHash string) (u user.User, isNotFoundErr bool, err error)
	GetUsername(ctx context.Context, userId int64) (username string, isNotFoundErr bool, err error)
	GetUserIsOrganization(ctx context.Context, userId int64) (isOrganization bool, isNotFoundErr bool, err error)
	CountUsers(ctx context.Context) (int64, error)
	GetAllUsers(ctx context.Context) (iterator.I[user.User], error)

	UpsertStripeSubscription(writeCtx context.Context, stripeSubscriptionId string,
		userId int64, isActive bool) error
	GetStripeSubscriptionIsActive(ctx context.Context,
		stripeSubscriptionId string) (isActive bool, isNotFoundErr bool, err error)

	UserQuotaOwnerName(userId int64) string
	SetQuota(quotaOwner string, nBytes int64) error
	FreezeQuota(quotaOwner string) error
}

var (
	SoloStorageQuota  = int64(10 * 1024 * 1024 * 1024) // 10 GB
	TeamStorageQuota  = int64(50 * 1024 * 1024 * 1024) // 50 GB
	TrialStorageQuota = int64(250 * 1024 * 1024)       // 250 MB
)

const (
	TrialMaxJobTimeout         = time.Duration(15) * time.Minute // limit for a single job
	TrialMaxParallelJobs       = 1                               // num of jobs running in parallel
	TrialMaxParallelTimeoutSum = time.Duration(1) * time.Minute  // sum of timeout of jobs running in parallel

	SoloMaxJobTimeout         = time.Duration(1) * time.Hour
	SoloMaxParallelJobs       = 1
	SoloMaxParallelTimeoutSum = time.Duration(1) * time.Minute

	TeamMaxJobTimeout         = time.Duration(3) * time.Hour
	TeamMaxParllelJobs        = 3
	TeamMaxParallelTimeoutSum = time.Duration(3) * time.Hour
)

type JobLimitSetter interface {
	PutLimits(ownerId int64, maxJobs int,
		maxTimeout time.Duration, tx context.Context) error
}

// Function used to hash passwords. Panics for empty salt.
func HashWithSalt(plainText, salt string) string {
	if salt == "" {
		panic("used empty salt")
	}
	hash := sha256.Sum256([]byte(salt + plainText))
	return hex.EncodeToString(hash[:])
}
