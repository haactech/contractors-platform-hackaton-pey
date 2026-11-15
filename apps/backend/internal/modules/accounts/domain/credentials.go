package domain

import (
	"contractors-platform/backend/internal/modules/accounts/definitions"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

// SignUpInput represents the JSON payload for POST /api/auth/signup.
type SignUpInput struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
	PIN   string `json:"pin"`
}

// LogInInput represents the JSON payload for POST /api/auth/login.
type LogInInput struct {
	Identifier string `json:"identifier"`
	PIN        string `json:"pin"`
}

// IdentifierKind identifies whether a login/recovery identifier is an email or phone.
type IdentifierKind string

const (
	IdentifierEmail IdentifierKind = "email"
	IdentifierPhone IdentifierKind = "phone"
)

// NormalizeEmail trims whitespace, lowercases, and validates an email address.
func NormalizeEmail(raw string) (string, bool) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" || utf8.RuneCountInString(trimmed) > definitions.MaxRunes {
		return "", false
	}
	address, err := mail.ParseAddress(trimmed)
	if err != nil || address.Address != trimmed {
		return "", false
	}
	return trimmed, true
}

// NormalizePhone extracts Mexican 10-digit mobile numbers (allowing +52 / 52 prefixes,
// spaces, parentheses, and hyphens) and formats them canonically as "+52XXXXXXXXXX".
func NormalizePhone(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	for _, r := range trimmed {
		if (r < '0' || r > '9') && r != '+' && r != ' ' && r != '(' && r != ')' && r != '-' {
			return "", false
		}
	}
	var digits strings.Builder
	for _, r := range trimmed {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	if len(d) == 12 && strings.HasPrefix(d, "52") {
		d = d[2:]
	}
	if len(d) != definitions.MXPhoneDigits {
		return "", false
	}
	return "+52" + d, true
}

// ValidatePIN checks that pin is exactly 6 ASCII digits.
func ValidatePIN(raw string) bool {
	if len(raw) != definitions.PINLength {
		return false
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			return false
		}
	}
	return true
}

// ParseIdentifier classifies and normalizes a login/recovery identifier as email or phone.
func ParseIdentifier(raw string) (IdentifierKind, string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", false
	}
	if strings.Contains(trimmed, "@") {
		email, ok := NormalizeEmail(trimmed)
		return IdentifierEmail, email, ok
	}
	phone, ok := NormalizePhone(trimmed)
	return IdentifierPhone, phone, ok
}

// ValidateSignUp normalizes and validates sign-up fields, returning an Account skeleton
// (without PINHash or ID) and a map of field validation errors.
func ValidateSignUp(input SignUpInput) (Account, map[string]string) {
	errors := map[string]string{}

	rawEmail := strings.TrimSpace(input.Email)
	var email string
	if rawEmail == "" {
		errors[definitions.FieldEmail] = definitions.MessageRequired
	} else if utf8.RuneCountInString(rawEmail) > definitions.MaxRunes {
		errors[definitions.FieldEmail] = definitions.MessageMaxRunes
	} else if normalized, ok := NormalizeEmail(rawEmail); !ok {
		errors[definitions.FieldEmail] = definitions.MessageEmail
	} else {
		email = normalized
	}

	rawPhone := strings.TrimSpace(input.Phone)
	var phone string
	if rawPhone == "" {
		errors[definitions.FieldPhone] = definitions.MessageRequired
	} else if normalized, ok := NormalizePhone(rawPhone); !ok {
		errors[definitions.FieldPhone] = definitions.MessagePhone
	} else {
		phone = normalized
	}

	if input.PIN == "" {
		errors[definitions.FieldPIN] = definitions.MessageRequired
	} else if !ValidatePIN(input.PIN) {
		errors[definitions.FieldPIN] = definitions.MessagePIN
	}

	if len(errors) > 0 {
		return Account{}, errors
	}
	return Account{Email: email, Phone: phone}, nil
}

// IsLocked reports whether the account is currently locked at now under policy.
func IsLocked(account Account, now time.Time, policy Policy) bool {
	if !policy.Lockout || account.IsDemo || account.LockedUntil == nil {
		return false
	}
	return now.Before(*account.LockedUntil)
}

// RegisterFailure records a failed login attempt and locks the account when threshold is reached.
func RegisterFailure(account Account, now time.Time, policy Policy) (Account, bool) {
	updated := CloneAccount(account)
	if !policy.Lockout || updated.IsDemo {
		return updated, false
	}
	// If a previous lock already expired, restart the counter from 1.
	if updated.LockedUntil != nil && !now.Before(*updated.LockedUntil) {
		updated.FailedLogins = 0
		updated.LockedUntil = nil
	}
	updated.FailedLogins++
	if updated.FailedLogins >= definitions.MaxFailedLogins {
		until := now.Add(definitions.LockoutDuration)
		updated.LockedUntil = &until
		return updated, true
	}
	return updated, false
}

// ClearFailures resets failed login counters on successful login or PIN reset.
func ClearFailures(account Account) Account {
	updated := CloneAccount(account)
	updated.FailedLogins = 0
	updated.LockedUntil = nil
	return updated
}

// MaskEmail hides most of the local part before logging an email address.
func MaskEmail(email string) string {
	at := strings.Index(email, "@")
	if at <= 1 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}
