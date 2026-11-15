package application

import (
	"context"
	"contractors-platform/backend/internal/modules/accounts/domain"
	onboardingdomain "contractors-platform/backend/internal/modules/onboarding/domain"
	"contractors-platform/backend/internal/platform/access"
	"errors"
)

// Owners implements onboarding's Owners port using the authenticated request Principal.
type Owners struct {
	repo domain.Repository
}

// NewOwners builds an Owners adapter over the accounts repository.
func NewOwners(repo domain.Repository) *Owners {
	return &Owners{repo: repo}
}

// Claimable checks that the caller is authenticated and does not yet own a contractor.
func (o *Owners) Claimable(ctx context.Context) error {
	principal, ok := access.PrincipalFrom(ctx)
	if !ok {
		return onboardingdomain.ErrAlreadyOnboarded
	}
	if principal.ContractorID != "" {
		return onboardingdomain.ErrAlreadyOnboarded
	}
	return nil
}

// Claim binds contractorID atomically 1:1 to the caller's account.
func (o *Owners) Claim(ctx context.Context, contractorID string) error {
	principal, ok := access.PrincipalFrom(ctx)
	if !ok {
		return onboardingdomain.ErrAlreadyOnboarded
	}
	err := o.repo.LinkContractor(principal.AccountID, contractorID)
	if errors.Is(err, domain.ErrAlreadyLinked) {
		return onboardingdomain.ErrAlreadyOnboarded
	}
	return err
}
