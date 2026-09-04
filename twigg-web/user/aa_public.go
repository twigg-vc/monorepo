package user

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
