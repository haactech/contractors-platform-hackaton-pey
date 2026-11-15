package presentation

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"contractors-platform/backend/internal/modules/clients/application"
	"contractors-platform/backend/internal/modules/clients/infrastructure"
)

func mount() http.Handler {
	repository := infrastructure.NewMemoryRepository()
	vendors := infrastructure.NewMemoryVendors()
	vendors.Allow("vendor-1")
	vendors.Allow("vendor-2")
	mux := http.NewServeMux()
	handler := New(application.NewService(repository, vendors))
	mux.HandleFunc("GET /api/contractors/{contractorId}/clients", handler.List)
	mux.HandleFunc("POST /api/contractors/{contractorId}/clients", handler.Create)
	mux.HandleFunc("GET /api/contractors/{contractorId}/clients/{clientId}", handler.Find)
	mux.HandleFunc("PATCH /api/contractors/{contractorId}/clients/{clientId}", handler.Update)
	return mux
}

func postClient(handler http.Handler, vendor string, payload any) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	request := httptest.NewRequest(http.MethodPost, "/api/contractors/"+vendor+"/clients", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestCreateDuplicateAndForeignClient(t *testing.T) {
	handler := mount()
	body := map[string]any{"entityType": "individual", "name": "Luis", "phone": "5551234567", "email": "luis@example.com"}
	created := postClient(handler, "vendor-1", body)
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"entityType":"individual"`) {
		t.Fatalf("create: %d %s", created.Code, created.Body)
	}
	var client struct {
		ClientID string `json:"clientId"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &client)
	duplicate := postClient(handler, "vendor-1", map[string]any{"entityType": "individual", "name": "Ana", "phone": "55 5123 4567", "email": "ana@example.com"})
	if duplicate.Code != http.StatusUnprocessableEntity || !strings.Contains(duplicate.Body.String(), "Este teléfono ya está registrado.") {
		t.Fatalf("duplicate: %d %s", duplicate.Code, duplicate.Body)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/contractors/vendor-2/clients/"+client.ClientID, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "No se encontró el cliente.") {
		t.Fatalf("foreign: %d %s", response.Code, response.Body)
	}
	unknown := postClient(handler, "vendor-1", map[string]any{"entityType": "individual", "name": "Luis", "phone": "5551234567", "email": "luis@example.com", "bankAccountId": "spoofed"})
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown field: %d %s", unknown.Code, unknown.Body)
	}
}
