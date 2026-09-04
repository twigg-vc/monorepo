package sign

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
)

func signAndAppend(msg string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	_, err := mac.Write([]byte(msg))
	if err != nil {
		panic(fmt.Sprintf("SignAndAppend failed:%s", err))
	}
	sig := mac.Sum(nil)
	encoded := base64.RawURLEncoding.EncodeToString(sig)
	return msg + "." + encoded
}

// VerifyAndExtract splits "message.signature" and verifies the signature.
// Returns the original message if valid, or an error if not.
func verifyAndExtract(signed string, key []byte) (msg string, isOk bool) {
	parts := strings.LastIndexByte(signed, '.')
	if parts < 0 {
		return "", false
	}

	msg = signed[:parts]
	sigB64 := signed[parts+1:]

	// recompute expected signature
	mac := hmac.New(sha256.New, key)
	_, err := mac.Write([]byte(msg))
	if err != nil {
		panic(fmt.Sprintf("hmac write failed: %s", err))
	}
	expected := mac.Sum(nil)
	expectedB64 := base64.RawURLEncoding.EncodeToString(expected)

	if subtle.ConstantTimeCompare([]byte(expectedB64), []byte(sigB64)) != 1 {
		return "", false
	}

	return msg, true
}