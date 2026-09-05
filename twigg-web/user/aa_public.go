package user

import (
	"fmt"
	"monorepo/twigg-web/services/stripeclient"
)

type User struct {
	Id             int64
	Email          string
	State          UserState
	IsOrganization bool

	// If !="", identifies user
	CliKeyHash string
	// If !="", identifies user
	StripeId string

	Username     string
	PasswordHash string

	SelfPaidSubscription         SubscriptionPlan
	SelfPaidSubscriptionQuantity int64

	StripeSessionId       string
	StripeSessionUrl      string
	StripeSessionPriceId  stripeclient.PriceId
	StripeSessionQuantity int64 // Quantity used to create session
	StripeSubscriptionID  string

	QuotaUsed      int64 // number of blob bytes used
	TotalQuota     int64 // number of blob bytes "purchased"
	QuotaLimmitted int64 // number of blob bytes that failed to write due to "no quota"
}

func (u User) HasSub() bool {
	return u.SelfPaidSubscription != Subscription_None
}

func (u User) MustUpgradeSelfPaidSub(
	hasMoreThanTwoNonArchivedRepos bool) bool {
	if u.QuotaUsed > u.TotalQuota || u.QuotaLimmitted > 0 {
		return true
	}
	switch u.SelfPaidSubscription {
	case Subscription_None, Subscription_Trial:
		return hasMoreThanTwoNonArchivedRepos
	case Subscription_Solo, Subscription_Team:
		return false
	default:
		panic(fmt.Sprintf("unknown sub %d", u.SelfPaidSubscription))
	}
}
func (u User) CanCreateRepo(hasTwoOrMoreReposAlready bool) bool {
	if u.SelfPaidSubscription == Subscription_None {
		return false
	}
	if u.SelfPaidSubscription == Subscription_Trial {
		return !hasTwoOrMoreReposAlready
	}
	return true
}

type UserState uint32

const (
	// Invalid state that is never used. Only exists to serve as undefined.
	UserState_None UserState = iota
	// User hasn't even signed up yet. This is only used so that we can use
	// the same User struct to only carry the user's email.
	UserState_NotSignedUp
	// User hasn't completely finished the signup yet as it has no username
	UserState_NoUsername
	// User has completed the signup, but has not paid for a subscription
	UserState_NoSubscription
	// User has completed the signup and is paying with Stripe
	UserState_PayingWithStripe
	// User has an stripe subscription
	UserState_StripeSubscription
	// User has a subscription that was paid manually (currently it never
	// expires but we probably want to change that eventually).
	UserState_ManualSubscription
)

type SubscriptionPlan int

const (
	Subscription_None SubscriptionPlan = iota
	Subscription_Solo
	Subscription_Team
	Subscription_Trial
)