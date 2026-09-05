package keys

type Service struct {
	s service
}

func (s Service) NewRandomCliKey() string {
	return s.s.NewRandomCliKey()
}

func New() Service {
	return Service{service{}}
}

type MockService struct {
	s *serviceMock
}

func (s MockService) NewRandomCliKey() string {
	return s.s.NewRandomCliKey()
}

// Returns the last generated key.
func (s MockService) GetLastRandomCliKey() string {
	return s.s.GetLastRandomCliKey()
}

func NewMock() MockService {
	return MockService{newMock()}
}
