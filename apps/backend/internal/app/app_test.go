package app

import (
	"bytes"
	"context"
	clientsinfra "contractors-platform/backend/internal/modules/clients/infrastructure"
	dashboarddomain "contractors-platform/backend/internal/modules/dashboard/domain"
	dashboardpresentation "contractors-platform/backend/internal/modules/dashboard/presentation"
	"contractors-platform/backend/internal/modules/onboarding/definitions"
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"contractors-platform/backend/internal/modules/onboarding/infrastructure"
	paymentsdomain "contractors-platform/backend/internal/modules/payments/domain"
	paymentsinfra "contractors-platform/backend/internal/modules/payments/infrastructure"
	"contractors-platform/backend/internal/platform/database"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func validInput() map[string]any {
	return map[string]any{"name": " Ana ", "profession": "Electrician", "client": map[string]any{"name": "Luis", "phone": "+52 555 123 4567", "email": "luis@example.com"}, "businessName": "Electrical Services", "businessActivity": "Electrical installations", "rfc": " aaa010101aaa ", "clabe": "002 010 077777777771"}
}

func submit(handler http.Handler, payload any) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	request := httptest.NewRequest(http.MethodPost, "/api/onboarding", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestCreateAndRetrieve(t *testing.T) {
	for _, withTax := range []bool{false, true} {
		t.Run(map[bool]string{false: "without tax regime", true: "with tax regime"}[withTax], func(t *testing.T) {
			handler := New()
			input := validInput()
			if withTax {
				input["taxRegime"] = "General"
			}
			response := submit(handler, input)
			if response.Code != http.StatusCreated {
				t.Fatalf("create: %d %s", response.Code, response.Body)
			}
			var result map[string]any
			_ = json.Unmarshal(response.Body.Bytes(), &result)
			if result["id"] == "" || result["name"] != "Ana" || result["rfc"] != "AAA010101AAA" || result["clabe"] != "002010077777777771" {
				t.Fatalf("unexpected record: %v", result)
			}
			if _, exists := result["taxRegime"]; exists != withTax {
				t.Fatalf("tax regime presence: %v", result)
			}
			location := response.Header().Get("Location")
			if location != "/api/onboarding/"+result["id"].(string) {
				t.Fatalf("invalid Location: %s", location)
			}
			fetched := httptest.NewRecorder()
			handler.ServeHTTP(fetched, httptest.NewRequest(http.MethodGet, location, nil))
			if fetched.Code != http.StatusOK || fetched.Body.String() != response.Body.String() {
				t.Fatalf("fetch: %d %s", fetched.Code, fetched.Body)
			}
		})
	}
}

func TestRequiredFields(t *testing.T) {
	for _, field := range []string{"name", "profession", "client.name", "client.phone", "client.email", "businessName", "businessActivity", "rfc", "clabe"} {
		t.Run(field, func(t *testing.T) {
			input := validInput()
			if strings.HasPrefix(field, "client.") {
				input["client"].(map[string]any)[strings.TrimPrefix(field, "client.")] = "  "
			} else {
				input[field] = "  "
			}
			response := submit(New(), input)
			if response.Code != 422 {
				t.Fatalf("expected 422: %d %s", response.Code, response.Body)
			}
			var result struct {
				Errors map[string]string `json:"errors"`
			}
			_ = json.Unmarshal(response.Body.Bytes(), &result)
			if result.Errors[field] == "" {
				t.Fatalf("missing field error: %s", response.Body)
			}
		})
	}
}

func TestInvalidFormats(t *testing.T) {
	for _, field := range []string{"email", "phone", "rfc", "clabe"} {
		t.Run(field, func(t *testing.T) {
			input := validInput()
			if field == "rfc" || field == "clabe" {
				input[field] = "invalid"
			} else {
				input["client"].(map[string]any)[field] = "invalid"
			}
			if response := submit(New(), input); response.Code != 422 {
				t.Fatalf("expected 422: %d", response.Code)
			}
		})
	}
}

type scriptedPey struct {
	response []byte
	err      error
	calls    int
}

func (pey *scriptedPey) Register(context.Context, []byte) ([]byte, error) {
	pey.calls++
	return pey.response, pey.err
}

func TestPeyAccountIsStoredAndHidden(t *testing.T) {
	repository := infrastructure.NewMemoryRepository()
	pey := &scriptedPey{response: []byte(`{"id":"01BANK","codiStatus":"PENDING"}`)}
	response := submit(NewWith(repository, pey), validInput())
	if response.Code != http.StatusCreated || pey.calls != 1 {
		t.Fatalf("create: %d calls=%d %s", response.Code, pey.calls, response.Body)
	}
	if strings.Contains(response.Body.String(), "01BANK") || strings.Contains(response.Body.String(), "codiStatus") || strings.Contains(response.Body.String(), "PENDING") {
		t.Fatalf("status leaked: %s", response.Body)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &created)
	stored, found := repository.Find(created.ID)
	if !found || stored.BankAccountID != "01BANK" || stored.CodiStatus != definitions.CodiPending {
		t.Fatalf("stored: found=%v %+v", found, stored)
	}
}

func TestPeyFailureKeepsSavedUser(t *testing.T) {
	repository := infrastructure.NewMemoryRepository()
	pey := &scriptedPey{err: errors.New("pey down")}
	response := submit(NewWith(repository, pey), validInput())
	if response.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", response.Code, response.Body)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &created)
	stored, found := repository.Find(created.ID)
	if !found || stored.BankAccountID != "" || stored.CodiStatus != domain.NotRegisteredStatus() {
		t.Fatalf("stored: found=%v %+v", found, stored)
	}
}

func TestUnknownPeyStatusDoesNotOverwriteUser(t *testing.T) {
	repository := infrastructure.NewMemoryRepository()
	pey := &scriptedPey{response: []byte(`{"id":"01BANK","codiStatus":"MAYBE"}`)}
	response := submit(NewWith(repository, pey), validInput())
	if response.Code != http.StatusCreated || pey.calls != 1 {
		t.Fatalf("create: %d calls=%d %s", response.Code, pey.calls, response.Body)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &created)
	stored, found := repository.Find(created.ID)
	if !found || stored.BankAccountID != "" || stored.CodiStatus != domain.NotRegisteredStatus() {
		t.Fatalf("stored: found=%v %+v", found, stored)
	}
}

func TestDuplicateCLABEDoesNotCallPey(t *testing.T) {
	pey := &scriptedPey{response: []byte(`{"id":"01BANK","codiStatus":"PENDING"}`)}
	handler := NewWith(infrastructure.NewMemoryRepository(), pey)
	if response := submit(handler, validInput()); response.Code != http.StatusCreated || pey.calls != 1 {
		t.Fatalf("first: %d calls=%d %s", response.Code, pey.calls, response.Body)
	}
	again := validInput()
	again["name"] = "Otro"
	response := submit(handler, again)
	if response.Code != http.StatusUnprocessableEntity || pey.calls != 1 {
		t.Fatalf("duplicate: %d calls=%d %s", response.Code, pey.calls, response.Body)
	}
}

func TestDuplicateCLABE(t *testing.T) {
	handler := New()
	if response := submit(handler, validInput()); response.Code != http.StatusCreated {
		t.Fatalf("first: %d %s", response.Code, response.Body)
	}
	again := validInput()
	again["name"] = "Otro"
	response := submit(handler, again)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate: %d %s", response.Code, response.Body)
	}
	var result struct {
		Errors map[string]string `json:"errors"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &result)
	if result.Errors[definitions.FieldCLABE] != definitions.MessageCLABETaken {
		t.Fatalf("field error: %s", response.Body)
	}
	other := validInput()
	other["clabe"] = "012180001183597198"
	if response := submit(handler, other); response.Code != http.StatusCreated {
		t.Fatalf("other clabe: %d %s", response.Code, response.Body)
	}
}

func TestMalformedRequests(t *testing.T) {
	for _, body := range []string{`{`, `{"unknown":true}`, `{} {}`, `[]`, `{"client":{"unknown":true}}`, `{"name":42}`} {
		request := httptest.NewRequest(http.MethodPost, "/api/onboarding", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		New().ServeHTTP(response, request)
		if response.Code != 400 {
			t.Errorf("body %s: expected 400, got %d", body, response.Code)
		}
	}
}

func TestHTTPStatusCodes(t *testing.T) {
	for _, test := range []struct {
		method, path, contentType, body string
		status                          int
	}{
		{"GET", "/api/health", "", "", 200},
		{"GET", "/api/contractors", "", "", 200},
		{"GET", "/api/onboarding/missing", "", "", 404},
		{"PUT", "/api/onboarding", "", "", 405},
		{"POST", "/api/onboarding", "text/plain", "{}", 415},
		{"POST", "/api/onboarding", "application/json", strings.Repeat(" ", 1<<20) + "{}", 413},
	} {
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		request.Header.Set("Content-Type", test.contentType)
		response := httptest.NewRecorder()
		New().ServeHTTP(response, request)
		if response.Code != test.status {
			t.Errorf("%s %s: expected %d, got %d", test.method, test.path, test.status, response.Code)
		}
	}
}

func TestCreateAndRetrieveWithSQLite(t *testing.T) {
	dir := t.TempDir()
	vendors, err := database.OpenSQLite(filepath.Join(dir, "vendors.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer vendors.Close()
	clients, err := database.OpenSQLite(filepath.Join(dir, "clients.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()
	repository, err := infrastructure.NewSQLiteRepository(vendors)
	if err != nil {
		t.Fatal(err)
	}
	clientsRepository, err := clientsinfra.NewSQLiteRepository(clients)
	if err != nil {
		t.Fatal(err)
	}
	handler := Mount(repository, infrastructure.NewUnavailablePey(), paymentsinfra.NewMemoryRepository(), paymentsinfra.NewSQLiteVendors(vendors), paymentsinfra.UnavailableCollector{}, paymentsdomain.Links{}, clientsRepository, clientsinfra.NewSQLiteVendors(vendors))
	response := submit(handler, validInput())
	if response.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", response.Code, response.Body)
	}
	location := response.Header().Get("Location")
	found := httptest.NewRecorder()
	handler.ServeHTTP(found, httptest.NewRequest(http.MethodGet, location, nil))
	if found.Code != http.StatusOK || !strings.Contains(found.Body.String(), `"rfc":"AAA010101AAA"`) || !strings.Contains(found.Body.String(), `"email":"luis@example.com"`) {
		t.Fatalf("find: %d %s", found.Code, found.Body)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &created)
	listed := httptest.NewRecorder()
	handler.ServeHTTP(listed, httptest.NewRequest(http.MethodGet, "/api/contractors/"+created.ID+"/clients", nil))
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"entityType":"individual"`) || !strings.Contains(listed.Body.String(), `"+52 555 123 4567"`) {
		t.Fatalf("clients: %d %s", listed.Code, listed.Body)
	}
}

func TestDashboardProfile(t *testing.T) {
	for _, withTax := range []bool{false, true} {
		t.Run(map[bool]string{false: "incomplete onboarding", true: "complete onboarding"}[withTax], func(t *testing.T) {
			handler := New()
			input := validInput()
			if withTax {
				input["taxRegime"] = "General"
			}
			var created struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(submit(handler, input).Body.Bytes(), &created)
			path := strings.Replace(dashboardpresentation.ProfilePath, "{"+dashboardpresentation.PathParamContractorID+"}", created.ID, 1)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("dashboard: %d %s", response.Code, response.Body)
			}
			var dashboard dashboardpresentation.ProfileResponse
			if err := json.Unmarshal(response.Body.Bytes(), &dashboard); err != nil {
				t.Fatal(err)
			}
			wantStatus, wantPending := dashboarddomain.OnboardingStatusIncomplete, 1
			if withTax {
				wantStatus, wantPending = dashboarddomain.OnboardingStatusComplete, 0
			}
			if dashboard.ContractorID != created.ID || dashboard.Name != "Ana" || dashboard.FirstCustomer.Email != "luis@example.com" ||
				dashboard.Business.TaxpayerType != dashboarddomain.TaxpayerTypeCompany ||
				dashboard.Onboarding.Status != wantStatus || len(dashboard.Onboarding.PendingFields) != wantPending ||
				(dashboard.Business.TaxRegime != nil) != withTax {
				t.Fatalf("unexpected dashboard: %s", response.Body)
			}
		})
	}
}

func TestDashboardProfileNotFound(t *testing.T) {
	path := strings.Replace(dashboardpresentation.ProfilePath, "{"+dashboardpresentation.PathParamContractorID+"}", "missing", 1)
	response := httptest.NewRecorder()
	New().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.Code)
	}
}
