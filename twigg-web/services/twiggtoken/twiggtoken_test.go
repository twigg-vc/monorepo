package twiggtoken

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

type fakeSigner struct{}

func (m fakeSigner) SignAndAppend(msg string) string {
	return msg + "sig=1234"
}
func (m fakeSigner) VerifyAndExtract(signed string) (msg string, isOk bool) {
	if len(signed) < len("sig=1234") {
		return "", false
	}
	if !strings.HasSuffix(signed, "sig=1234") {
		return "", false
	}
	return strings.TrimSuffix(signed, "sig=1234"), true
}

func TestTwiggToken_RoundTrip(t *testing.T) {

	duration := 5 * time.Minute
	original := ParsedToken{
		RepoId:         123,
		CommitServerId: 99,
		CommitVersion:  42,
		Actions:        []TokenAction{TokenActionPush, TokenActionPull},
		ActionsArg:     []string{"1", "1"},
		ExpiresAt:      time.Now().Add(duration),
	}

	token, err := NewTwiggToken(
		original.RepoId,
		original.CommitServerId,
		original.CommitVersion,
		original.Actions,
		original.ActionsArg,
		duration,
		fakeSigner{},
	)
	if err != nil {
		t.Fatalf("NewTwiggToken() error: %v", err)
	}

	if !strings.HasPrefix(token, Prefix) {
		t.Fatalf("expected token to start with prefix %q, got %q", Prefix, token)
	}

	parsed, isExpiredErr, err := ParseToken(token, fakeSigner{})
	if err != nil {
		t.Fatalf("ParseToken() error: %v", err)
	}
	if isExpiredErr {
		t.Fatalf("token is not expired")
	}

	if parsed.RepoId != original.RepoId ||
		parsed.CommitServerId != original.CommitServerId ||
		parsed.CommitVersion != original.CommitVersion ||
		!slices.Equal(parsed.Actions, original.Actions) ||
		!slices.Equal(parsed.ActionsArg, original.ActionsArg) {
		t.Fatalf("parsed token mismatch:\nwant: %+v\ngot: %+v", original, parsed)
	}

	// This checks parsed token’s expiration timestamp is roughly equal to the
	// one you expected.
	diff := parsed.ExpiresAt.Sub(original.ExpiresAt)
	if diff < -20*time.Microsecond || diff > 20*time.Microsecond {
		t.Fatalf("ExpiresAt mismatch: want=%v got=%v", original.ExpiresAt, parsed.ExpiresAt)
	}
}

