package keys

type Service interface {
	NewRandomCliKey() string
}

func New() Service {
	return newService()
}

type MockService interface {
	Service
	// Returns the last generated key.
	GetLastRandomCliKey() string
}

func NewMock() MockService {
	return newMock()
}
