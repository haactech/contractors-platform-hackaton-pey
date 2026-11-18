package domain

import (
	"contractors-platform/backend/internal/modules/onboarding/definitions"
	"testing"
)

func TestClabeLast4(t *testing.T) {
	if got := ClabeLast4("002010077777777771"); got != "7771" {
		t.Fatalf("got %q", got)
	}
	if got := ClabeLast4("short"); got != "" {
		t.Fatalf("short: got %q", got)
	}
}

func TestBankReady(t *testing.T) {
	if BankReady(User{BankAccountID: "01BANK", CodiStatus: definitions.CodiPending}) {
		t.Fatal("pending must not be ready")
	}
	if !BankReady(User{BankAccountID: "01BANK", CodiStatus: definitions.CodiActive}) {
		t.Fatal("active must be ready")
	}
	if BankReady(User{CodiStatus: definitions.CodiActive}) {
		t.Fatal("active without id must not be ready")
	}
}

func TestViewMasksClabeAndExposesReadiness(t *testing.T) {
	view := View(User{
		ID:            "v1",
		Name:          "Ana",
		Profession:    "Electricista",
		CLABE:         "002010077777777771",
		BankAccountID: "01BANK",
		CodiStatus:    definitions.CodiPending,
		Customer:      Customer{Name: "Luis", Phone: "5512345678", Email: "luis@example.com"},
	})
	if view.ClabeLast4 != "7771" || view.CodiStatus != definitions.CodiPending || view.BankReady {
		t.Fatalf("view: %+v", view)
	}
	soft := View(User{ID: "v2", CodiStatus: NotRegisteredStatus()})
	if soft.ClabeLast4 != "" || soft.CodiStatus != definitions.CodiNotRegistered || soft.BankReady {
		t.Fatalf("soft: %+v", soft)
	}
}
