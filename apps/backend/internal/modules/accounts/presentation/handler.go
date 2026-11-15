package presentation

import (
	"contractors-platform/backend/internal/modules/accounts/application"
	"contractors-platform/backend/internal/modules/accounts/definitions"
	"contractors-platform/backend/internal/modules/accounts/domain"
	"contractors-platform/backend/internal/platform/access"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"strconv"
)

type errorResponse struct {
	Message string            `json:"message"`
	Code    string            `json:"code,omitempty"`
	Errors  map[string]string `json:"errors,omitempty"`
}

type accountDTO struct {
	ID            string  `json:"id"`
	Email         string  `json:"email"`
	Phone         string  `json:"phone"`
	ContractorID  *string `json:"contractorId"`
	EmailVerified bool    `json:"emailVerified"`
	Demo          bool    `json:"demo"`
}

type sessionResponse struct {
	Account accountDTO `json:"account"`
}

type demoAccountInfo struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
	PIN   string `json:"pin"`
}

type configResponse struct {
	EmailVerificationRequired bool             `json:"emailVerificationRequired"`
	PinResetRequiresCode      bool             `json:"pinResetRequiresCode"`
	DemoAccount               *demoAccountInfo `json:"demoAccount"`
}

type codeIssuedResponse struct {
	DevCode string `json:"devCode,omitempty"`
}

type confirmEmailInput struct {
	Code string `json:"code"`
}

// Handler serves HTTP endpoints for the accounts module.
type Handler struct {
	signUp       *application.SignUp
	logIn        *application.LogIn
	sessions     *application.Sessions
	verification *application.EmailVerification
	recovery     *application.PINRecovery
	demo         *application.Demo
	guard        *Guard
	policy       domain.Policy
	clock        domain.Clock
}

// New constructs a Handler for all /api/auth/* routes.
func New(
	signUp *application.SignUp,
	logIn *application.LogIn,
	sessions *application.Sessions,
	verification *application.EmailVerification,
	recovery *application.PINRecovery,
	demo *application.Demo,
	guard *Guard,
	policy domain.Policy,
	clock domain.Clock,
) *Handler {
	return &Handler{
		signUp:       signUp,
		logIn:        logIn,
		sessions:     sessions,
		verification: verification,
		recovery:     recovery,
		demo:         demo,
		guard:        guard,
		policy:       policy,
		clock:        clock,
	}
}

func toAccountDTO(account domain.Account) accountDTO {
	return accountDTO{
		ID:            account.ID,
		Email:         account.Email,
		Phone:         account.Phone,
		ContractorID:  domain.CopyText(account.ContractorID),
		EmailVerified: account.EmailVerified(),
		Demo:          account.IsDemo,
	}
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeJSON(writer, http.StatusUnsupportedMediaType, errorResponse{Message: definitions.MessageContentType})
		return false
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(writer, http.StatusRequestEntityTooLarge, errorResponse{Message: definitions.MessageBodyTooLarge})
		} else {
			writeJSON(writer, http.StatusBadRequest, errorResponse{Message: definitions.MessageInvalidJSON})
		}
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeJSON(writer, http.StatusBadRequest, errorResponse{Message: definitions.MessageSingleJSON})
		return false
	}
	return true
}

func (h *Handler) Config(writer http.ResponseWriter, _ *http.Request) {
	var demoInfo *demoAccountInfo
	if h.policy.DemoAccount {
		demoInfo = &demoAccountInfo{
			Email: definitions.DemoEmail,
			Phone: definitions.DemoPhoneDisplay,
			PIN:   definitions.DemoPIN,
		}
	}
	writeJSON(writer, http.StatusOK, configResponse{
		EmailVerificationRequired: h.policy.RequireEmailVerification,
		PinResetRequiresCode:      h.policy.PINResetRequiresCode,
		DemoAccount:               demoInfo,
	})
}

