package presentation

import (
	"contractors-platform/backend/internal/modules/onboarding/application"
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

type Handler struct {
	register *application.RegisterUser
	update   *application.UpdateUser
	read     *application.ReadUser
}

func New(register *application.RegisterUser, update *application.UpdateUser, read *application.ReadUser) *Handler {
	return &Handler{register: register, update: update, read: read}
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func decodeJSONBody(writer http.ResponseWriter, request *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeJSON(writer, 415, map[string]string{"message": "Usa Content-Type application/json."})
		return false
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(writer, 413, map[string]string{"message": "El cuerpo de la solicitud es demasiado grande."})
		} else {
			writeJSON(writer, 400, map[string]string{"message": "El JSON es inválido o contiene campos desconocidos."})
		}
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeJSON(writer, 400, map[string]string{"message": "Envía un único objeto JSON."})
		return false
	}
	return true
}

func (handler *Handler) Create(writer http.ResponseWriter, request *http.Request) {
	var input domain.Input
	if !decodeJSONBody(writer, request, &input) {
		return
	}
	record, validation, err := handler.register.Create(request.Context(), input)
	if len(validation) > 0 {
		writeJSON(writer, 422, map[string]any{"message": "Revisa los campos del formulario.", "errors": validation})
		return
	}
	if errors.Is(err, domain.ErrAlreadyOnboarded) {
		writeJSON(writer, 409, map[string]string{"message": "Tu cuenta ya tiene un registro."})
		return
	}
	if err != nil {
		writeJSON(writer, 500, map[string]string{"message": "No fue posible guardar el registro."})
		return
	}
	writer.Header().Set("Location", "/api/onboarding/"+record.ID)
	writeJSON(writer, 201, record)
}

func (handler *Handler) Update(writer http.ResponseWriter, request *http.Request) {
	var patch domain.ProfilePatch
	if !decodeJSONBody(writer, request, &patch) {
		return
	}
	record, validation, err := handler.update.Update(request.Context(), request.PathValue("id"), patch)
	if len(validation) > 0 {
		writeJSON(writer, 422, map[string]any{"message": "Revisa los campos del formulario.", "errors": validation})
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		writeJSON(writer, 404, map[string]string{"message": "No se encontró el registro."})
		return
	}
	if err != nil {
		writeJSON(writer, 500, map[string]string{"message": "No fue posible actualizar el registro."})
		return
	}
	writeJSON(writer, 200, record)
}

func (handler *Handler) Find(writer http.ResponseWriter, request *http.Request) {
	record, found := handler.read.Find(request.PathValue("id"))
	if !found {
		writeJSON(writer, 404, map[string]string{"message": "No se encontró el registro."})
		return
	}
	writeJSON(writer, 200, record)
}
