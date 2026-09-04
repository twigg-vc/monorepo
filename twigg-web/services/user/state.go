package user

import (
	"fmt"
	"monorepo/twigg-web/services/stripeclient"
	"monorepo/twigg-web/user"
)

// Id is not populated
func SignupUserWithPassword(email, username, passwordHash string) User {
	u := User{
		State:        user.UserState_NoSubscription,
		Email:        email,
		Username:     username,
		PasswordHash: passwordHash,
	}
	u.CheckStateOrDie()
	return u
}

// Id is not populated
func SignupWithOAuth(email string) User {
	u := User{
		State: user.UserState_NoUsername,
		Email: email,
	}
	u.CheckStateOrDie()
	return u
}

func (u *User) SetUsername(username string) {
	u.Username = username
	if u.State == user.UserState_NoUsername {
		u.State = user.UserState_NoSubscription
	}
	u.CheckStateOrDie()
}

func (u *User) ResetManually() {
	// Some of these fields are saved on other tables, but this resets them
	// anyway
	u.State = user.UserState_NoSubscription
	u.CliKeyHash = ""
	u.SelfPaidSubscription = Subscription_None
	u.SelfPaidSubscriptionQuantity = 0
	u.StripeSessionId = ""
	u.StripeSessionUrl = ""
	u.StripeSessionPriceId = ""
	u.StripeSessionQuantity = 0
	u.StripeSubscriptionID = ""
	u.TotalQuota = u.QuotaUsed
	u.CheckStateOrDie()
}

func (u *User) ManuallyPayForPlan(plan SubscriptionPlan, quantity int64) {
	if u.State != user.UserState_NoSubscription {
		panicF("cant ManuallyPayForPlan on state %s", u.State)
	}
	u.State = user.UserState_ManualSubscription
	u.SelfPaidSubscription = plan
	u.SelfPaidSubscriptionQuantity = quantity
	u.CheckStateOrDie()
}

func (u *User) StartStripePayment(
	StripeSessionId,
	StripeSessionUrl string,
	StripeSessionPriceId stripeclient.PriceId,
	StripeSessionQuantity int64) {
	if u.State == user.UserState_NoUsername {
		panicF("cant StartStripePayment on state %s", u.State)
	}
	if u.StripeId == "" {
		panicF("stripeId must be set before calling StartStripePayment")
	}
	u.State = user.UserState_PayingWithStripe
	u.StripeSessionId = StripeSessionId
	u.StripeSessionUrl = StripeSessionUrl
	u.StripeSessionPriceId = StripeSessionPriceId
	u.StripeSessionQuantity = StripeSessionQuantity
	u.CheckStateOrDie()
}

func (u *User) StopPayingWithStripe() {
	if u.State != user.UserState_PayingWithStripe {
		panicF("cant cancelStripePayment on state %s", u.State)
	}
	u.State = user.UserState_NoSubscription
	u.StripeSessionId = ""
	u.StripeSessionUrl = ""
	u.StripeSessionPriceId = ""
	u.StripeSessionQuantity = 0
	u.CheckStateOrDie()
}

func (u *User) DeleteStripeSubscription() {
	if u.State != user.UserState_StripeSubscription {
		panicF("cant DeleteStripeSubscription on state %s", u.State)
	}

	u.DeleteSubscription()
	u.StripeSubscriptionID = ""
	u.StripeSessionId = ""
	u.StripeSessionUrl = ""
	u.StripeSessionPriceId = ""
	u.StripeSessionQuantity = 0
	u.CheckStateOrDie()
}
func (u *User) DeleteManualSubscription() {
	if u.State != user.UserState_ManualSubscription {
		panicF("cant DeleteManualSubscription on state %s", u.State)
	}
	u.DeleteSubscription()
	u.CheckStateOrDie()
}

func (u *User) HandleStripePaymentCompleted(
	plan SubscriptionPlan, quantity int64, stripeSubscriptionId string) {
	if u.State != user.UserState_PayingWithStripe {
		panicF("cant handleStripePayment on state %s", u.State)
	}

	u.State = user.UserState_StripeSubscription
	u.SelfPaidSubscription = plan
	u.SelfPaidSubscriptionQuantity = quantity
	u.StripeSubscriptionID = stripeSubscriptionId
	u.StripeSessionId = ""
	u.StripeSessionUrl = ""
	u.StripeSessionPriceId = ""
	u.StripeSessionQuantity = 0
	u.CheckStateOrDie()
}

// helper to set all properties that need to be set when a user
// cancels any kind of subscription
func (u *User) DeleteSubscription() {
	u.State = user.UserState_NoSubscription
	u.SelfPaidSubscription = Subscription_None
	u.SelfPaidSubscriptionQuantity = 0
}

const MaxUsersInSoloPlan = 3
const MaxUsersInTrialPlan = 1

// Function used to validade the fields of a user given its state.
// It panics if it finds any inconsistency.
func (u User) CheckStateOrDie() {
	s := u.State

	if s == user.UserState_None {
		panicF("got user with None state")
	}
	if u.Email == "" && !u.IsOrganization {
		panicF("%s without email", s)
	}
	if s == user.UserState_NotSignedUp {
		// The NotSignedUp state only exists so that the User struct can
		// be used as a "container" for the email of an account that hasn't
		// signed up yet.
		panicF("got user with not signed up state")
	}

	switch s {
	case user.UserState_None:
	case user.UserState_NoUsername:
		if u.Username != "" {
			panicF("%s with username", s)
		}
	case user.UserState_NoSubscription:
		if u.HasSub() {
			panicF("%s with sub", s)
		}
	case user.UserState_PayingWithStripe:
		if u.HasSub() {
			panicF("%s with sub", s)
		}
		if u.StripeId == "" {
			panicF("%s without stripe id", s)
		}
		if u.StripeSessionId == "" ||
			u.StripeSessionUrl == "" ||
			u.StripeSessionPriceId == "" ||
			u.StripeSessionQuantity == 0 {
			panicF("%s without required fields", s)
		}
	case user.UserState_StripeSubscription:
		if !u.HasSub() {
			panicF("%s without sub", s)
		}
		if u.StripeId == "" {
			panicF("%s without stripe id", s)
		}
		if u.StripeSubscriptionID == "" {
			panicF("%s without subscription id", s)
		}
	case user.UserState_ManualSubscription:
		if !u.HasSub() {
			panicF("%s without sub", s)
		}
		if u.Username == "" {
			panicF("%s without username", s)
		}
	default:
		panic("unknown state")
	}
}

func panicF(format string, a ...any) {
	panic(fmt.Sprintf(format, a...))
}
