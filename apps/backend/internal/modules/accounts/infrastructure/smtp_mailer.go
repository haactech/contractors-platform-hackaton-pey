package infrastructure

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

// SMTPConfig configures SMTPMailer.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// SMTPMailer sends plain-text Spanish emails via net/smtp (STARTTLS when supported).
type SMTPMailer struct {
	config SMTPConfig
}

// NewSMTPMailer builds an SMTPMailer from cfg.
func NewSMTPMailer(cfg SMTPConfig) SMTPMailer {
	if cfg.Port == "" {
		cfg.Port = "587"
	}
	if cfg.From == "" {
		cfg.From = cfg.Username
	}
	return SMTPMailer{config: cfg}
}

// Send delivers a plain-text UTF-8 email using net/smtp.
func (m SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	addr := net.JoinHostPort(m.config.Host, m.config.Port)
	var auth smtp.Auth
	if m.config.Username != "" {
		auth = smtp.PlainAuth("", m.config.Username, m.config.Password, m.config.Host)
	}
	msg := strings.Join([]string{
		fmt.Sprintf("From: %s", m.config.From),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")
	return smtp.SendMail(addr, auth, m.config.From, []string{to}, []byte(msg))
}
