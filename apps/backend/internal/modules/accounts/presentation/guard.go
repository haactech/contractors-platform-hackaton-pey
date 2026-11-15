package presentation

import (
	"contractors-platform/backend/internal/modules/accounts/application"
	"contractors-platform/backend/internal/modules/accounts/definitions"
	"contractors-platform/backend/internal/modules/accounts/domain"
	"contractors-platform/backend/internal/platform/access"
	"net/http"
)

// Guard enforces session cookie authentication, email verification policy,
// and contractor 1:1 ownership on HTTP handlers.
type Guard struct {
	sessions *application.Sessions
	policy   domain.Policy
}

// NewGuard creates a Guard backed by sessions and policy.
func NewGuard(sessions *application.Sessions, policy domain.Policy) *Guard {
	return &Guard{sessions: sessions, policy: policy}
}

func (g *Guard) authenticate(writer http.ResponseWriter, request *http.Request) (domain.Account, bool) {
	token := readSessionCookie(request)
	account, found, err := g.sessions.Current(token)
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, errorResponse{Message: definitions.MessageServerFailure})
		return domain.Account{}, false
	}
	if !found {
		writeJSON(writer, http.StatusUnauthorized, errorResponse{
			Message: definitions.MessageUnauthenticated,
			Code:    definitions.ErrorCodeUnauthenticated,
		})
		return domain.Account{}, false
	}
	return account, true
}

func attachPrincipal(request *http.Request, account domain.Account) *http.Request {
	contractorID := ""
	if account.ContractorID != nil {
		contractorID = *account.ContractorID
	}
	ctx := access.WithPrincipal(request.Context(), access.Principal{
		AccountID:     account.ID,
		ContractorID:  contractorID,
		EmailVerified: account.EmailVerified(),
	})
	return request.WithContext(ctx)
}

func (g *Guard) sessionOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		account, ok := g.authenticate(writer, request)
		if !ok {
			return
		}
		next(writer, attachPrincipal(request, account))
	}
}

// Session requires an authenticated session and, if enforced by policy, a verified email.
func (g *Guard) Session(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		account, ok := g.authenticate(writer, request)
		if !ok {
			return
		}
		if g.policy.RequireEmailVerification && !account.EmailVerified() {
			writeJSON(writer, http.StatusForbidden, errorResponse{
				Message: definitions.MessageEmailUnverified,
				Code:    definitions.ErrorCodeEmailUnverified,
			})
			return
		}
		next(writer, attachPrincipal(request, account))
	}
}

// Contractor enforces Session and requires the path parameter param to match the caller's contractorId.
func (g *Guard) Contractor(param string, next http.HandlerFunc) http.HandlerFunc {
	return g.Session(func(writer http.ResponseWriter, request *http.Request) {
		principal, ok := access.PrincipalFrom(request.Context())
		if !ok || principal.ContractorID == "" || request.PathValue(param) != principal.ContractorID {
			writeJSON(writer, http.StatusNotFound, errorResponse{Message: definitions.MessageNotFound})
			return
		}
		next(writer, request)
	})
}
