package user

func (s UserState) String() string {
	switch s {
	case UserState_None:
		return "none"
	case UserState_NotSignedUp:
		return "NotSignedUp"
	case UserState_NoUsername:
		return "NoUsername"
	case UserState_NoSubscription:
		return "InactivePaymentPlan"
	case UserState_PayingWithStripe:
		return "PayingWithStripe"
	case UserState_StripeSubscription:
		return "StripeSubscription"
	case UserState_ManualSubscription:
		return "ManualSubscription"
	default:
		panic("unknown state")
	}
}
