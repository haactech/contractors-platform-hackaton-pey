package presentation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"contractors-platform/backend/internal/modules/payments/application"
	"contractors-platform/backend/internal/modules/payments/domain"
	"contractors-platform/backend/internal/modules/payments/infrastructure"
)

func mount(repository *infrastructure.MemoryRepository, vendors *infrastructure.MemoryVendors, clients fakeClients, collector *scriptedCollector, links domain.Links) http.Handler {
	mux := http.NewServeMux()
	handler := New(application.NewCreatePayment(repository, vendors, clients, collector, links), application.NewReadPayments(repository))
	mux.HandleFunc("POST /api/payments", handler.Create)
	mux.HandleFunc("GET /api/contractors/{contractorId}/payments", handler.List)
	mux.HandleFunc("GET /api/contractors/{contractorId}/payments/{paymentId}", handler.Find)
	return mux
}

type fakeClients struct {
	records map[string][]domain.Client
}

func (clients fakeClients) List(vendorID string) ([]domain.Client, error) {
	records, found := clients.records[vendorID]
	if !found {
		return nil, domain.ErrVendorNotFound
	}
	return records, nil
}

func (clients fakeClients) Find(vendorID, clientID string) (domain.Client, bool, error) {
	records, found := clients.records[vendorID]
	if !found {
		return domain.Client{}, false, domain.ErrVendorNotFound
	}
	for _, client := range records {
		if client.ID == clientID {
			return client, true, nil
		}
	}
	return domain.Client{}, false, nil
}

func readyClients() fakeClients {
	return fakeClients{records: map[string][]domain.Client{
		"vendor-1": {{ID: "client-1", Name: "Luis", Phone: "+52 555 123 4567", Email: "luis@example.com"}},
		"vendor-2": {{ID: "client-2", Name: "Ana", Phone: "5511111111", Email: "ana@example.com"}},
	}}
}

type scriptedCollector struct {
	response []byte
	err      error
	calls    int
	body     []byte
}

func (collector *scriptedCollector) Collect(_ context.Context, _ string, body []byte) ([]byte, error) {
	collector.calls++
	collector.body = append([]byte(nil), body...)
	return collector.response, collector.err
}

func readyVendors() *infrastructure.MemoryVendors {
	vendors := infrastructure.NewMemoryVendors()
	vendors.Set("vendor-1", domain.Account{BankAccountID: "bank-vendor-1", CodiStatus: "ACTIVE"})
	vendors.Set("vendor-2", domain.Account{BankAccountID: "bank-other", CodiStatus: "ACTIVE"})
	return vendors
}

