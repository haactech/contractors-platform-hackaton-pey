package app

import (
	"contractors-platform/backend/internal/modules/clients"
	clientsinfra "contractors-platform/backend/internal/modules/clients/infrastructure"
	"contractors-platform/backend/internal/modules/dashboard"
	"contractors-platform/backend/internal/modules/onboarding"
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"contractors-platform/backend/internal/modules/onboarding/infrastructure"
	"contractors-platform/backend/internal/modules/payments"
	paymentsdomain "contractors-platform/backend/internal/modules/payments/domain"
	paymentsinfra "contractors-platform/backend/internal/modules/payments/infrastructure"
	"errors"
	"net/http"
)

// New builds the API with in-memory storage and no Pey key.
func New() http.Handler {
	return NewWith(infrastructure.NewMemoryRepository(), infrastructure.NewUnavailablePey())
}

// NewWith builds the API on the given onboarding repository and Pey registrar.
// Clients live in memory and the vendor check reads that same repository.
func NewWith(repository domain.Repository, banks domain.BankRegistrar) http.Handler {
	return Mount(repository, banks, paymentsinfra.NewMemoryRepository(), paymentsinfra.NewMemoryVendors(), paymentsinfra.UnavailableCollector{}, paymentsdomain.Links{}, clientsinfra.NewMemoryRepository(), vendorGate{repository: repository})
}

// Mount builds the API with the onboarding store, the clients store and the payments store.
func Mount(repository domain.Repository, banks domain.BankRegistrar, paymentsRepository paymentsdomain.Repository, vendors paymentsdomain.VendorSource, collector paymentsdomain.Collector, links paymentsdomain.Links, clientsRepository clients.Repository, clientVendors clients.Vendors) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/contractors", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"contractors":[]}`))
	})
	directory := clients.Register(mux, clientsRepository, clientVendors)
	onboardingReader := onboarding.Register(mux, repository, banks, onboardingClients{directory: directory})
	payments.Register(mux, paymentsRepository, vendors, paymentsClients{directory: directory}, collector, links)
	dashboard.Register(mux, dashboard.NewOnboardingSource(onboardingReader))
	return mux
}

// vendorGate answers whether the onboarding repository already has that vendor.
type vendorGate struct{ repository domain.Repository }

func (gate vendorGate) Exists(id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	_, found := gate.repository.Find(id)
	return found, nil
}

type onboardingClients struct{ directory clients.Directory }

func (adapter onboardingClients) SaveFirst(vendorID string, customer domain.Customer) error {
	return adapter.directory.SaveFirst(vendorID, customer.Name, customer.Phone, customer.Email)
}

func (adapter onboardingClients) First(vendorID string) (domain.Customer, bool, error) {
	record, found, err := adapter.directory.First(vendorID)
	if err != nil || !found {
		return domain.Customer{}, false, err
	}
	return domain.Customer{Name: record.Name, Phone: record.Phone, Email: record.Email}, true, nil
}

type paymentsClients struct{ directory clients.Directory }

func (adapter paymentsClients) Find(vendorID, clientID string) (paymentsdomain.Client, bool, error) {
	record, found, err := adapter.directory.Find(vendorID, clientID)
	if errors.Is(err, clients.ErrVendorNotFound) {
		return paymentsdomain.Client{}, false, paymentsdomain.ErrVendorNotFound
	}
	if err != nil || !found {
		return paymentsdomain.Client{}, false, err
	}
	return paymentClient(record), true, nil
}

func (adapter paymentsClients) List(vendorID string) ([]paymentsdomain.Client, error) {
	records, err := adapter.directory.List(vendorID)
	if errors.Is(err, clients.ErrVendorNotFound) {
		return nil, paymentsdomain.ErrVendorNotFound
	}
	if err != nil {
		return nil, err
	}
	clientsForVendor := make([]paymentsdomain.Client, 0, len(records))
	for _, record := range records {
		clientsForVendor = append(clientsForVendor, paymentClient(record))
	}
	return clientsForVendor, nil
}

func paymentClient(record clients.Record) paymentsdomain.Client {
	return paymentsdomain.Client{ID: record.ID, Name: record.Name, Phone: record.Phone, Email: record.Email}
}
