package payments

import (
	"contractors-platform/backend/internal/modules/payments/application"
	"contractors-platform/backend/internal/modules/payments/domain"
	"contractors-platform/backend/internal/modules/payments/presentation"
	"contractors-platform/backend/internal/platform/access"
	"net/http"
)

// Register publica POST /api/payments.
func Register(mux *http.ServeMux, repository domain.Repository, vendors domain.VendorSource, clients domain.ClientDirectory, collector domain.Collector, links domain.Links, guard access.Guard) {
	handler := presentation.New(application.NewCreatePayment(repository, vendors, clients, collector, links))
	mux.HandleFunc("POST /api/payments", guard.Session(handler.Create))
}
