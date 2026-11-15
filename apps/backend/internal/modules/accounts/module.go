// Package accounts manages user sign-up, login, sessions, email verification,
// PIN recovery, demo account seeding, and access guards.
package accounts

import (
	"contractors-platform/backend/internal/modules/accounts/application"
	"contractors-platform/backend/internal/modules/accounts/domain"
	"contractors-platform/backend/internal/modules/accounts/infrastructure"
	"contractors-platform/backend/internal/modules/accounts/presentation"
	"contractors-platform/backend/internal/platform/access"
	"net/http"
	"time"
)

// Deps configures the accounts module.
type Deps struct {
	Repository domain.Repository
	Hasher     domain.PINHasher
	Mailer     domain.Mailer
	Policy     domain.Policy
	Clock      domain.Clock
}

// Module exposes the public API of the accounts module for app composition.
type Module struct {
	Guard  access.Guard
	Owners *application.Owners
	Demo   *application.Demo
}

// Register mounts all /api/auth/* routes on mux and returns the module public API.
func Register(mux *http.ServeMux, deps Deps) Module {
	clock := deps.Clock
	if clock == nil {
		clock = time.Now
	}
	mailer := deps.Mailer
	if mailer == nil {
		mailer = infrastructure.NewLogMailer()
	}
	hasher := deps.Hasher
	if hasher == nil {
		hasher = infrastructure.NewPBKDF2Hasher(0)
	}

	sessions := application.NewSessions(deps.Repository, clock)
	verification := application.NewEmailVerification(deps.Repository, mailer, deps.Policy, clock)
	signUp := application.NewSignUp(deps.Repository, hasher, sessions, verification, deps.Policy)
	logIn := application.NewLogIn(deps.Repository, hasher, sessions, deps.Policy, clock)
	recovery := application.NewPINRecovery(deps.Repository, hasher, sessions, verification, deps.Policy, clock)
	owners := application.NewOwners(deps.Repository)
	demo := application.NewDemo(deps.Repository, hasher, sessions, deps.Policy, clock)
	guard := presentation.NewGuard(sessions, deps.Policy)

	handler := presentation.New(signUp, logIn, sessions, verification, recovery, demo, guard, deps.Policy, clock)

	mux.HandleFunc("GET /api/auth/config", handler.Config)
	mux.HandleFunc("POST /api/auth/signup", handler.SignUp)
	mux.HandleFunc("POST /api/auth/login", handler.LogIn)
	mux.HandleFunc("POST /api/auth/demo", handler.LogInDemo)
	mux.HandleFunc("POST /api/auth/logout", handler.LogOut)
	mux.HandleFunc("GET /api/auth/session", handler.Session)
	mux.HandleFunc("POST /api/auth/email-verification", handler.SessionOnly(handler.SendEmailVerification))
	mux.HandleFunc("POST /api/auth/email-verification/confirm", handler.SessionOnly(handler.ConfirmEmailVerification))
	mux.HandleFunc("POST /api/auth/pin-recovery", handler.StartPINRecovery)
	mux.HandleFunc("POST /api/auth/pin-recovery/confirm", handler.ConfirmPINRecovery)

	return Module{
		Guard:  guard,
		Owners: owners,
		Demo:   demo,
	}
}

// PolicyFromEnv resolves the authentication policy from environment variables.
func PolicyFromEnv(getenv func(string) string) (domain.Policy, []string) {
	return domain.PolicyFromEnv(getenv)
}

// NewPBKDF2Hasher constructs a PBKDF2-SHA256 PIN hasher.
func NewPBKDF2Hasher(iterations int) domain.PINHasher {
	return infrastructure.NewPBKDF2Hasher(iterations)
}

// NewLogMailer returns a logger-backed Mailer.
func NewLogMailer() domain.Mailer {
	return infrastructure.NewLogMailer()
}

// NewSMTPMailer returns an SMTP Mailer.
func NewSMTPMailer(host, port, username, password, from string) domain.Mailer {
	return infrastructure.NewSMTPMailer(infrastructure.SMTPConfig{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from,
	})
}
