package application

import (
	"context"
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"errors"
)

// UpdateUser actualiza el perfil y los datos opcionales de negocio/depósito de un contratista.
type UpdateUser struct {
	repository domain.Repository
	banks      domain.BankRegistrar
	clients    domain.ClientStore
}

// NewUpdateUser prepara el caso de uso de edición de perfil.
func NewUpdateUser(repository domain.Repository, banks domain.BankRegistrar, clients domain.ClientStore) *UpdateUser {
	return &UpdateUser{repository: repository, banks: banks, clients: clients}
}

// Update valida, guarda los cambios y registra o actualiza la CLABE en Pey cuando corresponda.
func (act *UpdateUser) Update(
	ctx context.Context,
	id string,
	patch domain.ProfilePatch,
) (domain.Onboarding, map[string]string, error) {
	existing, found := act.repository.Find(id)
	if !found {
		return domain.Onboarding{}, nil, domain.ErrNotFound
	}
	updated, fieldErrors := domain.ApplyProfilePatch(existing, patch)
	if len(fieldErrors) > 0 {
		return domain.Onboarding{}, fieldErrors, nil
	}
	clabeChanged := updated.CLABE != existing.CLABE
	if clabeChanged {
		if updated.CLABE != "" {
			taken, err := act.repository.HasOtherCLABE(updated.CLABE, existing.ID)
			if err != nil {
				return domain.Onboarding{}, nil, err
			}
			if fieldErrors = domain.TakenCLABE(updated.CLABE, taken); len(fieldErrors) > 0 {
				return domain.Onboarding{}, fieldErrors, nil
			}
		}
		updated.BankAccountID = ""
		updated.CodiStatus = domain.NotRegisteredStatus()
	}

	saved, err := act.repository.Update(updated)
	if errors.Is(err, domain.ErrCLABETaken) {
		return domain.Onboarding{}, domain.TakenCLABE(updated.CLABE, true), nil
	}
	if err != nil {
		return domain.Onboarding{}, nil, err
	}
	if act.clients != nil {
		if customer, ok, clientErr := act.clients.First(saved.ID); clientErr == nil && ok {
			saved.Customer = customer
		}
	}
	if saved.CLABE != "" && (clabeChanged || saved.BankAccountID == "") {
		return registerCLABEInPey(ctx, act.repository, act.banks, saved), nil, nil
	}
	return domain.View(saved), nil, nil
}
