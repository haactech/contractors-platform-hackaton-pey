// Package access carries the authenticated account through a request so that
// modules can enforce ownership without importing the accounts module.
package access

import (
	"context"
	"net/http"
)

// Principal is the authenticated account behind a request.
type Principal struct {
	AccountID string
	// ContractorID is empty until the account finishes onboarding.
	ContractorID  string
	EmailVerified bool
}

type principalKey struct{}

// WithPrincipal stores the authenticated account in the context.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

// PrincipalFrom returns the authenticated account, if any.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}

// Guard wraps handlers with access rules.
type Guard interface {
	// Session requires a valid session (401 otherwise).
	Session(next http.HandlerFunc) http.HandlerFunc
	// Contractor requires a session whose contractor matches the path value
	// named param (404 otherwise, so ids cannot be probed).
	Contractor(param string, next http.HandlerFunc) http.HandlerFunc
}
