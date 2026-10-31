package validation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// ErrInvalidSignature is returned when the HMAC signature does not match the payload.
var ErrInvalidSignature = errors.New("invalid HMAC signature")

// ErrMalformedSignature is returned when the provided signature cannot be parsed.
var ErrMalformedSignature = errors.New("malformed HMAC signature")

// ComputeHMAC returns the hex‑encoded HMAC‑SHA256 of the given payload using the supplied secret.
// This helper is useful for generating signatures (e.g., in tests or when sending webhooks).
func ComputeHMAC(payload []byte, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload) // Write never returns an error for hash.Hash
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature validates that the supplied signature matches the HMAC‑SHA256 of the payload.
// The signature may be provided in either of the following forms:
//
//   1. "sha256=hexvalue"  – the common format used by many webhook providers.
//   2. "hexvalue"          – raw hex string without the algorithm prefix.
//
// The comparison is performed in constant time using hmac.Equal to mitigate timing attacks.
// It returns nil on success or a descriptive error on failure.
func VerifySignature(payload []byte, signature string, secret []byte) error {
	// Normalise the incoming signature string.
	sig := strings.TrimSpace(signature)

	// If the signature contains an algorithm prefix (e.g., "sha256=..."),
	// split it and ensure the algorithm is SHA256.
	if strings.Contains(sig, "=") {
		parts := strings.SplitN(sig, "=", 2)
		if len(parts) != 2 {
			return ErrMalformedSignature
		}
		algo := strings.TrimSpace(strings.ToLower(parts[0]))
		if algo != "sha256" {
			return ErrMalformedSignature
		}
		sig = parts[1]
	}

	// Decode the hex‑encoded signature.
	sigBytes, err := hex.DecodeString(sig)
	if err != nil {
		return ErrMalformedSignature
	}

	// Compute the expected HMAC.
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)

	// Constant‑time comparison.
	if !hmac.Equal(sigBytes, expected) {
		return ErrInvalidSignature
	}
	return nil
}