func TestTwiggToken_InvalidPrefixAndEmptyToken(t *testing.T) {
	token, err := NewTwiggToken(
		123,
		99,
		42,
		[]TokenAction{TokenActionPush},
		[]string{""},
		5*time.Second,
		fakeSigner{},
	)

	_, _, err = ParseToken("", fakeSigner{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	withoutPrefix := token[len(Prefix):]
	fakePrefix := "tw_fak_"
	tokenWithFakePrefix := fakePrefix + withoutPrefix

	_, _, err = ParseToken(tokenWithFakePrefix, fakeSigner{})
	if err == nil {
		t.Fatal("expected error for invalid prefix, got nil")
	}
}

func TestTwiggToken_TamperedSignature(t *testing.T) {
	token, err := NewTwiggToken(1, 2, 3, []TokenAction{TokenActionPull}, []string{""}, time.Hour, fakeSigner{})
	if err != nil {
		t.Fatalf("NewTwiggToken error: %v", err)
	}

	tampered := token + "xyz" // break signature

	_, _, err = ParseToken(tampered, fakeSigner{})
	if err == nil {
		t.Fatal("expected signature error, got nil")
	}
}

func TestTwiggToken_CorruptedBase64(t *testing.T) {
	token, err := NewTwiggToken(1, 2, 3, []TokenAction{TokenActionPull}, []string{""}, time.Hour, fakeSigner{})
	if err != nil {
		t.Fatalf("NewTwiggToken error: %v", err)
	}

	// Replace last chars with something not base64-valid
	corrupted := token[:len(token)-3] + "###"

	_, _, err = ParseToken(corrupted, fakeSigner{})
	if err == nil {
		t.Fatal("expected base64 decode error, got nil")
	}
}

func TestTwiggToken_MultipleActions(t *testing.T) {
	actions := []TokenAction{TokenActionPull, TokenActionPush}
	actionsArg := []string{"a", "b"}

	token, err := NewTwiggToken(5, 10, 20, actions, actionsArg, time.Hour, fakeSigner{})
	if err != nil {
		t.Fatalf("NewTwiggToken(%s) error: %v", actions, err)
	}

	out, isExpiredErr, err := ParseToken(token, fakeSigner{})
	if err != nil {
		t.Fatalf("ParseToken(%s) error: %v", actions, err)
	}
	if isExpiredErr {
		t.Fatalf("token is not expired")
	}

	if !slices.Equal(out.Actions, actions) {
		t.Fatalf("expected actions %v, got %v", actions, out.Actions)
	}
	if out.ActionsArg == nil {
		t.Fatalf("expected ActionsArg to be != nil, got %s", out.ActionsArg)
	}
	if !slices.Equal(out.ActionsArg, actionsArg) {
		t.Fatalf("expected actions %v, got %v", actions, out.Actions)
	}
}

func TestTwiggToken_RandomizedValues(t *testing.T) {
	actions := []TokenAction{TokenActionPull, TokenActionPush}
	actionsArg := []string{"", ""}

	for i := uint64(1); i <= 10; i++ {
		token, err := NewTwiggToken(i, i+10, i+20, actions, actionsArg, time.Hour, fakeSigner{})
		if err != nil {
			t.Fatalf("NewTwiggToken(%d) error: %v", i, err)
		}

		out, isExpiredErr, err := ParseToken(token, fakeSigner{})
		if err != nil {
			t.Fatalf("ParseToken(%d) error: %v", i, err)
		}
		if isExpiredErr {
			t.Fatalf("token is not expired")
		}

		if out.RepoId != i || out.CommitServerId != i+10 || out.CommitVersion != i+20 {
			t.Fatalf("decoded mismatch for %d: %+v", i, out)
		}
	}
}
func TestTwiggToken_ExpiredToken(t *testing.T) {
	actions := []TokenAction{TokenActionPull, TokenActionPush}
	actionsArg := []string{"", ""}

	duration := 1 * time.Microsecond
	token, err := NewTwiggToken(1, 1, 1, actions, actionsArg, duration, fakeSigner{})
	if err != nil {
		t.Fatalf("NewTwiggToken error: %v", err)
	}

	// wait until it expires
	time.Sleep(2 * time.Millisecond)

	parsed, isExpiredErr, err := ParseToken(token, fakeSigner{})
	if err == nil {
		t.Fatal("expected expiration error, got nil")
	}
	if !isExpiredErr {
		t.Fatal("expected isExpiredErr=true")
	}
	if parsed.RepoId != 1 ||
		parsed.CommitServerId != 1 ||
		parsed.CommitVersion != 1 ||
		!slices.Equal(parsed.Actions, actions) ||
		!slices.Equal(parsed.ActionsArg, actionsArg) {
		t.Fatalf("parsed token mismatch after expiration: %+v", parsed)
	}
}

func TestTwiggToken_ActionsLenMismatch(t *testing.T) {
	_, err := NewTwiggToken(
		1,
		1,
		1,
		[]TokenAction{TokenActionPull},
		[]string{"", ""},
		time.Hour,
		fakeSigner{},
	)
	if err == nil {
		t.Fatal("expected error for mismatched actions/actionsArg length")
	}
}
func TestTwiggToken_EmptyActions(t *testing.T) {
	_, err := NewTwiggToken(
		1,
		1,
		1,
		[]TokenAction{},
		[]string{},
		time.Hour,
		fakeSigner{},
	)
	if err == nil {
		t.Fatal("expected error for empty actions")
	}
}
func TestTwiggToken_ManyActions(t *testing.T) {
	var actions []TokenAction
	var args []string

	for i := 0; i < 100; i++ {
		actions = append(actions, TokenActionPull)
		args = append(args, "")
	}

	token, err := NewTwiggToken(1, 2, 3, actions, args, time.Hour, fakeSigner{})
	if err != nil {
		t.Fatalf("NewTwiggToken error: %v", err)
	}

	out, expired, err := ParseToken(token, fakeSigner{})
	if err != nil || expired {
		t.Fatalf("ParseToken error: %v", err)
	}
	if !slices.Equal(out.Actions, actions) {
		t.Fatal("actions mismatch")
	}
}

func TestTwiggTokenHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	SetTwiggTokenInHeader("test-token", r)

	got := GetTwiggTokenInHeader(r)
	if got != "test-token" {
		t.Fatalf("expected token %q, got %q", "test-token", got)
	}
}