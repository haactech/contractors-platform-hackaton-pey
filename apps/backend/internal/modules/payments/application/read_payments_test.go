package application

import (
	"contractors-platform/backend/internal/modules/payments/definitions"
	"contractors-platform/backend/internal/modules/payments/domain"
	"testing"
)

func TestListByVendorReturnsNewestFirst(t *testing.T) {
	repository := &fakeRepository{}
	first := readySaved("pay-1", "vendor-1")
	second := readySaved("pay-2", "vendor-1")
	other := readySaved("pay-3", "vendor-2")
	if _, err := repository.Save(first); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Save(second); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Save(other); err != nil {
		t.Fatal(err)
	}

	views, err := NewReadPayments(repository).ListByVendor("vendor-1")
	if err != nil || len(views) != 2 {
		t.Fatalf("list: %v len=%d", err, len(views))
	}
	if views[0].ID != "pay-2" || views[1].ID != "pay-1" {
		t.Fatalf("order: %+v", views)
	}
	if views[0].ClientID != "client-1" || views[0].Status != definitions.StatusPending {
		t.Fatalf("view: %+v", views[0])
	}
}

func TestFindScopedToVendor(t *testing.T) {
	repository := &fakeRepository{}
	if _, err := repository.Save(readySaved("pay-1", "vendor-1")); err != nil {
		t.Fatal(err)
	}
	read := NewReadPayments(repository)

	view, found, err := read.Find("vendor-1", "pay-1")
	if err != nil || !found || view.ID != "pay-1" || view.ClientID != "client-1" {
		t.Fatalf("find: found=%v err=%v %+v", found, err, view)
	}
	if _, found, err := read.Find("vendor-2", "pay-1"); err != nil || found {
		t.Fatalf("other vendor: found=%v err=%v", found, err)
	}
	if _, found, err := read.Find("vendor-1", "missing"); err != nil || found {
		t.Fatalf("missing: found=%v err=%v", found, err)
	}
}

func readySaved(id, vendorID string) domain.Payment {
	return domain.Payment{
		ID: id, VendorID: vendorID, BankAccountID: "bank-1", ClientID: "client-1",
		ClientName: "Luis", Concept: "Mensualidad", AmountCents: 100,
		Method: definitions.MethodCodiQR, Status: definitions.StatusPending,
		PeyID: "pey-1", CodiQRString: "QR", CreatedAt: "2026-11-14T00:00:00Z",
	}
}