func postPayment(handler http.Handler, payload any) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	request := httptest.NewRequest(http.MethodPost, "/api/payments", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func validBody(method string) map[string]any {
	return map[string]any{
		"onboardingId": "vendor-1",
		"clientName":   "Luis",
		"clientPhone":  "5551234567",
		"concept":      "Mensualidad",
		"amountCents":  15000,
		"dueDate":      "2026-12-01",
		"method":       method,
	}
}

func TestCreateEachMethod(t *testing.T) {
	repository := infrastructure.NewMemoryRepository()
	collector := &scriptedCollector{}
	links := domain.Links{Success: "https://example.com/ok", Back: "https://example.com/back"}
	mux := mount(repository, readyVendors(), readyClients(), collector, links)

	collector.response = []byte(`{"id":"pey-link","url":"https://pay.example/abc"}`)
	response := postPayment(mux, validBody("PAYMENT_LINK"))
	if response.Code != http.StatusCreated {
		t.Fatalf("link: %d %s", response.Code, response.Body)
	}
	if strings.Contains(response.Body.String(), "bank-vendor-1") || strings.Contains(collectorBody(collector), "bankAccountId") {
		t.Fatalf("bank id leaked: response=%s pey=%s", response.Body, collector.body)
	}
	var link map[string]any
	_ = json.Unmarshal(response.Body.Bytes(), &link)
	if link["paymentLinkUrl"] != "https://pay.example/abc" || link["status"] != "PENDING_PAYMENT" || link["amountCents"] != float64(15000) || link["clientEmail"] != "luis@example.com" || link["clientId"] != "client-1" {
		t.Fatalf("link body: %s", response.Body)
	}

	collector.response = []byte(`{"id":"pey-qr","qrString":"QRDATA","qrImage":"img"}`)
	response = postPayment(mux, validBody("CODI_QR"))
	if response.Code != http.StatusCreated {
		t.Fatalf("qr: %d %s", response.Code, response.Body)
	}
	if !strings.Contains(string(collector.body), `"bankAccountId":"bank-vendor-1"`) || strings.Contains(string(collector.body), "bank-other") {
		t.Fatalf("qr pey body: %s", collector.body)
	}
	var qr map[string]any
	_ = json.Unmarshal(response.Body.Bytes(), &qr)
	if qr["codiQrId"] != "pey-qr" || qr["codiQrString"] != "QRDATA" || qr["codiQrImage"] != "img" {
		t.Fatalf("qr body: %s", response.Body)
	}
	if len(repository.Stored()) != 2 {
		t.Fatalf("stored %d", len(repository.Stored()))
	}
}

func TestInactiveAccountAndUnknownClient(t *testing.T) {
	vendors := readyVendors()
	vendors.Set("vendor-1", domain.Account{BankAccountID: "bank-vendor-1", CodiStatus: "PENDING"})
	repository := infrastructure.NewMemoryRepository()
	collector := &scriptedCollector{response: []byte(`{"id":"pey-qr","qrString":"QRDATA"}`)}
	mux := mount(repository, vendors, readyClients(), collector, domain.Links{Success: "https://example.com/ok", Back: "https://example.com/back"})

	response := postPayment(mux, validBody("CODI_QR"))
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "La cuenta todavía no puede recibir CoDi.") {
		t.Fatalf("inactive: %d %s", response.Code, response.Body)
	}
	unknown := validBody("CODI_QR")
	unknown["clientName"] = "Nadie"
	vendors.Set("vendor-1", domain.Account{BankAccountID: "bank-vendor-1", CodiStatus: "ACTIVE"})
	response = postPayment(mux, unknown)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "Este cliente no está registrado.") {
		t.Fatalf("client: %d %s", response.Code, response.Body)
	}
	if collector.calls != 0 || len(repository.Stored()) != 0 {
		t.Fatalf("calls=%d stored=%d", collector.calls, len(repository.Stored()))
	}
}

func TestLinkWithoutHTTPSUrls(t *testing.T) {
	repository := infrastructure.NewMemoryRepository()
	collector := &scriptedCollector{response: []byte(`{"id":"pey-link","url":"https://pay.example/abc"}`)}
	mux := mount(repository, readyVendors(), readyClients(), collector, domain.Links{})
	response := postPayment(mux, validBody("PAYMENT_LINK"))
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "Configura las URLs HTTPS de retorno del cobro.") {
		t.Fatalf("links: %d %s", response.Code, response.Body)
	}
	if collector.calls != 0 || len(repository.Stored()) != 0 {
		t.Fatalf("calls=%d stored=%d", collector.calls, len(repository.Stored()))
	}
}

func TestUnknownJSONField(t *testing.T) {
	mux := mount(infrastructure.NewMemoryRepository(), readyVendors(), readyClients(), &scriptedCollector{}, domain.Links{})
	body := validBody("CODI_QR")
	body["bankAccountId"] = "spoofed"
	response := postPayment(mux, body)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field: %d %s", response.Code, response.Body)
	}
}

func TestPeyFailureStoresNothing(t *testing.T) {
	repository := infrastructure.NewMemoryRepository()
	collector := &scriptedCollector{err: errors.New("pey down")}
	mux := mount(repository, readyVendors(), readyClients(), collector, domain.Links{})
	response := postPayment(mux, validBody("CODI_QR"))
	if response.Code != http.StatusBadGateway || len(repository.Stored()) != 0 {
		t.Fatalf("pey: %d stored=%d %s", response.Code, len(repository.Stored()), response.Body)
	}
}

