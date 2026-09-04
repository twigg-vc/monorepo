package keys

import "testing"

func Test(t *testing.T) {
	s := New()
	k0 := s.NewRandomCliKey()
	if len(k0) < 32 {
		t.Fatal("got short key")
	}
	k1 := s.NewRandomCliKey()
	if k0 == k1 {
		t.Fatal("got identical keys")
	}
}

func TestPrefix(t *testing.T) {
	s := New()
	key := s.NewRandomCliKey()
	kPrefix := key[:len(cliKeyPrefix)]
	if kPrefix != cliKeyPrefix {
		t.Fatal("wrong prefix")
	}
}

func TestMock(t *testing.T) {
	s := NewMock()
	k0 := s.NewRandomCliKey()
	if k0 != s.GetLastRandomCliKey() {
		t.Fatal("wrong last key0")
	}
	k1 := s.NewRandomCliKey()
	if k0 == k1 {
		t.Fatal("got identical keys")
	}
	if k1 != s.GetLastRandomCliKey() {
		t.Fatal("wrong last key1")
	}
}