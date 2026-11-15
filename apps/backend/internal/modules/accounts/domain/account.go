package domain

import (
	"context"
	"errors"
	"time"
)

// Account represents a user login account bound 1:1 to a contractor onboarding record.
type Account struct {
	ID              string
	Email           string
	Phone           string
	PINHash         string
	ContractorID    *string
	EmailVerifiedAt *time.Time
	FailedLogins    int
	LockedUntil     *time.Time
	IsDemo          bool
}

// EmailVerified reports whether the account's email address has been verified.
func (a Account) EmailVerified() bool {
	return a.EmailVerifiedAt != nil
}

// Session represents an authenticated session persisted as a SHA-256 token hash.
type Session struct {
	TokenHash string
	AccountID string
	ExpiresAt time.Time
}

// Code represents a single-use verification or PIN-recovery code.
type Code struct {
	ID         string
	AccountID  string
	Purpose    string
	CodeHash   string
	Salt       string
	Attempts   int
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

var (
	ErrEmailTaken        = errors.New("correo tomado")
	ErrPhoneTaken        = errors.New("teléfono tomado")
	ErrAlreadyLinked     = errors.New("cuenta ya tiene contractor")
	ErrInvalidCredential = errors.New("credenciales inválidas")
	ErrInvalidCode       = errors.New("código inválido o vencido")
	ErrDemoProtected     = errors.New("cuenta demo protegida")
)

// ErrLocked indicates the account is temporarily locked after repeated wrong PIN attempts.
type ErrLocked struct {
	Until time.Time
}

func (e ErrLocked) Error() string { return "cuenta bloqueada temporalmente" }

// ErrResendTooSoon indicates the cooldown between code dispatches has not elapsed.
type ErrResendTooSoon struct {
	RetryAfter time.Duration
}

func (e ErrResendTooSoon) Error() string { return "espera antes de reenviar otro código" }

// Repository persists accounts, sessions, and verification/recovery codes.
type Repository interface {
	Create(Account) (Account, error)
	Find(id string) (Account, bool, error)
	FindByEmail(email string) (Account, bool, error)
	FindByPhone(phone string) (Account, bool, error)
	LinkContractor(accountID, contractorID string) error
	UpdateSecurity(Account) error

	SaveSession(session Session, now time.Time) error
	FindSession(tokenHash string, now time.Time) (Session, bool, error)
	DeleteSession(tokenHash string) error
	DeleteSessionsFor(accountID string) error

	SaveCode(Code) error
	LatestCode(accountID, purpose string) (Code, bool, error)
	UpdateCode(Code) error
}

// PINHasher hashes and verifies 6-digit PINs.
type PINHasher interface {
	Hash(pin string) (string, error)
	Verify(pin, encoded string) bool
}

// Mailer delivers verification and PIN recovery emails.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Clock returns the current instant, injectable for deterministic tests.
type Clock func() time.Time

// CopyText clones an optional string pointer.
func CopyText(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

// CopyTime clones an optional time.Time pointer.
func CopyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

// CloneAccount returns a deep copy of Account so callers cannot mutate stored pointers.
func CloneAccount(a Account) Account {
	a.ContractorID = CopyText(a.ContractorID)
	a.EmailVerifiedAt = CopyTime(a.EmailVerifiedAt)
	a.LockedUntil = CopyTime(a.LockedUntil)
	return a
}

// CloneCode returns a deep copy of Code.
func CloneCode(c Code) Code {
	c.ConsumedAt = CopyTime(c.ConsumedAt)
	return c
}
