package infrastructure

import (
	"context"
	"contractors-platform/backend/internal/modules/dashboard/domain"
	"contractors-platform/backend/internal/modules/onboarding"
)

// OnboardingSource reads contractors through the public API of the onboarding
// module, without touching its repository or internal packages.
type OnboardingSource struct{ records onboarding.Reader }

func NewOnboardingSource(records onboarding.Reader) *OnboardingSource {
	return &OnboardingSource{records: records}
}

func (source *OnboardingSource) FindContractor(_ context.Context, id string) (domain.Contractor, error) {
	record, found := source.records.Find(id)
	if !found {
		return domain.Contractor{}, domain.ErrContractorNotFound
	}
	return domain.Contractor{
		ID:               record.ID,
		Name:             record.Name,
		Profession:       record.Profession,
		BusinessName:     record.BusinessName,
		BusinessActivity: record.BusinessActivity,
		RFC:              record.RFC,
		TaxRegime:        record.TaxRegime,
		ClabeLast4:       record.ClabeLast4,
		CodiStatus:       record.CodiStatus,
		BankReady:        record.BankReady,
		FirstCustomer:    domain.FirstCustomer{Name: record.Client.Name, Phone: record.Client.Phone, Email: record.Client.Email},
	}, nil
}
