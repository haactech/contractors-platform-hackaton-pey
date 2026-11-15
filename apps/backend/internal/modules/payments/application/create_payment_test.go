package application

import (
	"context"
	"contractors-platform/backend/internal/modules/payments/definitions"
	"contractors-platform/backend/internal/modules/payments/domain"
	"encoding/json"
	"errors"
	"testing"
)

type fakeVendors struct {
	accounts map[string]domain.Account
}

func (vendors fakeVendors) Account(vendorID string) (domain.Account, bool, error) {
	account, found := vendors.accounts[vendorID]
	return account, found, nil
}

type fakeDirectory struct {
	clients map[string][]domain.Client
}

func (directory fakeDirectory) List(vendorID string) ([]domain.Client, error) {
	clients, found := directory.clients[vendorID]
	if !found {
		return nil, domain.ErrVendorNotFound
	}
	copied := make([]domain.Client, len(clients))
	copy(copied, clients)
	return copied, nil
}

func (directory fakeDirectory) Find(vendorID, clientID string) (domain.Client, bool, error) {
	clients, found := directory.clients[vendorID]
	if !found {
		return domain.Client{}, false, domain.ErrVendorNotFound
	}
	for _, client := range clients {
		if client.ID == clientID {
			return client, true, nil
		}
	}
	return domain.Client{}, false, nil
}

type fakeRepository struct {
	saved []domain.Payment
}

func (repository *fakeRepository) Allocate() (string, error) { return "pay-1", nil }

func (repository *fakeRepository) Save(payment domain.Payment) (domain.Payment, error) {
	if fieldErrors := domain.Allow(payment); len(fieldErrors) > 0 {
		return domain.Payment{}, errors.New("rechazado")
	}
	payment.CreatedAt = "2026-11-14T00:00:00Z"
	repository.saved = append(repository.saved, payment)
	return payment, nil
}

type fakeCollector struct {
	response []byte
	err      error
	calls    int
	path     string
	body     []byte
}

func (collector *fakeCollector) Collect(_ context.Context, path string, body []byte) ([]byte, error) {
	collector.calls++
	collector.path = path
	collector.body = append([]byte(nil), body...)
	return collector.response, collector.err
}

func vendors() fakeVendors {
	return fakeVendors{accounts: map[string]domain.Account{
		"vendor-1": {BankAccountID: "bank-vendor-1", CodiStatus: definitions.CodiActive},
		"vendor-2": {BankAccountID: "bank-vendor-2", CodiStatus: definitions.CodiActive},
	}}
}

func directory() fakeDirectory {
	return fakeDirectory{clients: map[string][]domain.Client{
		"vendor-1": {{ID: "client-1", Name: "Luis", Phone: "+52 555 123 4567", Email: "luis@example.com"}},
		"vendor-2": {{ID: "client-2", Name: "Ana", Phone: "5511111111", Email: "ana@example.com"}},
	}}
}

func request(method string) domain.Request {
	return domain.Request{
		VendorID:    "vendor-1",
		ClientName:  "Luis",
		ClientPhone: "5551234567",
		Concept:     "Mensualidad",
		AmountCents: 15000,
		DueDate:     "2026-12-01",
		Method:      method,
	}
}

func TestCreateStoresTheRequestedAmountAndClient(t *testing.T) {
	repository := &fakeRepository{}
	collector := &fakeCollector{response: []byte(`{"id":"pey-qr","qrString":"QRDATA","qrImage":"img"}`)}
	view, fieldErrors, err := NewCreatePayment(repository, vendors(), directory(), collector, domain.Links{}).Create(context.Background(), request(definitions.MethodCodiQR))
	if err != nil || len(fieldErrors) > 0 || collector.calls != 1 {
		t.Fatalf("create: %v %v calls=%d", err, fieldErrors, collector.calls)
	}
	if len(repository.saved) != 1 {
		t.Fatalf("saved %d", len(repository.saved))
	}
	saved := repository.saved[0]
	if saved.AmountCents != 15000 || saved.ClientName != "Luis" || saved.ClientEmail != "luis@example.com" || saved.ClientPhone != "+52 555 123 4567" || saved.ClientID != "client-1" || saved.BankAccountID != "bank-vendor-1" {
		t.Fatalf("stored: %+v", saved)
	}
	if saved.VendorID != "vendor-1" || view.AmountCents != 15000 || view.CodiQRString != "QRDATA" {
		t.Fatalf("view: %+v", view)
	}
	var body map[string]any
	if err := json.Unmarshal(collector.body, &body); err != nil {
		t.Fatal(err)
	}
	if body[definitions.PeyBankAccountID] != "bank-vendor-1" || body[definitions.PeyBankAccountID] == "bank-vendor-2" {
		t.Fatalf("pey bank id: %v", body[definitions.PeyBankAccountID])
	}
	if collector.path != definitions.PeyPathCodiQR {
		t.Fatalf("path: %s", collector.path)
	}
}

