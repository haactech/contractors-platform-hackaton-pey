package presentation

import (
	"contractors-platform/backend/internal/modules/payments/application"
	"contractors-platform/backend/internal/modules/payments/domain"
	"contractors-platform/backend/internal/platform/access"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

// Handler publica el cobro que pide la pantalla y la lectura de cobros guardados.
type Handler struct {
	create *application.CreatePayment
	read   *application.ReadPayments
}

// New prepara el manejador con crear y leer cobros.
func New(create *application.CreatePayment, read *application.ReadPayments) *Handler {
	return &Handler{create: create, read: read}
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

// Create recibe el monto y el cliente, y responde con el cobro dirigido a la cuenta del vendedor.
func (handler *Handler) Create(writer http.ResponseWriter, request *http.Request) {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeJSON(writer, http.StatusUnsupportedMediaType, map[string]string{"message": domain.ContentTypeMessage()})
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input domain.Request
	if err := decoder.Decode(&input); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(writer, http.StatusRequestEntityTooLarge, map[string]string{"message": domain.TooLargeMessage()})
		} else {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"message": domain.JSONMessage()})
		}
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"message": domain.OneObjectMessage()})
		return
	}
	if principal, ok := access.PrincipalFrom(request.Context()); ok {
		if principal.ContractorID == "" || principal.ContractorID != input.VendorID {
			writeJSON(writer, http.StatusNotFound, map[string]string{"message": domain.VendorMissingMessage()})
			return
		}
	}
	view, validation, err := handler.create.Create(request.Context(), input)
	if len(validation) > 0 {
		writeJSON(writer, http.StatusUnprocessableEntity, map[string]any{"message": domain.ReviewMessage(), "errors": validation})
		return
	}
	if errors.Is(err, domain.ErrVendorNotFound) {
		writeJSON(writer, http.StatusNotFound, map[string]string{"message": domain.VendorMissingMessage()})
		return
	}
	if errors.Is(err, domain.ErrCollector) {
		writeJSON(writer, http.StatusBadGateway, map[string]string{"message": domain.PeyMessage()})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"message": domain.SaveMessage()})
		return
	}
	writeJSON(writer, http.StatusCreated, view)
}

// List responde con los cobros del vendedor, más recientes primero.
func (handler *Handler) List(writer http.ResponseWriter, request *http.Request) {
	views, err := handler.read.ListByVendor(request.PathValue("contractorId"))
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"message": domain.SaveMessage()})
		return
	}
	writeJSON(writer, http.StatusOK, views)
}

// Find responde con un cobro de ese vendedor.
func (handler *Handler) Find(writer http.ResponseWriter, request *http.Request) {
	view, found, err := handler.read.Find(request.PathValue("contractorId"), request.PathValue("paymentId"))
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"message": domain.SaveMessage()})
		return
	}
	if !found {
		writeJSON(writer, http.StatusNotFound, map[string]string{"message": domain.PaymentMissingMessage()})
		return
	}
	writeJSON(writer, http.StatusOK, view)
}
