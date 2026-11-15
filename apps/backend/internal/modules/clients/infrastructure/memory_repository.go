package infrastructure

import (
	"contractors-platform/backend/internal/modules/clients/domain"
	"sort"
	"sync"
	"time"
)

// MemoryRepository guarda los clientes solo mientras el proceso vive.
type MemoryRepository struct {
	mutex   sync.Mutex
	records map[string]domain.Client
}

// NewMemoryRepository crea el almacén vacío.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{records: map[string]domain.Client{}}
}

// Save persiste el cliente si el contrato lo permite.
func (repository *MemoryRepository) Save(client domain.Client) (domain.Client, error) {
	if fieldErrors := domain.Allow(client); len(fieldErrors) > 0 {
		return domain.Client{}, errRejected
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if repository.phoneTaken(client.VendorID, client.PhoneKey, "") {
		return domain.Client{}, domain.ErrPhoneTaken
	}
	var err error
	client.ID, err = newID()
	if err != nil {
		return domain.Client{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	client.CreatedAt = now
	client.UpdatedAt = now
	repository.records[client.ID] = client
	return client, nil
}

// Update reemplaza el perfil y conserva el alta original.
func (repository *MemoryRepository) Update(client domain.Client) (domain.Client, error) {
	if fieldErrors := domain.Allow(client); len(fieldErrors) > 0 {
		return domain.Client{}, errRejected
	}
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	current, found := repository.records[client.ID]
	if !found || current.VendorID != client.VendorID {
		return domain.Client{}, domain.ErrClientNotFound
	}
	if repository.phoneTaken(client.VendorID, client.PhoneKey, client.ID) {
		return domain.Client{}, domain.ErrPhoneTaken
	}
	client.CreatedAt = current.CreatedAt
	client.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	repository.records[client.ID] = client
	return client, nil
}

// List devuelve los clientes del vendedor, del más antiguo al más nuevo.
func (repository *MemoryRepository) List(vendorID string) ([]domain.Client, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	var clients []domain.Client
	for _, client := range repository.records {
		if client.VendorID == vendorID {
			clients = append(clients, client)
		}
	}
	sort.Slice(clients, func(i, j int) bool {
		if clients[i].CreatedAt == clients[j].CreatedAt {
			return clients[i].ID < clients[j].ID
		}
		return clients[i].CreatedAt < clients[j].CreatedAt
	})
	return clients, nil
}

// Find busca un cliente de ese vendedor.
func (repository *MemoryRepository) Find(vendorID, clientID string) (domain.Client, bool, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	client, found := repository.records[clientID]
	if !found || client.VendorID != vendorID {
		return domain.Client{}, false, nil
	}
	return client, true, nil
}

func (repository *MemoryRepository) phoneTaken(vendorID, phoneKey, exceptID string) bool {
	if phoneKey == "" {
		return false
	}
	for _, client := range repository.records {
		if client.VendorID == vendorID && client.PhoneKey == phoneKey && client.ID != exceptID {
			return true
		}
	}
	return false
}
