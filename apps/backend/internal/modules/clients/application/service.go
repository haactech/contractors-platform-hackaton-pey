package application

import (
	"contractors-platform/backend/internal/modules/clients/domain"
	"errors"
)

// Service guarda y lee los clientes de un vendedor.
type Service struct {
	repository domain.Repository
	vendors    domain.Vendors
}

// NewService prepara el acto con el almacén y la lectura del vendedor.
func NewService(repository domain.Repository, vendors domain.Vendors) *Service {
	return &Service{repository: repository, vendors: vendors}
}

// SaveFirst guarda el primer cliente que llega con el alta.
func (service *Service) SaveFirst(vendorID, name, phone, email string) error {
	client, fieldErrors := domain.BuildFirst(vendorID, name, phone, email)
	if len(fieldErrors) > 0 {
		return errors.New("el primer cliente no cumple el contrato")
	}
	if _, err := service.repository.Save(client); err != nil {
		return err
	}
	return nil
}

// Create guarda el perfil que captura la pantalla.
func (service *Service) Create(vendorID string, input domain.Input) (domain.Client, map[string]string, error) {
	if err := service.knownVendor(vendorID); err != nil {
		return domain.Client{}, nil, err
	}
	client, fieldErrors := domain.BuildDashboard(vendorID, input)
	if len(fieldErrors) > 0 {
		return domain.Client{}, fieldErrors, nil
	}
	saved, err := service.repository.Save(client)
	if errors.Is(err, domain.ErrPhoneTaken) {
		return domain.Client{}, map[string]string{domain.PhoneField(): domain.PhoneTakenMessage()}, nil
	}
	if err != nil {
		return domain.Client{}, nil, err
	}
	return saved, nil, nil
}

// Update reemplaza el perfil editable de un cliente de ese vendedor.
func (service *Service) Update(vendorID, clientID string, input domain.Input) (domain.Client, map[string]string, error) {
	if err := service.knownVendor(vendorID); err != nil {
		return domain.Client{}, nil, err
	}
	current, found, err := service.repository.Find(vendorID, clientID)
	if err != nil {
		return domain.Client{}, nil, err
	}
	if !found {
		return domain.Client{}, nil, domain.ErrClientNotFound
	}
	client, fieldErrors := domain.BuildDashboard(vendorID, input)
	if len(fieldErrors) > 0 {
		return domain.Client{}, fieldErrors, nil
	}
	client.ID = current.ID
	client.CreatedAt = current.CreatedAt
	saved, err := service.repository.Update(client)
	if errors.Is(err, domain.ErrPhoneTaken) {
		return domain.Client{}, map[string]string{domain.PhoneField(): domain.PhoneTakenMessage()}, nil
	}
	if errors.Is(err, domain.ErrClientNotFound) {
		return domain.Client{}, nil, err
	}
	if err != nil {
		return domain.Client{}, nil, err
	}
	return saved, nil, nil
}

// List devuelve los clientes del vendedor, del más antiguo al más nuevo.
func (service *Service) List(vendorID string) ([]domain.Client, error) {
	if err := service.knownVendor(vendorID); err != nil {
		return nil, err
	}
	return service.repository.List(vendorID)
}

// Find devuelve un cliente si pertenece a ese vendedor.
func (service *Service) Find(vendorID, clientID string) (domain.Client, bool, error) {
	if err := service.knownVendor(vendorID); err != nil {
		return domain.Client{}, false, err
	}
	return service.repository.Find(vendorID, clientID)
}

// DeleteByVendor removes every client of that vendor (onboarding claim rollback).
func (service *Service) DeleteByVendor(vendorID string) error {
	return service.repository.DeleteByVendor(vendorID)
}

// First devuelve el cliente más antiguo del vendedor.
func (service *Service) First(vendorID string) (domain.Client, bool, error) {
	clients, err := service.repository.List(vendorID)
	if err != nil || len(clients) == 0 {
		return domain.Client{}, false, err
	}
	return clients[0], true, nil
}

func (service *Service) knownVendor(vendorID string) error {
	exists, err := service.vendors.Exists(vendorID)
	if err != nil {
		return err
	}
	if !exists {
		return domain.ErrVendorNotFound
	}
	return nil
}
