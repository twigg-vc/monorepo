package featureflags

import (
	"context"
	"errors"
	"testing"
)

func TestFlagsByUserIdHelper(t *testing.T) {
	u := fakeUserService{
		userIdToUsername: map[int64]string{
			1: "andre",
		},
	}
	helper := NewFlagsHelper("mock", u)
	f0, err := helper.GetFlagsByRepoOwnerUserId(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	f1 := GetFlags("mock", "andre", "")
	if f0 != f1 {
		t.Fatalf("flag mismatch")
	}

	f0, err = helper.GetFlagsByRepoOwnerUsername("andre", nil)
	if err != nil {
		t.Fatal(err)
	}
	if f0 != f1 {
		t.Fatalf("flag mismatch")
	}
}

type fakeUserService struct {
	userIdToUsername map[int64]string
}

func (u fakeUserService) GetUsername(userId int64, tx context.Context) (string, error) {
	if u.userIdToUsername == nil {
		return "", errors.New("not found")
	}
	uName, ok := u.userIdToUsername[userId]
	if !ok {
		return "", errors.New("not found")
	}
	return uName, nil
}
