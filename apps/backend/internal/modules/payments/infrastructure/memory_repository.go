package infrastructure

import (
	"contractors-platform/backend/internal/modules/payments/domain"
	"sync"
	"time"
)

// MemoryRepository guarda los cobros solo mientras el proceso vive.
type MemoryRepository struct {
	mutex    sync.Mutex
	payments []domain.Payment
}

// NewMemoryRepository crea el almacén vacío.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{}
}

// Allocate reserva el id del cobro antes de llamar a Pey.
func (repository *MemoryRepository) Allocate() (string, error) {
	return newID()
}

// Save persiste el cobro si el contrato lo permite.
func (repository *MemoryRepository) Save(payment domain.Payment) (domain.Payment, error) {
	if fieldErrors := domain.Allow(payment); len(fieldErrors) > 0 {
		return domain.Payment{}, errRejected
	}
	if payment.CreatedAt == "" {
		payment.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.payments = append(repository.payments, payment)
	return payment, nil
}

// Find lee un cobro por id.
func (repository *MemoryRepository) Find(id string) (domain.Payment, bool, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	for _, payment := range repository.payments {
		if payment.ID == id {
			return payment, true, nil
		}
	}
	return domain.Payment{}, false, nil
}

// ListByVendor lista los cobros de un vendedor, más recientes primero.
func (repository *MemoryRepository) ListByVendor(vendorID string) ([]domain.Payment, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	var listed []domain.Payment
	for i := len(repository.payments) - 1; i >= 0; i-- {
		if repository.payments[i].VendorID == vendorID {
			listed = append(listed, repository.payments[i])
		}
	}
	if listed == nil {
		listed = []domain.Payment{}
	}
	return listed, nil
}

// Stored devuelve los cobros guardados, para las pruebas.
func (repository *MemoryRepository) Stored() []domain.Payment {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	copied := make([]domain.Payment, len(repository.payments))
	copy(copied, repository.payments)
	return copied
}