func (h *Handler) SignUp(writer http.ResponseWriter, request *http.Request) {
	var input domain.SignUpInput
	if !decodeJSON(writer, request, &input) {
		return
	}
	account, token, validation, err := h.signUp.Create(request.Context(), input)
	if len(validation) > 0 {
		writeJSON(writer, http.StatusUnprocessableEntity, errorResponse{
			Message: definitions.MessageValidation,
			Errors:  validation,
		})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, errorResponse{Message: definitions.MessageServerFailure})
		return
	}
	setSessionCookie(writer, request, token)
	writeJSON(writer, http.StatusCreated, sessionResponse{Account: toAccountDTO(account)})
}

func (h *Handler) LogIn(writer http.ResponseWriter, request *http.Request) {
	var input domain.LogInInput
	if !decodeJSON(writer, request, &input) {
		return
	}
	account, token, validation, err := h.logIn.Open(input)
	if len(validation) > 0 {
		writeJSON(writer, http.StatusUnprocessableEntity, errorResponse{
			Message: definitions.MessageValidation,
			Errors:  validation,
		})
		return
	}
	var locked domain.ErrLocked
	if errors.As(err, &locked) {
		seconds := int(math.Ceil(locked.Until.Sub(h.clock().UTC()).Seconds()))
		if seconds < 1 {
			seconds = 1
		}
		minutes := (seconds + 59) / 60
		writer.Header().Set("Retry-After", strconv.Itoa(seconds))
		writeJSON(writer, http.StatusTooManyRequests, errorResponse{
			Message: fmt.Sprintf("Demasiados intentos. Intenta de nuevo en %d min.", minutes),
			Code:    definitions.ErrorCodeAccountLocked,
		})
		return
	}
	if errors.Is(err, domain.ErrInvalidCredential) {
		writeJSON(writer, http.StatusUnauthorized, errorResponse{Message: definitions.MessageInvalidCredential})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, errorResponse{Message: definitions.MessageServerFailure})
		return
	}
	setSessionCookie(writer, request, token)
	writeJSON(writer, http.StatusOK, sessionResponse{Account: toAccountDTO(account)})
}

func (h *Handler) LogInDemo(writer http.ResponseWriter, request *http.Request) {
	account, token, err := h.demo.LogInDemo(request.Context())
	if errors.Is(err, application.ErrDemoDisabled) {
		writeJSON(writer, http.StatusNotFound, errorResponse{Message: definitions.MessageNotFound})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, errorResponse{Message: definitions.MessageServerFailure})
		return
	}
	setSessionCookie(writer, request, token)
	writeJSON(writer, http.StatusOK, sessionResponse{Account: toAccountDTO(account)})
}

func (h *Handler) LogOut(writer http.ResponseWriter, request *http.Request) {
	if token := readSessionCookie(request); token != "" {
		if err := h.sessions.Close(token); err != nil {
			writeJSON(writer, http.StatusInternalServerError, errorResponse{Message: definitions.MessageServerFailure})
			return
		}
	}
	clearSessionCookie(writer, request)
	writer.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Session(writer http.ResponseWriter, request *http.Request) {
	account, ok := h.guard.authenticate(writer, request)
	if !ok {
		return
	}
	writeJSON(writer, http.StatusOK, sessionResponse{Account: toAccountDTO(account)})
}

func (h *Handler) SendEmailVerification(writer http.ResponseWriter, request *http.Request) {
	principal, ok := access.PrincipalFrom(request.Context())
	if !ok {
		writeJSON(writer, http.StatusUnauthorized, errorResponse{
			Message: definitions.MessageUnauthenticated,
			Code:    definitions.ErrorCodeUnauthenticated,
		})
		return
	}
	devCode, err := h.verification.Send(request.Context(), principal.AccountID)
	var tooSoon domain.ErrResendTooSoon
	if errors.As(err, &tooSoon) {
		seconds := int(math.Ceil(tooSoon.RetryAfter.Seconds()))
		if seconds < 1 {
			seconds = 1
		}
		writer.Header().Set("Retry-After", strconv.Itoa(seconds))
		writeJSON(writer, http.StatusTooManyRequests, errorResponse{Message: definitions.MessageResendTooSoon})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, errorResponse{Message: definitions.MessageServerFailure})
		return
	}
	writeJSON(writer, http.StatusAccepted, codeIssuedResponse{DevCode: devCode})
}

