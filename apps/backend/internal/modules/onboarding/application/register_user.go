package application

import (
	"context"
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"errors"
	"log"
)

// RegisterUser guarda el contrato del vendedor, su primer cliente y registra su CLABE en Pey.
type RegisterUser struct {
	repository domain.Repository
	banks      domain.BankRegistrar
	clients    domain.ClientStore
}

// NewRegisterUser prepara el acto de alta con el repositorio, el registro en Pey y los clientes.
func NewRegisterUser(repository domain.Repository, banks domain.BankRegistrar, clients domain.ClientStore) *RegisterUser {
	return &RegisterUser{repository: repository, banks: banks, clients: clients}
}

// Create arma el vendedor, lo guarda y pide a Pey el bank id de su CLABE.
func (act *RegisterUser) Create(ctx context.Context, input domain.Input) (domain.Onboarding, map[string]string, error) {
	user, fieldErrors := domain.Build(input)
	if len(fieldErrors) > 0 {
		return domain.Onboarding{}, fieldErrors, nil
	}
	taken, err := act.repository.HasCLABE(user.CLABE)
	if err != nil {
		return domain.Onboarding{}, nil, err
	}
	if fieldErrors = domain.TakenCLABE(user.CLABE, taken); len(fieldErrors) > 0 {
		return domain.Onboarding{}, fieldErrors, nil
	}
	saved, err := act.repository.Save(user)
	if errors.Is(err, domain.ErrCLABETaken) {
		return domain.Onboarding{}, domain.TakenCLABE(user.CLABE, true), nil
	}
	if err != nil {
		return domain.Onboarding{}, nil, err
	}
	if err := act.clients.SaveFirst(saved.ID, saved.Customer); err != nil {
		if removeErr := act.repository.Remove(saved.ID); removeErr != nil {
			log.Printf("onboarding: remove vendor %s after client failure: %v", saved.ID, removeErr)
		}
		log.Printf("onboarding: first client %s: %v", saved.ID, err)
		return domain.Onboarding{}, nil, err
	}
	return act.registerVendorBank(ctx, saved), nil, nil
}

// registerVendorBank envía la CLABE y la identidad del vendedor ya guardado a Pey.
// El id y el estado que Pey devuelve se anotan solo en el contrato de ese vendedor.
func (act *RegisterUser) registerVendorBank(ctx context.Context, saved domain.User) domain.Onboarding {
	payload, ok := domain.EncodeRegistration(saved)
	if !ok {
		return domain.View(saved)
	}
	response, err := act.banks.Register(ctx, payload)
	if err != nil {
		log.Printf("onboarding: pey register %s: %v", saved.ID, err)
		return domain.View(saved)
	}
	account, ok := domain.DecodeBankAccount(response)
	if !ok {
		log.Printf("onboarding: pey register %s: respuesta sin id o estado válido", saved.ID)
		return domain.View(saved)
	}
	updated, ok := domain.AttachBankAccount(saved, account)
	if !ok {
		return domain.View(saved)
	}
	recorded, err := act.repository.RecordBankAccount(updated)
	if err != nil {
		log.Printf("onboarding: record bank account %s %s: %v", saved.ID, account.ID, err)
		return domain.View(saved)
	}
	recorded.Customer = saved.Customer
	return domain.View(recorded)
}
