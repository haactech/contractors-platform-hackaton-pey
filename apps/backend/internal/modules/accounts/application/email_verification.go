package application

import (
	"context"
	"contractors-platform/backend/internal/modules/accounts/definitions"
	"contractors-platform/backend/internal/modules/accounts/domain"
	"fmt"
	"time"
)

// EmailVerification issues and validates 6-digit email verification codes.
type EmailVerification struct {
	repo   domain.Repository
	mailer domain.Mailer
	policy domain.Policy
	clock  domain.Clock
}

// NewEmailVerification creates an EmailVerification service.
func NewEmailVerification(repo domain.Repository, mailer domain.Mailer, policy domain.Policy, clock domain.Clock) *EmailVerification {
	return &EmailVerification{repo: repo, mailer: mailer, policy: policy, clock: clock}
}

func (v *EmailVerification) issueCode(ctx context.Context, account domain.Account, purpose, subject, bodyTemplate string) (string, error) {
	now := v.clock().UTC()
	if latest, found, err := v.repo.LatestCode(account.ID, purpose); err != nil {
		return "", err
	} else if found {
		elapsed := now.Sub(latest.CreatedAt)
		if elapsed < definitions.ResendCooldown {
			return "", domain.ErrResendTooSoon{RetryAfter: definitions.ResendCooldown - elapsed}
		}
	}

	plain, salt, hash, err := domain.NewVerificationCode()
	if err != nil {
		return "", err
	}
	err = v.repo.SaveCode(domain.Code{
		AccountID: account.ID,
		Purpose:   purpose,
		CodeHash:  hash,
		Salt:      salt,
		ExpiresAt: now.Add(definitions.CodeTTL),
		CreatedAt: now,
	})
	if err != nil {
		return "", err
	}

	body := fmt.Sprintf(bodyTemplate, plain)
	if err := v.mailer.Send(ctx, account.Email, subject, body); err != nil {
		return "", err
	}
	if v.policy.ExposeCodes {
		return plain, nil
	}
	return "", nil
}

// Send dispatches a fresh email verification code to the account's email address.
func (v *EmailVerification) Send(ctx context.Context, accountID string) (string, error) {
	account, found, err := v.repo.Find(accountID)
	if err != nil || !found {
		return "", err
	}
	if account.EmailVerified() {
		return "", nil
	}
	return v.issueCode(
		ctx,
		account,
		definitions.PurposeEmailVerification,
		"Tu código de verificación en Praxis",
		"Tu código de verificación es %s. Vence en 15 minutos.",
	)
}

// Confirm verifies the submitted 6-digit code and marks the account email as verified.
func (v *EmailVerification) Confirm(ctx context.Context, accountID, rawCode string) (domain.Account, error) {
	if !domain.ValidateCodeFormat(rawCode) {
		return domain.Account{}, domain.ErrInvalidCode
	}
	account, found, err := v.repo.Find(accountID)
	if err != nil {
		return domain.Account{}, err
	}
	if !found {
		return domain.Account{}, domain.ErrInvalidCode
	}
	if account.EmailVerified() {
		return account, nil
	}
	if err := consumeCode(v.repo, account.ID, definitions.PurposeEmailVerification, rawCode, v.clock().UTC()); err != nil {
		return domain.Account{}, err
	}
	now := v.clock().UTC()
	account.EmailVerifiedAt = &now
	if err := v.repo.UpdateSecurity(account); err != nil {
		return domain.Account{}, err
	}
	return account, nil
}

func consumeCode(repo domain.Repository, accountID, purpose, candidate string, now time.Time) error {
	code, found, err := repo.LatestCode(accountID, purpose)
	if err != nil {
		return err
	}
	if !found || code.ConsumedAt != nil || !now.Before(code.ExpiresAt) || code.Attempts >= definitions.MaxCodeAttempts {
		return domain.ErrInvalidCode
	}
	if !domain.VerifyCode(code, candidate, now) {
		code.Attempts++
		if code.Attempts >= definitions.MaxCodeAttempts {
			code.ConsumedAt = &now
		}
		if updateErr := repo.UpdateCode(code); updateErr != nil {
			return updateErr
		}
		return domain.ErrInvalidCode
	}
	code.ConsumedAt = &now
	return repo.UpdateCode(code)
}
