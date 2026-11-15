package infrastructure

import (
	"contractors-platform/backend/internal/modules/accounts/domain"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var errNotFound = errors.New("cuenta o código no encontrado")

// MemoryRepository stores accounts, sessions, and verification codes in memory.
type MemoryRepository struct {
	mutex    sync.RWMutex
	accounts map[string]domain.Account
	sessions map[string]domain.Session
	codes    map[string]domain.Code
}

// NewMemoryRepository returns an empty thread-safe in-memory repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		accounts: make(map[string]domain.Account),
		sessions: make(map[string]domain.Session),
		codes:    make(map[string]domain.Code),
	}
}

func randomID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func (r *MemoryRepository) Create(account domain.Account) (domain.Account, error) {
	id, err := randomID()
	if err != nil {
		return domain.Account{}, err
	}
	account.ID = id
	stored := domain.CloneAccount(account)

	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, existing := range r.accounts {
		if existing.Email == stored.Email {
			return domain.Account{}, domain.ErrEmailTaken
		}
		if existing.Phone == stored.Phone {
			return domain.Account{}, domain.ErrPhoneTaken
		}
		if stored.ContractorID != nil && existing.ContractorID != nil && *existing.ContractorID == *stored.ContractorID {
			return domain.Account{}, domain.ErrAlreadyLinked
		}
	}
	r.accounts[stored.ID] = stored
	return domain.CloneAccount(stored), nil
}

func (r *MemoryRepository) Find(id string) (domain.Account, bool, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	account, ok := r.accounts[id]
	if !ok {
		return domain.Account{}, false, nil
	}
	return domain.CloneAccount(account), true, nil
}

func (r *MemoryRepository) FindByEmail(email string) (domain.Account, bool, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for _, account := range r.accounts {
		if account.Email == email {
			return domain.CloneAccount(account), true, nil
		}
	}
	return domain.Account{}, false, nil
}

func (r *MemoryRepository) FindByPhone(phone string) (domain.Account, bool, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for _, account := range r.accounts {
		if account.Phone == phone {
			return domain.CloneAccount(account), true, nil
		}
	}
	return domain.Account{}, false, nil
}

func (r *MemoryRepository) LinkContractor(accountID, contractorID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	account, ok := r.accounts[accountID]
	if !ok {
		return errNotFound
	}
	if account.ContractorID != nil {
		return domain.ErrAlreadyLinked
	}
	for _, existing := range r.accounts {
		if existing.ContractorID != nil && *existing.ContractorID == contractorID {
			return domain.ErrAlreadyLinked
		}
	}
	account.ContractorID = domain.CopyText(&contractorID)
	r.accounts[accountID] = account
	return nil
}

func (r *MemoryRepository) UpdateSecurity(account domain.Account) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	stored, ok := r.accounts[account.ID]
	if !ok {
		return errNotFound
	}
	stored.PINHash = account.PINHash
	stored.FailedLogins = account.FailedLogins
	stored.LockedUntil = domain.CopyTime(account.LockedUntil)
	stored.EmailVerifiedAt = domain.CopyTime(account.EmailVerifiedAt)
	r.accounts[account.ID] = stored
	return nil
}

func (r *MemoryRepository) SaveSession(session domain.Session, now time.Time) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for tokenHash, existing := range r.sessions {
		if !now.Before(existing.ExpiresAt) {
			delete(r.sessions, tokenHash)
		}
	}
	r.sessions[session.TokenHash] = session
	return nil
}

func (r *MemoryRepository) FindSession(tokenHash string, now time.Time) (domain.Session, bool, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	session, ok := r.sessions[tokenHash]
	if !ok || !now.Before(session.ExpiresAt) {
		return domain.Session{}, false, nil
	}
	return session, true, nil
}

func (r *MemoryRepository) DeleteSession(tokenHash string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.sessions, tokenHash)
	return nil
}

func (r *MemoryRepository) DeleteSessionsFor(accountID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for tokenHash, session := range r.sessions {
		if session.AccountID == accountID {
			delete(r.sessions, tokenHash)
		}
	}
	return nil
}

func (r *MemoryRepository) SaveCode(code domain.Code) error {
	id, err := randomID()
	if err != nil {
		return err
	}
	code.ID = id
	stored := domain.CloneCode(code)

	r.mutex.Lock()
	defer r.mutex.Unlock()
	now := stored.CreatedAt
	for existingID, existing := range r.codes {
		if !now.Before(existing.ExpiresAt) {
			delete(r.codes, existingID)
			continue
		}
		if existing.AccountID == stored.AccountID && existing.Purpose == stored.Purpose && existing.ConsumedAt == nil {
			existing.ConsumedAt = domain.CopyTime(&now)
			r.codes[existingID] = existing
		}
	}
	r.codes[stored.ID] = stored
	return nil
}

func (r *MemoryRepository) LatestCode(accountID, purpose string) (domain.Code, bool, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	var latest domain.Code
	found := false
	for _, code := range r.codes {
		if code.AccountID != accountID || code.Purpose != purpose {
			continue
		}
		if !found || code.CreatedAt.After(latest.CreatedAt) {
			latest = domain.CloneCode(code)
			found = true
		}
	}
	return latest, found, nil
}

func (r *MemoryRepository) UpdateCode(code domain.Code) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if _, ok := r.codes[code.ID]; !ok {
		return errNotFound
	}
	r.codes[code.ID] = domain.CloneCode(code)
	return nil
}
