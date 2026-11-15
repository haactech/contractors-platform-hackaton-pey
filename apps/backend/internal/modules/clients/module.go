package clients

import (
	"contractors-platform/backend/internal/modules/clients/application"
	"contractors-platform/backend/internal/modules/clients/domain"
	"contractors-platform/backend/internal/modules/clients/presentation"
	"net/http"
)

// Record es el cliente que otros módulos pueden leer. No incluye la clave interna del teléfono.
type Record struct {
	ID            string
	VendorID      string
	EntityType    string
	Name          string
	Phone         string
	Email         string
	LegalName     string
	TradeName     string
	RFC           string
	Activity      string
	TaxRegime     string
	FiscalAddress string
	CreatedAt     string
	UpdatedAt     string
}

// Directory es la lectura y el alta del primer cliente. El HTTP de la pantalla vive en Register.
type Directory interface {
	SaveFirst(vendorID, name, phone, email string) error
	First(vendorID string) (Record, bool, error)
	Find(vendorID, clientID string) (Record, bool, error)
	List(vendorID string) ([]Record, error)
}

// Repository guarda los clientes. Vendors solo indica si el vendedor existe.
type Repository = domain.Repository

// Vendors no modifica el alta del vendedor.
type Vendors = domain.Vendors

// ErrVendorNotFound avisa que el vendedor no existe.
var ErrVendorNotFound = domain.ErrVendorNotFound

type facade struct{ service *application.Service }

// Register publica el directorio y devuelve la fachada que onboarding y payments usan.
func Register(mux *http.ServeMux, repository Repository, vendors Vendors) Directory {
	service := application.NewService(repository, vendors)
	handler := presentation.New(service)
	mux.HandleFunc("GET /api/contractors/{contractorId}/clients", handler.List)
	mux.HandleFunc("POST /api/contractors/{contractorId}/clients", handler.Create)
	mux.HandleFunc("GET /api/contractors/{contractorId}/clients/{clientId}", handler.Find)
	mux.HandleFunc("PATCH /api/contractors/{contractorId}/clients/{clientId}", handler.Update)
	return facade{service: service}
}

func (facade facade) SaveFirst(vendorID, name, phone, email string) error {
	return facade.service.SaveFirst(vendorID, name, phone, email)
}

func (facade facade) First(vendorID string) (Record, bool, error) {
	client, found, err := facade.service.First(vendorID)
	return toRecord(client), found, err
}

func (facade facade) Find(vendorID, clientID string) (Record, bool, error) {
	client, found, err := facade.service.Find(vendorID, clientID)
	return toRecord(client), found, err
}

func (facade facade) List(vendorID string) ([]Record, error) {
	clients, err := facade.service.List(vendorID)
	if err != nil {
		return nil, err
	}
	records := make([]Record, 0, len(clients))
	for _, client := range clients {
		records = append(records, toRecord(client))
	}
	return records, nil
}

func toRecord(client domain.Client) Record {
	return Record{
		ID: client.ID, VendorID: client.VendorID, EntityType: client.EntityType,
		Name: client.Name, Phone: client.Phone, Email: client.Email,
		LegalName: client.LegalName, TradeName: client.TradeName, RFC: client.RFC,
		Activity: client.Activity, TaxRegime: client.TaxRegime, FiscalAddress: client.FiscalAddress,
		CreatedAt: client.CreatedAt, UpdatedAt: client.UpdatedAt,
	}
}
