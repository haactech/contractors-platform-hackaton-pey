package infrastructure

import "sync"

// MemoryVendors recuerda los vendedores que una prueba dio de alta.
type MemoryVendors struct {
	mutex sync.RWMutex
	ids   map[string]bool
}

// NewMemoryVendors crea la lectura vacía.
func NewMemoryVendors() *MemoryVendors {
	return &MemoryVendors{ids: map[string]bool{}}
}

// Allow marca que ese vendedor existe.
func (vendors *MemoryVendors) Allow(id string) {
	vendors.mutex.Lock()
	defer vendors.mutex.Unlock()
	vendors.ids[id] = true
}

// Exists indica si la prueba dio de alta a ese vendedor.
func (vendors *MemoryVendors) Exists(id string) (bool, error) {
	vendors.mutex.RLock()
	defer vendors.mutex.RUnlock()
	return vendors.ids[id], nil
}
