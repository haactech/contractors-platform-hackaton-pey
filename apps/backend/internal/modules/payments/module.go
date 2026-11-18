package payments

import (
	"contractors-platform/backend/internal/modules/payments/application"
	"contractors-platform/backend/internal/modules/payments/domain"
	"contractors-platform/backend/internal/modules/payments/presentation"
	"contractors-platform/backend/internal/platform/access"
	"net/http"
)

// Record es el cobro que otros módulos pueden leer (proyección pública).
type Record = domain.View

// Reader es la lectura pública de cobros guardados.
type Reader interface {
	Find(vendorID, paymentID string) (Record, bool, error)
	ListByVendor(vendorID string) ([]Record, error)
}

// Module expone las capacidades públicas del módulo payments.
type Module struct {
	Reader Reader
}

// Register publica POST /api/payments y la lectura por vendedor.
func Register(mux *http.ServeMux, repository domain.Repository, vendors domain.VendorSource, clients domain.ClientDirectory, collector domain.Collector, links domain.Links, guard access.Guard) Module {
	create := application.NewCreatePayment(repository, vendors, clients, collector, links)
	read := application.NewReadPayments(repository)
	handler := presentation.New(create, read)
	mux.HandleFunc("POST /api/payments", guard.Session(handler.Create))
	mux.HandleFunc("GET /api/contractors/{contractorId}/payments", guard.Contractor("contractorId", handler.List))
	mux.HandleFunc("GET /api/contractors/{contractorId}/payments/{paymentId}", guard.Contractor("contractorId", handler.Find))
	return Module{Reader: read}
}
