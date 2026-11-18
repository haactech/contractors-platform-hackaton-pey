package application

import (
	"context"
	"contractors-platform/backend/internal/modules/onboarding/definitions"
	"contractors-platform/backend/internal/modules/onboarding/domain"
)

// RetryBankRegistration re-sends the stored CLABE to Pey when the vendor has no
// bankAccountId yet (soft-fail recovery) without requiring a CLABE change.
type RetryBankRegistration struct {
	repository domain.Repository
	banks      domain.BankRegistrar
	clients    domain.ClientStore
}

// NewRetryBankRegistration prepares the retry use case.
func NewRetryBankRegistration(
	repository domain.Repository,
	banks domain.BankRegistrar,
	clients domain.ClientStore,
) *RetryBankRegistration {
	return &RetryBankRegistration{repository: repository, banks: banks, clients: clients}
}

// Retry registers the CLABE again when needed and returns the public onboarding view.
func (act *RetryBankRegistration) Retry(ctx context.Context, id string) (domain.Onboarding, map[string]string, error) {
	user, found := act.repository.Find(id)
	if !found {
		return domain.Onboarding{}, nil, domain.ErrNotFound
	}
	if act.clients != nil {
		if customer, ok, err := act.clients.First(id); err == nil && ok {
			user.Customer = customer
		}
	}
	if user.CLABE == "" {
		return domain.Onboarding{}, map[string]string{definitions.FieldCLABE: definitions.MessageCLABE}, nil
	}
	if domain.BankReady(user) {
		return domain.View(user), nil, nil
	}
	return registerCLABEInPey(ctx, act.repository, act.banks, user), nil, nil
}
