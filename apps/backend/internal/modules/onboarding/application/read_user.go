package application

import (
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"log"
)

// ReadUser recupera un usuario ya guardado y adjunta su primer cliente.
type ReadUser struct {
	repository domain.Repository
	clients    domain.ClientStore
}

// NewReadUser prepara el acto de lectura con el repositorio y los clientes.
func NewReadUser(repository domain.Repository, clients domain.ClientStore) *ReadUser {
	return &ReadUser{repository: repository, clients: clients}
}

// Find devuelve el usuario en la forma que el formulario ya entiende.
func (act *ReadUser) Find(id string) (domain.Onboarding, bool) {
	user, found := act.repository.Find(id)
	if !found {
		return domain.Onboarding{}, false
	}
	customer, ok, err := act.clients.First(id)
	if err != nil {
		log.Printf("onboarding: first client %s: %v", id, err)
		return domain.Onboarding{}, false
	}
	if ok {
		user.Customer = customer
	}
	return domain.View(user), true
}
