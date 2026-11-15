package presentation

import (
	"contractors-platform/backend/internal/modules/accounts/definitions"
	"net/http"
	"strings"
)

func isSecureRequest(request *http.Request) bool {
	if request.TLS != nil {
		return true
	}
	return strings.EqualFold(request.Header.Get("X-Forwarded-Proto"), "https")
}

func readSessionCookie(request *http.Request) string {
	cookie, err := request.Cookie(definitions.SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func setSessionCookie(writer http.ResponseWriter, request *http.Request, token string) {
	http.SetCookie(writer, &http.Cookie{
		Name:     definitions.SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(definitions.SessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(request),
	})
}

func clearSessionCookie(writer http.ResponseWriter, request *http.Request) {
	http.SetCookie(writer, &http.Cookie{
		Name:     definitions.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(request),
	})
}
