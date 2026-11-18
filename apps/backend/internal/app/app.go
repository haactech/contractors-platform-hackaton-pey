package app

import (
	"context"
	"contractors-platform/backend/internal/modules/accounts"
	accdomain "contractors-platform/backend/internal/modules/accounts/domain"
	accinfra "contractors-platform/backend/internal/modules/accounts/infrastructure"
	"contractors-platform/backend/internal/modules/clients"
	clientsinfra "contractors-platform/backend/internal/modules/clients/infrastructure"
	"contractors-platform/backend/internal/modules/dashboard"
	"contractors-platform/backend/internal/modules/onboarding"
	onbdef "contractors-platform/backend/internal/modules/onboarding/definitions"
	onbdomain "contractors-platform/backend/internal/modules/onboarding/domain"
	onbinfra "contractors-platform/backend/internal/modules/onboarding/infrastructure"
	"contractors-platform/backend/internal/modules/payments"
	paymentsdomain "contractors-platform/backend/internal/modules/payments/domain"
	paymentsinfra "contractors-platform/backend/internal/modules/payments/infrastructure"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

// healthResponse is the public GET /api/health body (OpenAPI HealthResponse).
type healthResponse struct {
	Status string `json:"status"`
}

// Dependencies wires storage, external clients, and authentication policy into the API.
type Dependencies struct {
	Onboardings    onbdomain.Repository
	Accounts       accdomain.Repository
	Clients        clients.Repository
	ClientVendors  clients.Vendors
	Payments       paymentsdomain.Repository
	PaymentVendors paymentsdomain.VendorSource
	Collector      paymentsdomain.Collector
	Links          paymentsdomain.Links
	Banks          onbdomain.BankRegistrar
	Hasher         accdomain.PINHasher
	Mailer         accdomain.Mailer
	Policy         accdomain.Policy
	Clock          accdomain.Clock
}

// New builds the API with in-memory storage, strict auth policy, fast test PBKDF2, and no Pey key.
func New() http.Handler {
	handler, err := NewWith(Dependencies{
		Onboardings: onbinfra.NewMemoryRepository(),
		Accounts:    accinfra.NewMemoryRepository(),
		Banks:       onbinfra.NewUnavailablePey(),
		Hasher:      accounts.NewPBKDF2Hasher(1_000),
		Mailer:      accounts.NewLogMailer(),
		Policy:      accdomain.StrictPolicy(),
		Clock:       time.Now,
	})
	if err != nil {
		panic(err)
	}
	return handler
}

// NewWith builds the API with explicit dependencies and seeds the demo account when enabled.
func NewWith(deps Dependencies) (http.Handler, error) {
	if deps.Clients == nil {
		deps.Clients = clientsinfra.NewMemoryRepository()
	}
	if deps.ClientVendors == nil {
		deps.ClientVendors = vendorGate{repository: deps.Onboardings}
	}
	if deps.Payments == nil {
		deps.Payments = paymentsinfra.NewMemoryRepository()
	}
	if deps.PaymentVendors == nil {
		deps.PaymentVendors = paymentVendorGate{repository: deps.Onboardings}
	}
	if lookup, ok := deps.Banks.(bankStatusLookup); ok && deps.Onboardings != nil {
		deps.PaymentVendors = refreshingVendorSource{
			base:       deps.PaymentVendors,
			repository: deps.Onboardings,
			lookup:     lookup,
		}
	}
	if deps.Collector == nil {
		deps.Collector = paymentsinfra.UnavailableCollector{}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(healthResponse{Status: "ok"})
	})

	acc := accounts.Register(mux, accounts.Deps{
		Repository: deps.Accounts,
		Hasher:     deps.Hasher,
		Mailer:     deps.Mailer,
		Policy:     deps.Policy,
		Clock:      deps.Clock,
	})
	directory := clients.Register(mux, deps.Clients, deps.ClientVendors, acc.Guard)
	clientAdapter := onboardingClients{directory: directory}
	onb := onboarding.Register(mux, deps.Onboardings, deps.Banks, acc.Owners, clientAdapter, acc.Guard)
	_ = payments.Register(mux, deps.Payments, deps.PaymentVendors, paymentsClients{directory: directory}, deps.Collector, deps.Links, acc.Guard)
	dashboard.Register(mux, dashboard.NewOnboardingSource(onb.Reader), acc.Guard)

	if err := seedDemo(context.Background(), acc.Demo, onb.Seed); err != nil {
		return nil, err
	}
	return mux, nil
}

// vendorGate answers whether the onboarding repository already has that vendor.
type vendorGate struct{ repository onbdomain.Repository }

func (gate vendorGate) Exists(id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	_, found := gate.repository.Find(id)
	return found, nil
}

type bankStatusLookup interface {
	Lookup(ctx context.Context, bankAccountID string) ([]byte, error)
}

type paymentVendorGate struct{ repository onbdomain.Repository }

func (gate paymentVendorGate) Account(vendorID string) (paymentsdomain.Account, bool, error) {
	user, found := gate.repository.Find(vendorID)
	if !found {
		return paymentsdomain.Account{}, false, nil
	}
	return paymentsdomain.Account{
		BankAccountID: user.BankAccountID,
		CodiStatus:    user.CodiStatus,
	}, true, nil
}

// refreshingVendorSource syncs pending Pey bank account statuses on demand before creating a payment.
type refreshingVendorSource struct {
	base       paymentsdomain.VendorSource
	repository onbdomain.Repository
	lookup     bankStatusLookup
}

func (source refreshingVendorSource) Account(vendorID string) (paymentsdomain.Account, bool, error) {
	account, found, err := source.base.Account(vendorID)
	if err != nil || !found || account.BankAccountID == "" || account.CodiStatus == onbdef.CodiActive {
		return account, found, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	payload, lookupErr := source.lookup.Lookup(ctx, account.BankAccountID)
	if lookupErr != nil {
		log.Printf("payments: lookup bank status %s (%s): %v", vendorID, account.BankAccountID, lookupErr)
		return account, true, nil
	}
	refreshed, ok := onbdomain.DecodeBankAccount(payload)
	if !ok || refreshed.Status == account.CodiStatus {
		return account, true, nil
	}
	account.CodiStatus = refreshed.Status
	if user, ok := source.repository.Find(vendorID); ok {
		if updated, attachOK := onbdomain.AttachBankAccount(user, refreshed); attachOK {
			if _, saveErr := source.repository.RecordBankAccount(updated); saveErr != nil {
				log.Printf("payments: update refreshed bank status %s: %v", vendorID, saveErr)
			}
		}
	}
	return account, true, nil
}

type onboardingClients struct{ directory clients.Directory }

func (adapter onboardingClients) SaveFirst(vendorID string, customer onbdomain.Customer) error {
	return adapter.directory.SaveFirst(vendorID, customer.Name, customer.Phone, customer.Email)
}

func (adapter onboardingClients) First(vendorID string) (onbdomain.Customer, bool, error) {
	record, found, err := adapter.directory.First(vendorID)
	if err != nil || !found {
		return onbdomain.Customer{}, false, err
	}
	return onbdomain.Customer{Name: record.Name, Phone: record.Phone, Email: record.Email}, true, nil
}

func (adapter onboardingClients) RemoveForVendor(vendorID string) error {
	return adapter.directory.DeleteByVendor(vendorID)
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
