package secrets

import (
	"bytes"
	"encoding/base64"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestReadMasterKeyFromEnv(t *testing.T) {
	envVariableName := "TWIGG_MASTER_KEY"

	b := bytes.Repeat([]byte{'A'}, 32) // 32 bytes of 0x41
	enc := base64.StdEncoding.EncodeToString(b)
	t.Setenv(envVariableName, enc)

	key, err := ParseMasterKey(os.Getenv(envVariableName))
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
}
func TestCreateMasterWithOpensslThenReadMasterKeyFromEnv(t *testing.T) {
	// Check if openssl exists
	if _, err := exec.LookPath("openssl"); err != nil {
		// Cancel the test if openssl is not installed
		return
	}

	out, err := exec.Command("openssl", "rand", "-base64", "32").Output()
	if err != nil {
		t.Fatalf("failed to run openssl: %v", err)
	}

	// Trim whitespace/newlines
	base64Key := strings.TrimSpace(string(out))

	envVariableName := "TWIGG_MASTER_KEY"
	t.Setenv(envVariableName, base64Key)

	key, err := ParseMasterKey(os.Getenv(envVariableName))
	if err != nil {
		t.Fatal(err)
	}

	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
}
