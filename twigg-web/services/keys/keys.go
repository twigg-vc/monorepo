package keys

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

type service struct{}

func newService() Service {
	return service{}
}

func (s service) NewRandomCliKey() string {
	const numberOfBytesOfCliKey = 32
	b := make([]byte, numberOfBytesOfCliKey)
	_, err := rand.Read(b)
	if err != nil {
		// rand.Read guarantees an error is never returned
		panic(fmt.Sprintf("rand.Read err: %s", err))
	}
	// Encode to base64 to make it URL-safe
	return cliKeyPrefix + base64.RawURLEncoding.EncodeToString(b)
}

type serviceMock struct {
	keys    []string
	lastKey int
}

func newMock() *serviceMock {
	return &serviceMock{
		keys:    nil,
		lastKey: 0,
	}
}

func (s *serviceMock) NewRandomCliKey() string {
	s.keys = append(s.keys, strings.Repeat(fmt.Sprintf("%d", s.lastKey), 5))
	s.lastKey += 1
	return s.GetLastRandomCliKey()
}
func (s *serviceMock) GetLastRandomCliKey() string {
	return s.keys[len(s.keys)-1]
}

const cliKeyPrefix string = "tw_key_"
