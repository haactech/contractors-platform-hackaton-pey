package infrastructure

import (
	"context"
	"contractors-platform/backend/internal/modules/accounts/domain"
	"log"
	"strings"
)

// LogMailer logs outgoing verification/recovery emails (with masked recipient) for local development.
type LogMailer struct{}

// NewLogMailer returns a Mailer that writes messages to the application logger.
func NewLogMailer() LogMailer { return LogMailer{} }

// Send logs the masked recipient, subject, and plain-text body.
func (LogMailer) Send(_ context.Context, to, subject, body string) error {
	log.Printf("accounts: correo para %s (%s): %s", domain.MaskEmail(to), subject, strings.ReplaceAll(body, "\n", " "))
	return nil
}
