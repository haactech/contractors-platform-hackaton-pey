package application

import (
	"context"
	"contractors-platform/backend/internal/modules/accounts/definitions"
	"contractors-platform/backend/internal/modules/accounts/domain"
	"errors"
	"log"
)

// SignUp registers a new account with email, phone, and 6-digit PIN and opens a session.
type SignUp struct {
	repo         domain.Repository
	hasher       domain.PINHasher
	sessions     *Sessions
	verification *EmailVerification
	policy       domain.Policy
}

// NewSignUp creates a SignUp use case.
func NewSignUp(
	repo domain.Repository,
	hasher domain.PINHasher,
	sessions *Sessions,
	verification *EmailVerification,
	policy domain.Policy,
) *SignUp {
	return &SignUp{
		repo:         repo,
		hasher:       hasher,
		sessions:     sessions,
		verification: verification,
		policy:       policy,
	}
}

// Create validates input, creates the account, starts a session, and sends a verification code when required.
func (s *SignUp) Create(ctx context.Context, input domain.SignUpInput) (domain.Account, string, map[string]string, error) {
	account, fieldErrors := domain.ValidateSignUp(input)
	if len(fieldErrors) > 0 {
		return domain.Account{}, "", fieldErrors, nil
	}

	dupErrors := map[string]string{}
	if _, found, err := s.repo.FindByEmail(account.Email); err != nil {
		return domain.Account{}, "", nil, err
	} else if found {
		dupErrors[definitions.FieldEmail] = definitions.MessageEmailTaken
	}
	if _, found, err := s.repo.FindByPhone(account.Phone); err != nil {
		return domain.Account{}, "", nil, err
	} else if found {
		dupErrors[definitions.FieldPhone] = definitions.MessagePhoneTaken
	}
	if len(dupErrors) > 0 {
		return domain.Account{}, "", dupErrors, nil
	}

	pinHash, err := s.hasher.Hash(input.PIN)
	if err != nil {
		return domain.Account{}, "", nil, err
	}
	account.PINHash = pinHash

	created, err := s.repo.Create(account)
	if errors.Is(err, domain.ErrEmailTaken) {
		return domain.Account{}, "", map[string]string{definitions.FieldEmail: definitions.MessageEmailTaken}, nil
	}
	if errors.Is(err, domain.ErrPhoneTaken) {
		return domain.Account{}, "", map[string]string{definitions.FieldPhone: definitions.MessagePhoneTaken}, nil
	}
	if err != nil {
		return domain.Account{}, "", nil, err
	}

	token, err := s.sessions.issue(created.ID)
	if err != nil {
		return domain.Account{}, "", nil, err
	}

	if s.policy.RequireEmailVerification {
		if _, err := s.verification.Send(ctx, created.ID); err != nil {
			log.Printf("accounts: send initial verification %s: %v", created.ID, err)
		}
	}

	return created, token, nil, nil
}
