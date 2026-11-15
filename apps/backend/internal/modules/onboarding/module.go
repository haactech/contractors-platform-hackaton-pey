package onboarding

import (
	"contractors-platform/backend/internal/modules/onboarding/application"
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"contractors-platform/backend/internal/modules/onboarding/presentation"
	"contractors-platform/backend/internal/platform/access"
	"net/http"
)

// Record es el alta que otros módulos pueden leer.
type Record = domain.Onboarding

// Input es la entrada pública del alta para siembra controlada por la app.
type Input = domain.Input

// Client es la entrada pública del primer cliente.
type Client = domain.Client

// Reader es la lectura pública del usuario guardado.
type Reader interface {
	Find(id string) (Record, bool)
}

// Module expone las capacidades públicas del módulo onboarding.
type Module struct {
	Reader Reader
	Seed   func(Input) (string, error)
}

// Register publica las rutas protegidas y devuelve la lectura y siembra del módulo.
func Register(
	mux *http.ServeMux,
	repository domain.Repository,
	banks domain.BankRegistrar,
	owners domain.Owners,
	clients domain.ClientStore,
	guard access.Guard,
) Module {
	read := application.NewReadUser(repository, clients)
	seed := application.NewSeedUser(repository, clients)
	handler := presentation.New(
		application.NewRegisterUser(repository, banks, owners, clients),
		application.NewUpdateUser(repository, banks, clients),
		read,
	)
	mux.HandleFunc("POST /api/onboarding", guard.Session(handler.Create))
	mux.HandleFunc("GET /api/onboarding/{id}", guard.Contractor("id", handler.Find))
	mux.HandleFunc("PATCH /api/onboarding/{id}", guard.Contractor("id", handler.Update))
	return Module{
		Reader: read,
		Seed:   seed.Ensure,
	}
}
