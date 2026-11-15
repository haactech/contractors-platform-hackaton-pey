package presentation

import (
	"contractors-platform/backend/internal/modules/payments/application"
	"contractors-platform/backend/internal/modules/payments/domain"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

// Handler publica el cobro que pide la pantalla.
type Handler struct {
	create *application.CreatePayment
}

// New prepara el manejador con el acto de crear el cobro.
func New(create *application.CreatePayment) *Handler {
	return &Handler{create: create}
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
