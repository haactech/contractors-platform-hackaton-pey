package infrastructure

import (
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
)

var errRejected = errors.New("el usuario no cumple el contrato")

// MemoryRepository guarda los usuarios solo mientras el proceso vive.
type MemoryRepository struct {
	mutex   sync.RWMutex
	records map[string]domain.User
}

// NewMemoryRepository crea el almacén vacío.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{records: make(map[string]domain.User)}
}

func cloneUser(user domain.User) domain.User {
	user.TaxRegime = domain.CopyText(user.TaxRegime)
	return user
}

// HasCLABE indica si otro usuario ya guardó esa CLABE.
func (repository *MemoryRepository) HasCLABE(clabe string) (bool, error) {
	if clabe == "" {
		return false, nil
	}
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	return containsCLABE(repository.records, clabe), nil
}

func containsCLABE(records map[string]domain.User, clabe string) bool {
	for _, user := range records {
		if user.CLABE == clabe {
			return true
		}
	}
	return false
}

// Save persiste el usuario si el contrato lo permite y le asigna un id.
func (repository *MemoryRepository) Save(user domain.User) (domain.User, error) {
	if fieldErrors := domain.Allow(user); len(fieldErrors) > 0 {
		return domain.User{}, errRejected
	}
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return domain.User{}, err
	}
	user.ID = hex.EncodeToString(bytes[:])
	stored := cloneUser(user)
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if stored.CLABE != "" && containsCLABE(repository.records, stored.CLABE) {
		return domain.User{}, domain.ErrCLABETaken
	}
	repository.records[stored.ID] = stored
	return cloneUser(stored), nil
}

// RecordBankAccount anota el id y el estado que Pey devolvió para un usuario ya guardado.
func (repository *MemoryRepository) RecordBankAccount(user domain.User) (domain.User, error) {
	if fieldErrors := domain.Allow(user); len(fieldErrors) > 0 {
		return domain.User{}, errRejected
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, found := repository.records[user.ID]
	if !found {
		return domain.User{}, errRejected
	}
	stored.BankAccountID = user.BankAccountID
	stored.CodiStatus = user.CodiStatus
	repository.records[user.ID] = stored
	return cloneUser(stored), nil
}

// Find busca un usuario por id.
func (repository *MemoryRepository) Find(id string) (domain.User, bool) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	user, found := repository.records[id]
	return cloneUser(user), found
}

// Remove borra el alta que no pudo guardar su primer cliente.
func (repository *MemoryRepository) Remove(id string) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	delete(repository.records, id)
	return nil
}