func TestMissingVendor(t *testing.T) {
	mux := mount(infrastructure.NewMemoryRepository(), readyVendors(), readyClients(), &scriptedCollector{}, domain.Links{})
	body := validBody("CODI_QR")
	body["onboardingId"] = "missing"
	response := postPayment(mux, body)
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "No se encontró el vendedor.") {
		t.Fatalf("missing: %d %s", response.Code, response.Body)
	}
}

func TestPushReturnsCreatedWithoutALink(t *testing.T) {
	repository := infrastructure.NewMemoryRepository()
	collector := &scriptedCollector{response: []byte(`{"transactionId":"txn-1","codiId":"folio-1"}`)}
	mux := mount(repository, readyVendors(), readyClients(), collector, domain.Links{})
	body := validBody("CODI_PUSH")
	body["clientId"] = "client-1"
	body["clientPhone"] = "5511111111"
	response := postPayment(mux, body)
	if response.Code != http.StatusCreated {
		t.Fatalf("push: %d %s", response.Code, response.Body)
	}
	if strings.Contains(response.Body.String(), "paymentLinkUrl") || strings.Contains(response.Body.String(), "codiQr") || strings.Contains(response.Body.String(), "bank-vendor-1") {
		t.Fatalf("push leaked a link, qr or bank id: %s", response.Body)
	}
	if !strings.Contains(string(collector.body), `"phoneNumber":"5551234567"`) || !strings.Contains(string(collector.body), `"bankAccountId":"bank-vendor-1"`) || !strings.Contains(string(collector.body), `"expiration":360`) {
		t.Fatalf("pey body: %s", collector.body)
	}
	stored := repository.Stored()
	if len(stored) != 1 || stored[0].PeyID != "txn-1" || stored[0].ClientID != "client-1" || stored[0].ClientPhone != "+52 555 123 4567" {
		t.Fatalf("stored: %+v", stored)
	}
}

func TestListAndFindAfterCreate(t *testing.T) {
	repository := infrastructure.NewMemoryRepository()
	collector := &scriptedCollector{response: []byte(`{"id":"pey-qr","qrString":"QRDATA","qrImage":"img"}`)}
	mux := mount(repository, readyVendors(), readyClients(), collector, domain.Links{})
	created := postPayment(mux, validBody("CODI_QR"))
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body)
	}
	var payment map[string]any
	_ = json.Unmarshal(created.Body.Bytes(), &payment)
	paymentID, _ := payment["id"].(string)
	if paymentID == "" {
		t.Fatalf("missing id: %s", created.Body)
	}

	list := httptest.NewRecorder()
	mux.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/contractors/vendor-1/payments", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list: %d %s", list.Code, list.Body)
	}
	var listed []map[string]any
	_ = json.Unmarshal(list.Body.Bytes(), &listed)
	if len(listed) != 1 || listed[0]["id"] != paymentID || listed[0]["status"] != "PENDING_PAYMENT" || listed[0]["clientId"] != "client-1" {
		t.Fatalf("listed: %s", list.Body)
	}
	if strings.Contains(list.Body.String(), "bank-vendor-1") {
		t.Fatalf("bank id leaked: %s", list.Body)
	}

	find := httptest.NewRecorder()
	mux.ServeHTTP(find, httptest.NewRequest(http.MethodGet, "/api/contractors/vendor-1/payments/"+paymentID, nil))
	if find.Code != http.StatusOK || !strings.Contains(find.Body.String(), `"id":"`+paymentID+`"`) {
		t.Fatalf("find: %d %s", find.Code, find.Body)
	}
	missing := httptest.NewRecorder()
	mux.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/contractors/vendor-2/payments/"+paymentID, nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("scoped find: %d %s", missing.Code, missing.Body)
	}
}

func collectorBody(collector *scriptedCollector) string {
	return string(collector.body)
}
