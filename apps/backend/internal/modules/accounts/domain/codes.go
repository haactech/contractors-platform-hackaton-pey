package domain

import (
	"contractors-platform/backend/internal/modules/accounts/definitions"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// ValidateCodeFormat checks that code is 6 ASCII digits.
func ValidateCodeFormat(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) != definitions.CodeLength {
		return false
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] < '0' || trimmed[i] > '9' {
			return false
		}
	}
	return true
}

// NewVerificationCode generates a cryptographically random 6-digit code, a random salt,
// and its salted SHA-256 hex digest.
func NewVerificationCode() (plain string, salt string, hash string, err error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", "", "", err
	}
	plain = fmt.Sprintf("%06d", n.Int64())

	var saltBytes [16]byte
	if _, err := rand.Read(saltBytes[:]); err != nil {
		return "", "", "", err
	}
	salt = hex.EncodeToString(saltBytes[:])
	hash = HashCode(salt, plain)
	return plain, salt, hash, nil
}

// HashCode computes SHA-256(salt + ":" + code) as hex.
func HashCode(salt, code string) string {
	sum := sha256.Sum256([]byte(salt + ":" + strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}

// VerifyCode compares candidate against code in constant time and checks expiry/attempts.
func VerifyCode(code Code, candidate string, now time.Time) bool {
	if code.ConsumedAt != nil || !now.Before(code.ExpiresAt) || code.Attempts >= definitions.MaxCodeAttempts {
		return false
	}
	candidateHash := HashCode(code.Salt, candidate)
	return subtle.ConstantTimeCompare([]byte(code.CodeHash), []byte(candidateHash)) == 1
}

// HashToken computes the SHA-256 hex digest of a raw session token.
func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}
