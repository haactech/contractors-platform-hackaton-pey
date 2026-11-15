package infrastructure

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const (
	pbkdf2Algorithm = "pbkdf2-sha256"
	pbkdf2SaltBytes = 16
	pbkdf2KeyBytes  = 32
)

// PBKDF2Hasher hashes 6-digit PINs using standard-library PBKDF2-HMAC-SHA256.
type PBKDF2Hasher struct {
	Iterations int
}

// NewPBKDF2Hasher constructs a PBKDF2Hasher with the specified iteration count.
func NewPBKDF2Hasher(iterations int) PBKDF2Hasher {
	if iterations <= 0 {
		iterations = 100_000
	}
	return PBKDF2Hasher{Iterations: iterations}
}

// Hash derives a salted PBKDF2-SHA256 hash encoded as pbkdf2-sha256$iter$salt$hash.
func (h PBKDF2Hasher) Hash(pin string) (string, error) {
	salt := make([]byte, pbkdf2SaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, pin, salt, h.Iterations, pbkdf2KeyBytes)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"%s$%d$%s$%s",
		pbkdf2Algorithm,
		h.Iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify checks pin against an encoded PBKDF2 hash in constant time.
func (h PBKDF2Hasher) Verify(pin, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != pbkdf2Algorithm {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) == 0 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(expected) == 0 {
		return false
	}
	actual, err := pbkdf2.Key(sha256.New, pin, salt, iterations, len(expected))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
