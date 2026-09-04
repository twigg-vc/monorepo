package secrets

import (
	"encoding/base64"
	"fmt"
)

func parseMasterKey(raw string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}

	if len(key) != 32 { // AES-256
		return nil, fmt.Errorf("%q must decode to 32 bytes, got %d", raw, len(key))
	}

	return key, nil
}