func (h *Handler) ConfirmEmailVerification(writer http.ResponseWriter, request *http.Request) {
	principal, ok := access.PrincipalFrom(request.Context())
	if !ok {
		writeJSON(writer, http.StatusUnauthorized, errorResponse{
			Message: definitions.MessageUnauthenticated,
			Code:    definitions.ErrorCodeUnauthenticated,
		})
		return
	}
	var input confirmEmailInput
	if !decodeJSON(writer, request, &input) {
		return
	}
	account, err := h.verification.Confirm(request.Context(), principal.AccountID, input.Code)
	if errors.Is(err, domain.ErrInvalidCode) {
		writeJSON(writer, http.StatusUnprocessableEntity, errorResponse{
			Message: definitions.MessageInvalidCode,
			Errors:  map[string]string{definitions.FieldCode: definitions.MessageInvalidCode},
		})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, errorResponse{Message: definitions.MessageServerFailure})
		return
	}
	writeJSON(writer, http.StatusOK, sessionResponse{Account: toAccountDTO(account)})
}

func (h *Handler) StartPINRecovery(writer http.ResponseWriter, request *http.Request) {
	var input application.PINRecoveryStartInput
	if !decodeJSON(writer, request, &input) {
		return
	}
	devCode, validation, err := h.recovery.Start(request.Context(), input)
	if len(validation) > 0 {
		writeJSON(writer, http.StatusUnprocessableEntity, errorResponse{
			Message: definitions.MessageValidation,
			Errors:  validation,
		})
		return
	}
	var tooSoon domain.ErrResendTooSoon
	if errors.As(err, &tooSoon) {
		seconds := int(math.Ceil(tooSoon.RetryAfter.Seconds()))
		if seconds < 1 {
			seconds = 1
		}
		writer.Header().Set("Retry-After", strconv.Itoa(seconds))
		writeJSON(writer, http.StatusTooManyRequests, errorResponse{Message: definitions.MessageResendTooSoon})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, errorResponse{Message: definitions.MessageServerFailure})
		return
	}
	writeJSON(writer, http.StatusAccepted, codeIssuedResponse{DevCode: devCode})
}

func (h *Handler) ConfirmPINRecovery(writer http.ResponseWriter, request *http.Request) {
	var input application.PINRecoveryConfirmInput
	if !decodeJSON(writer, request, &input) {
		return
	}
	account, token, validation, err := h.recovery.Confirm(request.Context(), input)
	if len(validation) > 0 {
		writeJSON(writer, http.StatusUnprocessableEntity, errorResponse{
			Message: definitions.MessageValidation,
			Errors:  validation,
		})
		return
	}
	if errors.Is(err, domain.ErrDemoProtected) {
		writeJSON(writer, http.StatusForbidden, errorResponse{
			Message: definitions.MessageDemoProtected,
			Code:    definitions.ErrorCodeDemoProtected,
		})
		return
	}
	if errors.Is(err, domain.ErrInvalidCode) {
		field := definitions.FieldIdentifier
		if h.policy.PINResetRequiresCode {
			field = definitions.FieldCode
		}
		writeJSON(writer, http.StatusUnprocessableEntity, errorResponse{
			Message: definitions.MessageInvalidCode,
			Errors:  map[string]string{field: definitions.MessageInvalidCode},
		})
		return
	}
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, errorResponse{Message: definitions.MessageServerFailure})
		return
	}
	setSessionCookie(writer, request, token)
	writeJSON(writer, http.StatusOK, sessionResponse{Account: toAccountDTO(account)})
}

// SessionOnly exposes the unverified-session wrapper for email verification routes.
func (h *Handler) SessionOnly(next http.HandlerFunc) http.HandlerFunc {
	return h.guard.sessionOnly(next)
}
