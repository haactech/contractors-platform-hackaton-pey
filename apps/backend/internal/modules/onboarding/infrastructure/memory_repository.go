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
	return repository.HasOtherCLABE(clabe, "")
}

// HasOtherCLABE indica si otro contratista distinto a excludeID ya guardó esa CLABE.
func (repository *MemoryRepository) HasOtherCLABE(clabe, excludeID string) (bool, error) {
	if clabe == "" {
		return false, nil
	}
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	return containsOtherCLABE(repository.records, clabe, excludeID), nil
}

func containsOtherCLABE(records map[string]domain.User, clabe, excludeID string) bool {
	for id, user := range records {
		if id != excludeID && user.CLABE == clabe {
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
	if stored.CLABE != "" && containsOtherCLABE(repository.records, stored.CLABE, "") {
		return domain.User{}, domain.ErrCLABETaken
	}
	repository.records[stored.ID] = stored
	return cloneUser(stored), nil
}

// Update reemplaza los campos editables de perfil, negocio y cuenta de depósito.
func (repository *MemoryRepository) Update(user domain.User) (domain.User, error) {
	if fieldErrors := domain.AllowVendor(user); len(fieldErrors) > 0 {
		return domain.User{}, errRejected
	}
	stored := cloneUser(user)
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	existing, found := repository.records[stored.ID]
	if !found {
		return domain.User{}, domain.ErrNotFound
	}
	if stored.CLABE != "" && containsOtherCLABE(repository.records, stored.CLABE, stored.ID) {
		return domain.User{}, domain.ErrCLABETaken
	}
	stored.Customer = existing.Customer
	repository.records[stored.ID] = stored
	return cloneUser(stored), nil
}

// RecordBankAccount anota el id y el estado que Pey devolvió para un usuario ya guardado.
func (repository *MemoryRepository) RecordBankAccount(user domain.User) (domain.User, error) {
	if fieldErrors := domain.AllowVendor(user); len(fieldErrors) > 0 {
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

// FindByCLABE busca un usuario por CLABE normalizada.
func (repository *MemoryRepository) FindByCLABE(clabe string) (domain.User, bool) {
	if clabe == "" {
		return domain.User{}, false
	}
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	for _, user := range repository.records {
		if user.CLABE == clabe {
			return cloneUser(user), true
		}
	}
	return domain.User{}, false
}

// Remove borra el alta que no pudo guardar su primer cliente.
func (repository *MemoryRepository) Remove(id string) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	delete(repository.records, id)
	return nil
}
