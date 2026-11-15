package application

import (
	"context"
	"contractors-platform/backend/internal/modules/accounts/definitions"
	"contractors-platform/backend/internal/modules/accounts/domain"
	"errors"
)

var ErrDemoDisabled = errors.New("modo demo deshabilitado")

// Demo manages the idempotent demo account seed and passwordless demo login.
type Demo struct {
	repo     domain.Repository
	hasher   domain.PINHasher
	sessions *Sessions
	policy   domain.Policy
	clock    domain.Clock
}

// NewDemo builds a Demo service.
func NewDemo(repo domain.Repository, hasher domain.PINHasher, sessions *Sessions, policy domain.Policy, clock domain.Clock) *Demo {
	return &Demo{repo: repo, hasher: hasher, sessions: sessions, policy: policy, clock: clock}
}

// EnsureDemoAccount idempotently seeds the demo account and links it to the contractor returned by seedContractor.
func (d *Demo) EnsureDemoAccount(_ context.Context, seedContractor func() (string, error)) error {
	if !d.policy.DemoAccount {
		return nil
	}
	existing, found, err := d.repo.FindByEmail(definitions.DemoEmail)
	if err != nil {
		return err
	}
	if found && existing.ContractorID != nil {
		return nil
	}
	contractorID, err := seedContractor()
	if err != nil {
		return err
	}
	if found {
		return d.repo.LinkContractor(existing.ID, contractorID)
	}
	pinHash, err := d.hasher.Hash(definitions.DemoPIN)
	if err != nil {
		return err
	}
	now := d.clock().UTC()
	_, err = d.repo.Create(domain.Account{
		Email:           definitions.DemoEmail,
		Phone:           definitions.DemoPhone,
		PINHash:         pinHash,
		ContractorID:    &contractorID,
		EmailVerifiedAt: &now,
		IsDemo:          true,
	})
	return err
}

// LogInDemo creates a session for the seeded demo account when demo mode is active.
func (d *Demo) LogInDemo(_ context.Context) (domain.Account, string, error) {
	if !d.policy.DemoAccount {
		return domain.Account{}, "", ErrDemoDisabled
	}
	account, found, err := d.repo.FindByEmail(definitions.DemoEmail)
	if err != nil {
		return domain.Account{}, "", err
	}
	if !found {
		return domain.Account{}, "", ErrDemoDisabled
	}
	token, err := d.sessions.issue(account.ID)
	if err != nil {
		return domain.Account{}, "", err
	}
	return account, token, nil
}