func TestInactiveAccountDoesNotCallPey(t *testing.T) {
	source := vendors()
	source.accounts["vendor-1"] = domain.Account{BankAccountID: "bank-vendor-1", CodiStatus: "PENDING"}
	repository := &fakeRepository{}
	collector := &fakeCollector{}
	for _, method := range []string{definitions.MethodPaymentLink, definitions.MethodCodiQR} {
		_, fieldErrors, err := NewCreatePayment(repository, source, directory(), collector, domain.Links{Success: "https://example.com/ok", Back: "https://example.com/back"}).Create(context.Background(), request(method))
		if err != nil || fieldErrors[definitions.FieldMethod] != definitions.MessageAccount {
			t.Fatalf("%s: %v %v", method, err, fieldErrors)
		}
	}
	if collector.calls != 0 || len(repository.saved) != 0 {
		t.Fatalf("calls=%d saved=%d", collector.calls, len(repository.saved))
	}
}

func TestUnknownClientDoesNotCallPey(t *testing.T) {
	repository := &fakeRepository{}
	collector := &fakeCollector{}
	input := request(definitions.MethodCodiQR)
	input.ClientName = "Nadie"
	_, fieldErrors, err := NewCreatePayment(repository, vendors(), directory(), collector, domain.Links{}).Create(context.Background(), input)
	if err != nil || fieldErrors[definitions.FieldClientName] != definitions.MessageClientMissing || collector.calls != 0 || len(repository.saved) != 0 {
		t.Fatalf("create: %v %v calls=%d saved=%d", err, fieldErrors, collector.calls, len(repository.saved))
	}
}

func TestMissingVendorDoesNotCallPey(t *testing.T) {
	repository := &fakeRepository{}
	collector := &fakeCollector{}
	input := request(definitions.MethodCodiQR)
	input.VendorID = "missing"
	_, fieldErrors, err := NewCreatePayment(repository, vendors(), directory(), collector, domain.Links{}).Create(context.Background(), input)
	if !errors.Is(err, domain.ErrVendorNotFound) || len(fieldErrors) != 0 || collector.calls != 0 {
		t.Fatalf("create: %v %v calls=%d", err, fieldErrors, collector.calls)
	}
}

func TestPeyErrorLeavesTheStoreEmpty(t *testing.T) {
	repository := &fakeRepository{}
	collector := &fakeCollector{err: errors.New("pey down")}
	_, _, err := NewCreatePayment(repository, vendors(), directory(), collector, domain.Links{}).Create(context.Background(), request(definitions.MethodCodiQR))
	if !errors.Is(err, domain.ErrCollector) || len(repository.saved) != 0 {
		t.Fatalf("err=%v saved=%d", err, len(repository.saved))
	}
	collector.err = nil
	collector.response = []byte(`{"id":"pey-qr"}`)
	_, _, err = NewCreatePayment(repository, vendors(), directory(), collector, domain.Links{}).Create(context.Background(), request(definitions.MethodCodiQR))
	if !errors.Is(err, domain.ErrCollector) || len(repository.saved) != 0 {
		t.Fatalf("bad payload err=%v saved=%d", err, len(repository.saved))
	}
}

func TestLinkWithoutHTTPSDoesNotCallPey(t *testing.T) {
	repository := &fakeRepository{}
	collector := &fakeCollector{response: []byte(`{"id":"pey-link","url":"https://pay.example/abc"}`)}
	_, _, err := NewCreatePayment(repository, vendors(), directory(), collector, domain.Links{}).Create(context.Background(), request(definitions.MethodPaymentLink))
	if err == nil || errors.Is(err, domain.ErrCollector) || collector.calls != 0 || len(repository.saved) != 0 {
		t.Fatalf("err=%v calls=%d saved=%d", err, collector.calls, len(repository.saved))
	}
}

