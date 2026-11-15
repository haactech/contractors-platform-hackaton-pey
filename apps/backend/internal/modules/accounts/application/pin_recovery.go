package application

import (
	"context"
	"contractors-platform/backend/internal/modules/accounts/definitions"
	"contractors-platform/backend/internal/modules/accounts/domain"
	"strings"
)

// PINRecoveryStartInput represents POST /api/auth/pin-recovery.
type PINRecoveryStartInput struct {
	Identifier string `json:"identifier"`
}

// PINRecoveryConfirmInput represents POST /api/auth/pin-recovery/confirm.
type PINRecoveryConfirmInput struct {
	Identifier string `json:"identifier"`
	Code       string `json:"code,omitempty"`
	PIN        string `json:"pin"`
}

// PINRecovery handles forgotten-PIN reset requests and confirmation.
type PINRecovery struct {
	repo         domain.Repository
	hasher       domain.PINHasher
	sessions     *Sessions
	verification *EmailVerification
	policy       domain.Policy
	clock        domain.Clock
}

// NewPINRecovery creates a PINRecovery service.
func NewPINRecovery(
	repo domain.Repository,
	hasher domain.PINHasher,
	sessions *Sessions,
	verification *EmailVerification,
	policy domain.Policy,
	clock domain.Clock,
) *PINRecovery {
	return &PINRecovery{
		repo:         repo,
		hasher:       hasher,
		sessions:     sessions,
		verification: verification,
		policy:       policy,
		clock:        clock,
	}
}

func (r *PINRecovery) findByIdentifier(raw string) (domain.Account, bool, error) {
	kind, normalized, ok := domain.ParseIdentifier(raw)
	if !ok {
		return domain.Account{}, false, nil
	}
	switch kind {
	case domain.IdentifierEmail:
		return r.repo.FindByEmail(normalized)
	case domain.IdentifierPhone:
		return r.repo.FindByPhone(normalized)
	default:
		return domain.Account{}, false, nil
	}
}

// Start sends a recovery code to the account's email when found (and never reveals existence).
func (r *PINRecovery) Start(ctx context.Context, input PINRecoveryStartInput) (string, map[string]string, error) {
	if strings.TrimSpace(input.Identifier) == "" {
		return "", map[string]string{definitions.FieldIdentifier: definitions.MessageRequired}, nil
	}
	if _, _, ok := domain.ParseIdentifier(input.Identifier); !ok {
		return "", map[string]string{definitions.FieldIdentifier: definitions.MessageIdentifier}, nil
	}
	if !r.policy.PINResetRequiresCode {
		return "", nil, nil
	}
	account, found, err := r.findByIdentifier(input.Identifier)
	if err != nil {
		return "", nil, err
	}
	if !found || account.IsDemo {
		return "", nil, nil
	}
	devCode, err := r.verification.issueCode(
		ctx,
		account,
		definitions.PurposePINRecovery,
		"Recuperación de PIN en Praxis",
		"Tu código para crear un PIN nuevo en Praxis es %s. Vence en 15 minutos.",
	)
	if err != nil {
		return "", nil, err
	}
	return devCode, nil, nil
}

// Confirm validates the new PIN (and recovery code in strict mode), updates the PIN,
// revokes previous sessions, and opens a fresh session.
func (r *PINRecovery) Confirm(ctx context.Context, input PINRecoveryConfirmInput) (domain.Account, string, map[string]string, error) {
	fieldErrors := map[string]string{}
	if strings.TrimSpace(input.Identifier) == "" {
		fieldErrors[definitions.FieldIdentifier] = definitions.MessageRequired
	} else if _, _, ok := domain.ParseIdentifier(input.Identifier); !ok {
		fieldErrors[definitions.FieldIdentifier] = definitions.MessageIdentifier
	}
	if r.policy.PINResetRequiresCode {
		if strings.TrimSpace(input.Code) == "" {
			fieldErrors[definitions.FieldCode] = definitions.MessageRequired
		} else if !domain.ValidateCodeFormat(input.Code) {
			fieldErrors[definitions.FieldCode] = definitions.MessageCodeFormat
		}
	}
	if input.PIN == "" {
		fieldErrors[definitions.FieldPIN] = definitions.MessageRequired
	} else if !domain.ValidatePIN(input.PIN) {
		fieldErrors[definitions.FieldPIN] = definitions.MessagePIN
	}
	if len(fieldErrors) > 0 {
		return domain.Account{}, "", fieldErrors, nil
	}

	account, found, err := r.findByIdentifier(input.Identifier)
	if err != nil {
		return domain.Account{}, "", nil, err
	}
	if !found {
		return domain.Account{}, "", nil, domain.ErrInvalidCode
	}
	if account.IsDemo {
		return domain.Account{}, "", nil, domain.ErrDemoProtected
	}

	now := r.clock().UTC()
	if r.policy.PINResetRequiresCode {
		if err := consumeCode(r.repo, account.ID, definitions.PurposePINRecovery, input.Code, now); err != nil {
			return domain.Account{}, "", nil, err
		}
		if account.EmailVerifiedAt == nil {
			account.EmailVerifiedAt = &now
		}
	}

	pinHash, err := r.hasher.Hash(input.PIN)
	if err != nil {
		return domain.Account{}, "", nil, err
	}
	account.PINHash = pinHash
	account = domain.ClearFailures(account)
	if err := r.repo.UpdateSecurity(account); err != nil {
		return domain.Account{}, "", nil, err
	}
	if err := r.repo.DeleteSessionsFor(account.ID); err != nil {
		return domain.Account{}, "", nil, err
	}
	token, err := r.sessions.issue(account.ID)
	if err != nil {
		return domain.Account{}, "", nil, err
	}
	return account, token, nil, nil
}
