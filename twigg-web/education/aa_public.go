package education

type UserEducation struct {
	UserId          int64
	WelcomeWasShown bool
}

func NewUserEducation(userId int64) UserEducation {
	return UserEducation{
		UserId:          userId,
		WelcomeWasShown: false,
	}
}