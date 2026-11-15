package application

import (
	"contractors-platform/backend/internal/modules/accounts/definitions"
	"contractors-platform/backend/internal/modules/accounts/domain"
	"crypto/rand"
	"encoding/base64"
)

// Sessions issues, resolves, and revokes authenticated sessions.
type Sessions struct {
	repo  domain.Repository
	clock domain.Clock
}

// NewSessions creates a Sessions service.
func NewSessions(repo domain.Repository, clock domain.Clock) *Sessions {
	return &Sessions{repo: repo, clock: clock}
}

func (s *Sessions) issue(accountID string) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	now := s.clock().UTC()
	err := s.repo.SaveSession(domain.Session{
		TokenHash: domain.HashToken(token),
		AccountID: accountID,
		ExpiresAt: now.Add(definitions.SessionTTL),
	}, now)
	if err != nil {
		return "", err
	}
	return token, nil
}

// Current resolves a raw session cookie token to its Account, if active and unexpired.
func (s *Sessions) Current(rawToken string) (domain.Account, bool, error) {
	if rawToken == "" {
		return domain.Account{}, false, nil
	}
	session, found, err := s.repo.FindSession(domain.HashToken(rawToken), s.clock().UTC())
	if err != nil || !found {
		return domain.Account{}, false, err
	}
	return s.repo.Find(session.AccountID)
}

// Close revokes the session associated with rawToken.
func (s *Sessions) Close(rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.repo.DeleteSession(domain.HashToken(rawToken))
}
