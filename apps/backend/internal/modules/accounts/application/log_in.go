package application

import (
	"contractors-platform/backend/internal/modules/accounts/definitions"
	"contractors-platform/backend/internal/modules/accounts/domain"
	"strings"
)

// LogIn authenticates an account by email or phone plus 6-digit PIN.
type LogIn struct {
	repo      domain.Repository
	hasher    domain.PINHasher
	sessions  *Sessions
	policy    domain.Policy
	clock     domain.Clock
	dummyHash string
}

// NewLogIn creates a LogIn use case with a precomputed dummy hash for constant-time misses.
func NewLogIn(repo domain.Repository, hasher domain.PINHasher, sessions *Sessions, policy domain.Policy, clock domain.Clock) *LogIn {
	dummyHash, _ := hasher.Hash("000000")
	return &LogIn{
		repo:      repo,
		hasher:    hasher,
		sessions:  sessions,
		policy:    policy,
		clock:     clock,
		dummyHash: dummyHash,
	}
}

// Open verifies credentials, enforces lockout policy when active, and returns Account + session token.
func (l *LogIn) Open(input domain.LogInInput) (domain.Account, string, map[string]string, error) {
	fieldErrors := map[string]string{}
	if strings.TrimSpace(input.Identifier) == "" {
		fieldErrors[definitions.FieldIdentifier] = definitions.MessageRequired
	}
	if input.PIN == "" {
		fieldErrors[definitions.FieldPIN] = definitions.MessageRequired
	}
	if len(fieldErrors) > 0 {
		return domain.Account{}, "", fieldErrors, nil
	}

	kind, normalized, ok := domain.ParseIdentifier(input.Identifier)
	if !ok || !domain.ValidatePIN(input.PIN) {
		l.hasher.Verify(input.PIN, l.dummyHash)
		return domain.Account{}, "", nil, domain.ErrInvalidCredential
	}

	var (
		account domain.Account
		found   bool
		err     error
	)
	switch kind {
	case domain.IdentifierEmail:
		account, found, err = l.repo.FindByEmail(normalized)
	case domain.IdentifierPhone:
		account, found, err = l.repo.FindByPhone(normalized)
	}
	if err != nil {
		return domain.Account{}, "", nil, err
	}
	if !found {
		l.hasher.Verify(input.PIN, l.dummyHash)
		return domain.Account{}, "", nil, domain.ErrInvalidCredential
	}

	now := l.clock().UTC()
	if domain.IsLocked(account, now, l.policy) {
		return domain.Account{}, "", nil, domain.ErrLocked{Until: *account.LockedUntil}
	}

	if !l.hasher.Verify(input.PIN, account.PINHash) {
		updated, locked := domain.RegisterFailure(account, now, l.policy)
		if l.policy.Lockout && !account.IsDemo {
			if err := l.repo.UpdateSecurity(updated); err != nil {
				return domain.Account{}, "", nil, err
			}
		}
		if locked {
			return domain.Account{}, "", nil, domain.ErrLocked{Until: *updated.LockedUntil}
		}
		return domain.Account{}, "", nil, domain.ErrInvalidCredential
	}

	if account.FailedLogins > 0 || account.LockedUntil != nil {
		account = domain.ClearFailures(account)
		if err := l.repo.UpdateSecurity(account); err != nil {
			return domain.Account{}, "", nil, err
		}
	}

	token, err := l.sessions.issue(account.ID)
	if err != nil {
		return domain.Account{}, "", nil, err
	}
	return account, token, nil, nil
}
