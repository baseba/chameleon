package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// Generate creates a SHA256 hash from HTTP method, URL path, and request body
// Returns a hex-encoded string suitable for use as a filename
func Generate(method, path string, body io.Reader) (string, error) {
	h := sha256.New()

	// Include method and path in the hash
	if _, err := fmt.Fprintf(h, "%s:%s:", method, path); err != nil {
		return "", fmt.Errorf("failed to write method and path to hash: %w", err)
	}

	// Include request body in the hash if present
	if body != nil {
		if _, err := io.Copy(h, body); err != nil {
			return "", fmt.Errorf("failed to read request body for hashing: %w", err)
		}
	}

	// Return hex-encoded hash
	return hex.EncodeToString(h.Sum(nil)), nil
}

