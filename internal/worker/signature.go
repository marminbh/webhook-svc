package worker

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// GenerateHMACSignature generates an HMAC SHA256 signature for the payload
// Returns the signature in the format: sha256=<hex_encoded_hmac>
func GenerateHMACSignature(payload []byte, secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("secret cannot be empty")
	}

	// Create HMAC with SHA256
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write(payload)
	if err != nil {
		return "", fmt.Errorf("failed to write payload to HMAC: %w", err)
	}

	// Get the HMAC sum and encode as hex
	signature := hex.EncodeToString(mac.Sum(nil))

	// Return in the format: sha256=<hex_encoded_hmac>
	return fmt.Sprintf("sha256=%s", signature), nil
}
