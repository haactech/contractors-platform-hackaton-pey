package infrastructure

import (
	"contractors-platform/backend/internal/modules/payments/domain"
	"sync"
)

// MemoryVendors conserva la cuenta que las pruebas cargan.
type MemoryVendors struct {
	mutex    sync.RWMutex
	accounts map[string]domain.Account
}

// NewMemoryVendors crea la lectura vacía.
func NewMemoryVendors() *MemoryVendors {
	return &MemoryVendors{accounts: map[string]domain.Account{}}
}

// Set anota la cuenta de un vendedor.
func (source *MemoryVendors) Set(vendorID string, account domain.Account) {
	source.mutex.Lock()
	defer source.mutex.Unlock()
	source.accounts[vendorID] = account
}

// Account lee la cuenta Pey de ese vendedor.
func (source *MemoryVendors) Account(vendorID string) (domain.Account, bool, error) {
	source.mutex.RLock()
	defer source.mutex.RUnlock()
	account, found := source.accounts[vendorID]
	return account, found, nil
}
