package application

import (
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"errors"
	"fmt"
)

// SeedUser creates a contractor record idempotently for system seeding (such as the demo account)
// without calling Pey or binding through Owners.
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
		return existing.ID, nil
	}
	saved, err := s.repository.Save(user)
	if errors.Is(err, domain.ErrCLABETaken) {
		if existing, found := s.repository.FindByCLABE(user.CLABE); found {
			if clientErr := s.ensureFirstClient(existing.ID, user.Customer); clientErr != nil {
				return "", clientErr
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
