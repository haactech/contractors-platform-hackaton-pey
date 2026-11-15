package presentation

import (
	"contractors-platform/backend/internal/modules/clients/application"
	"contractors-platform/backend/internal/modules/clients/domain"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

// Handler publica el directorio de clientes de un vendedor.
type Handler struct {
	service *application.Service
}

// New prepara el manejador con el acto de clientes.
func New(service *application.Service) *Handler {
	return &Handler{service: service}
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func (handler *Handler) List(writer http.ResponseWriter, request *http.Request) {
	clients, err := handler.service.List(request.PathValue("contractorId"))
	if errors.Is(err, domain.ErrVendorNotFound) {
		writeJSON(writer, http.StatusNotFound, map[string]string{"message": domain.VendorMissingMessage()})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"message": domain.SaveMessage()})
		return
	}
	views := make([]domain.View, 0, len(clients))
	for _, client := range clients {
		views = append(views, domain.Present(client))
	}
	writeJSON(writer, http.StatusOK, views)
}

func (handler *Handler) Create(writer http.ResponseWriter, request *http.Request) {
	input, ok := readInput(writer, request)
	if !ok {
		return
	}
	saved, fieldErrors, err := handler.service.Create(request.PathValue("contractorId"), input)
	writeClient(writer, http.StatusCreated, saved, fieldErrors, err)
}

func (handler *Handler) Find(writer http.ResponseWriter, request *http.Request) {
	client, found, err := handler.service.Find(request.PathValue("contractorId"), request.PathValue("clientId"))
	if errors.Is(err, domain.ErrVendorNotFound) {
		writeJSON(writer, http.StatusNotFound, map[string]string{"message": domain.VendorMissingMessage()})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"message": domain.SaveMessage()})
		return
	}
	if !found {
		writeJSON(writer, http.StatusNotFound, map[string]string{"message": domain.ClientMissingMessage()})
		return
	}
	writeJSON(writer, http.StatusOK, domain.Present(client))
}

func (handler *Handler) Update(writer http.ResponseWriter, request *http.Request) {
	input, ok := readInput(writer, request)
	if !ok {
		return
	}
	saved, fieldErrors, err := handler.service.Update(request.PathValue("contractorId"), request.PathValue("clientId"), input)
	writeClient(writer, http.StatusOK, saved, fieldErrors, err)
}

func writeClient(writer http.ResponseWriter, status int, client domain.Client, fieldErrors map[string]string, err error) {
	if len(fieldErrors) > 0 {
		writeJSON(writer, http.StatusUnprocessableEntity, map[string]any{"message": domain.ReviewMessage(), "errors": fieldErrors})
		return
	}
	if errors.Is(err, domain.ErrVendorNotFound) {
		writeJSON(writer, http.StatusNotFound, map[string]string{"message": domain.VendorMissingMessage()})
		return
	}
	if errors.Is(err, domain.ErrClientNotFound) {
		writeJSON(writer, http.StatusNotFound, map[string]string{"message": domain.ClientMissingMessage()})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"message": domain.SaveMessage()})
		return
	}
	writeJSON(writer, status, domain.Present(client))
}

func readInput(writer http.ResponseWriter, request *http.Request) (domain.Input, bool) {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeJSON(writer, http.StatusUnsupportedMediaType, map[string]string{"message": domain.ContentTypeMessage()})
		return domain.Input{}, false
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input domain.Input
	if err := decoder.Decode(&input); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(writer, http.StatusRequestEntityTooLarge, map[string]string{"message": domain.TooLargeMessage()})
		} else {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"message": domain.JSONMessage()})
		}
		return domain.Input{}, false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"message": domain.OneObjectMessage()})
		return domain.Input{}, false
	}
	return input, true
}
