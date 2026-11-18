package application

import (
	"contractors-platform/backend/internal/modules/onboarding/definitions"
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"errors"
	"fmt"
)

// SeedUser creates a contractor record idempotently for system seeding (such as the demo account)
// without calling Pey or binding through Owners. It attaches a synthetic ACTIVE bank account so
// Stage 2 payment create can be exercised offline.
type SeedUser struct {
	repository domain.Repository
	clients    domain.ClientStore
}

// NewSeedUser prepares a SeedUser use case.
func NewSeedUser(repository domain.Repository, clients domain.ClientStore) *SeedUser {
	return &SeedUser{repository: repository, clients: clients}
}

// Ensure stores input if its CLABE is not already present and returns the contractor ID.
func (s *SeedUser) Ensure(input domain.Input) (string, error) {
	user, fieldErrors := domain.Build(input)
	if len(fieldErrors) > 0 {
		return "", fmt.Errorf("invalid seed onboarding input: %v", fieldErrors)
	}
	if existing, found := s.repository.FindByCLABE(user.CLABE); found {
		if err := s.ensureFirstClient(existing.ID, user.Customer); err != nil {
			return "", err
		}
		if _, err := s.ensureDemoBank(existing); err != nil {
			return "", err
		}
		return existing.ID, nil
	}
	saved, err := s.repository.Save(user)
	if errors.Is(err, domain.ErrCLABETaken) {
		if existing, found := s.repository.FindByCLABE(user.CLABE); found {
			if clientErr := s.ensureFirstClient(existing.ID, user.Customer); clientErr != nil {
				return "", clientErr
			}
			if _, bankErr := s.ensureDemoBank(existing); bankErr != nil {
				return "", bankErr
			}
			return existing.ID, nil
		}
	}
	if err != nil {
		return "", err
	}
	if err := s.ensureFirstClient(saved.ID, saved.Customer); err != nil {
		_ = s.repository.Remove(saved.ID)
		return "", err
	}
	if _, err := s.ensureDemoBank(saved); err != nil {
		_ = s.clientsRemove(saved.ID)
		_ = s.repository.Remove(saved.ID)
		return "", err
	}
	return saved.ID, nil
}

func (s *SeedUser) ensureFirstClient(vendorID string, customer domain.Customer) error {
	if s.clients == nil {
		return nil
	}
	_, found, err := s.clients.First(vendorID)
	if err != nil {
		return err
	}
	if found {
		return nil
	}
	return s.clients.SaveFirst(vendorID, customer)
}

func (s *SeedUser) ensureDemoBank(user domain.User) (domain.User, error) {
	if domain.BankReady(user) {
		return user, nil
	}
	updated, ok := domain.AttachBankAccount(user, domain.BankAccount{
		ID:     definitions.DemoBankAccountID,
		Status: definitions.CodiActive,
	})
	if !ok {
		return user, fmt.Errorf("invalid demo bank attachment")
	}
	recorded, err := s.repository.RecordBankAccount(updated)
	if err != nil {
		return user, err
	}
	return recorded, nil
}

func (s *SeedUser) clientsRemove(vendorID string) error {
	if s.clients == nil {
		return nil
	}
	return s.clients.RemoveForVendor(vendorID)
}