func TestClientIDStoresThatClientAndIgnoresTheRequestPhone(t *testing.T) {
	repository := &fakeRepository{}
	collector := &fakeCollector{response: []byte(`{"transactionId":"txn-1","codiId":"folio-1"}`)}
	input := request(definitions.MethodCodiPush)
	input.ClientID = "client-1"
	input.ClientPhone = "5511111111"
	input.ClientName = "Otra"
	view, fieldErrors, err := NewCreatePayment(repository, vendors(), directory(), collector, domain.Links{}).Create(context.Background(), input)
	if err != nil || len(fieldErrors) > 0 || collector.path != definitions.PeyPathCodiPush {
		t.Fatalf("create: %v %v path=%s", err, fieldErrors, collector.path)
	}
	saved := repository.saved[0]
	if saved.ClientID != "client-1" || saved.ClientPhone != "+52 555 123 4567" || saved.ClientName != "Luis" || view.CodiQRString != "" || view.PaymentLinkURL != "" {
		t.Fatalf("stored: %+v view=%+v", saved, view)
	}
	var body map[string]any
	if err := json.Unmarshal(collector.body, &body); err != nil {
		t.Fatal(err)
	}
	if body[definitions.PeyPhoneNumber] != "5551234567" || body[definitions.PeyBankAccountID] != "bank-vendor-1" || body[definitions.PeyExpiration] != float64(definitions.PushExpirationSeconds) {
		t.Fatalf("pey body: %s", collector.body)
	}
	if _, exists := body["numRef"]; exists {
		t.Fatalf("push sent numRef: %s", collector.body)
	}
}

func TestUnknownClientIDDoesNotCallPey(t *testing.T) {
	repository := &fakeRepository{}
	collector := &fakeCollector{}
	input := request(definitions.MethodCodiPush)
	input.ClientID = "missing-client"
	_, fieldErrors, err := NewCreatePayment(repository, vendors(), directory(), collector, domain.Links{}).Create(context.Background(), input)
	if err != nil || fieldErrors[definitions.FieldClientID] != definitions.MessageClientMissing || collector.calls != 0 || len(repository.saved) != 0 {
		t.Fatalf("create: %v %v calls=%d", err, fieldErrors, collector.calls)
	}
}

func TestPushRejectsAShortPhoneAndAnInactiveAccount(t *testing.T) {
	repository := &fakeRepository{}
	collector := &fakeCollector{response: []byte(`{"transactionId":"txn-1"}`)}
	clients := directory()
	clients.clients["vendor-1"] = []domain.Client{{ID: "client-1", Name: "Luis", Phone: "1234567", Email: "luis@example.com"}}
	short := request(definitions.MethodCodiPush)
	short.ClientID = "client-1"
	_, fieldErrors, err := NewCreatePayment(repository, vendors(), clients, collector, domain.Links{}).Create(context.Background(), short)
	if err != nil || fieldErrors[definitions.FieldClientPhone] != definitions.MessagePushPhone || collector.calls != 0 {
		t.Fatalf("phone: %v %v calls=%d", err, fieldErrors, collector.calls)
	}
	source := vendors()
	source.accounts["vendor-1"] = domain.Account{BankAccountID: "bank-vendor-1", CodiStatus: "PENDING"}
	_, fieldErrors, err = NewCreatePayment(repository, source, directory(), collector, domain.Links{}).Create(context.Background(), request(definitions.MethodCodiPush))
	if err != nil || fieldErrors[definitions.FieldMethod] != definitions.MessageAccount || collector.calls != 0 || len(repository.saved) != 0 {
		t.Fatalf("account: %v %v calls=%d saved=%d", err, fieldErrors, collector.calls, len(repository.saved))
	}
}

func TestPushPeyErrorLeavesTheStoreEmpty(t *testing.T) {
	repository := &fakeRepository{}
	collector := &fakeCollector{err: errors.New("pey down")}
	_, _, err := NewCreatePayment(repository, vendors(), directory(), collector, domain.Links{}).Create(context.Background(), request(definitions.MethodCodiPush))
	if !errors.Is(err, domain.ErrCollector) || len(repository.saved) != 0 {
		t.Fatalf("err=%v saved=%d", err, len(repository.saved))
	}
	collector.err = nil
	collector.response = []byte(`{"id":"not-the-push-id"}`)
	_, _, err = NewCreatePayment(repository, vendors(), directory(), collector, domain.Links{}).Create(context.Background(), request(definitions.MethodCodiPush))
	if !errors.Is(err, domain.ErrCollector) || len(repository.saved) != 0 {
		t.Fatalf("bad payload err=%v saved=%d", err, len(repository.saved))
	}
}
