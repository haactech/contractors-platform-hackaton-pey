package application

import (
	"contractors-platform/backend/internal/modules/onboarding/definitions"
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"contractors-platform/backend/internal/modules/onboarding/infrastructure"
	"testing"
)

func TestSeedEnsureAttachesSyntheticActiveBank(t *testing.T) {
	repo := infrastructure.NewMemoryRepository()
	clients := &memoryClients{}
	seed := NewSeedUser(repo, clients)
	input := domain.Input{
		Name: "Ana García López", Profession: "Electricista",
		Client:           domain.Client{Name: "Carlos", Phone: "5512345678", Email: "carlos@empresa.mx"},
		BusinessName:     "Servicios Eléctricos García",
		BusinessActivity: "Electricidad y plomería",
		RFC:              "GALA900101AB1",
		CLABE:            "002180000000000012",
	}
	id, err := seed.Ensure(input)
	if err != nil || id == "" {
		t.Fatalf("ensure: id=%q err=%v", id, err)
	}
	stored, found := repo.Find(id)
	if !found || !domain.BankReady(stored) || stored.BankAccountID != definitions.DemoBankAccountID {
		t.Fatalf("stored bank: found=%v %+v", found, stored)
	}
	if _, found, err := clients.First(id); err != nil || !found {
		t.Fatalf("first client: found=%v err=%v", found, err)
	}

	// Idempotent: second Ensure keeps a single ACTIVE bank.
	id2, err := seed.Ensure(input)
	if err != nil || id2 != id {
		t.Fatalf("second ensure: id=%q err=%v", id2, err)
	}
	stored, _ = repo.Find(id)
	if !domain.BankReady(stored) || stored.BankAccountID != definitions.DemoBankAccountID {
		t.Fatalf("second bank: %+v", stored)
	}
}
