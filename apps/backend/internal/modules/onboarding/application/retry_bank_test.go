package application

import (
	"context"
	"contractors-platform/backend/internal/modules/onboarding/definitions"
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"contractors-platform/backend/internal/modules/onboarding/infrastructure"
	"testing"
)

func TestRetryBankRegistersWhenAccountMissing(t *testing.T) {
	repo := infrastructure.NewMemoryRepository()
	saved, err := repo.Save(domain.User{
		Name: "Ana", Profession: "Electricista",
		BusinessName: "Servicios", BusinessActivity: "Electricidad", RFC: "AAA010101AAA",
		CLABE: "002010077777777771", CodiStatus: domain.NotRegisteredStatus(),
		Customer: domain.Customer{Name: "Luis", Phone: "5551234567", Email: "luis@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	banks := &countingBanks{}
	view, fields, err := NewRetryBankRegistration(repo, banks, nil).Retry(context.Background(), saved.ID)
	if err != nil || len(fields) > 0 || banks.calls != 1 {
		t.Fatalf("retry: view=%+v fields=%v err=%v calls=%d", view, fields, err, banks.calls)
	}
	if view.CodiStatus != definitions.CodiPending || view.BankReady || view.ClabeLast4 != "7771" {
		t.Fatalf("view: %+v", view)
	}
	stored, _ := repo.Find(saved.ID)
	if stored.BankAccountID != "01BANK" || stored.CodiStatus != definitions.CodiPending {
		t.Fatalf("stored: %+v", stored)
	}
}

func TestRetryBankSkipsWhenAlreadyReady(t *testing.T) {
	repo := infrastructure.NewMemoryRepository()
	saved, err := repo.Save(domain.User{
		Name: "Ana", Profession: "Electricista",
		CLABE: "002010077777777771", CodiStatus: definitions.CodiActive, BankAccountID: "01READY",
		Customer: domain.Customer{Name: "Luis", Phone: "5551234567", Email: "luis@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	banks := &countingBanks{}
	view, fields, err := NewRetryBankRegistration(repo, banks, nil).Retry(context.Background(), saved.ID)
	if err != nil || len(fields) > 0 || banks.calls != 0 {
		t.Fatalf("retry: fields=%v err=%v calls=%d", fields, err, banks.calls)
	}
	if !view.BankReady || view.CodiStatus != definitions.CodiActive {
		t.Fatalf("view: %+v", view)
	}
}

func TestRetryBankRequiresCLABE(t *testing.T) {
	repo := &memoryUsers{users: map[string]domain.User{
		"vendor-1": {
			ID: "vendor-1", Name: "Ana", Profession: "Electricista",
			CodiStatus: domain.NotRegisteredStatus(),
		},
	}}
	_, fields, err := NewRetryBankRegistration(repo, &countingBanks{}, nil).Retry(context.Background(), "vendor-1")
	if err != nil || fields[definitions.FieldCLABE] == "" {
		t.Fatalf("expected clabe field error, fields=%v err=%v", fields, err)
	}
}
