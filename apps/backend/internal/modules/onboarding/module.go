package onboarding

import (
	"contractors-platform/backend/internal/modules/onboarding/application"
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"contractors-platform/backend/internal/modules/onboarding/presentation"
	"net/http"
)

// Record es el alta que otros módulos pueden leer.
type Record = domain.Onboarding

// Reader es la lectura pública del usuario guardado.
type Reader interface {
	Find(id string) (Record, bool)
}

// Register publica las rutas y devuelve la lectura del usuario.
func Register(mux *http.ServeMux, repository domain.Repository, banks domain.BankRegistrar, clients domain.ClientStore) Reader {
	read := application.NewReadUser(repository, clients)
	handler := presentation.New(application.NewRegisterUser(repository, banks, clients), read)
	mux.HandleFunc("POST /api/onboarding", handler.Create)
	mux.HandleFunc("GET /api/onboarding/{id}", handler.Find)
	return read
}
