package infrastructure

import (
	onboardinginfra "contractors-platform/backend/internal/modules/onboarding/infrastructure"
	"contractors-platform/backend/internal/modules/payments/domain"
	"contractors-platform/backend/internal/platform/database"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestVendorPortReadsOnboardingRowsAndPaymentReopens(t *testing.T) {
	dir := t.TempDir()
	vendors, err := database.OpenSQLite(filepath.Join(dir, "vendors.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer vendors.Close()
	if _, err := onboardinginfra.NewSQLiteRepository(vendors); err != nil {
		t.Fatal(err)
	}
	if _, err := vendors.Exec(`INSERT INTO vendors (id, name, profession) VALUES (?, ?, ?)`,
		"vendor-1", "Luis", "Plomero"); err != nil {
		t.Fatal(err)
	}
	if _, err := vendors.Exec(`INSERT INTO bank_accounts (vendor_id, clabe, codi_status, bank_account_id) VALUES (?, ?, ?, ?)`,
		"vendor-1", "002010077777777771", "ACTIVE", "bank-vendor-1"); err != nil {
		t.Fatal(err)
	}

	source := NewSQLiteVendors(vendors)
	account, found, err := source.Account("vendor-1")
	if err != nil || !found || account.BankAccountID != "bank-vendor-1" || account.CodiStatus != "ACTIVE" {
		t.Fatalf("account: found=%v err=%v %+v", found, err, account)
	}
	if _, found, err := source.Account("missing"); err != nil || found {
		t.Fatalf("missing vendor: found=%v err=%v", found, err)
	}

	paymentsPath := filepath.Join(dir, "payments.db")
	paymentsDB, err := database.OpenSQLite(paymentsPath)
	if err != nil {
		t.Fatal(err)
	}
	repository, err := NewSQLiteRepository(paymentsDB)
	if err != nil {
		t.Fatal(err)
	}
	payment := domain.Payment{
		ID: "pay-1", VendorID: "vendor-1", BankAccountID: account.BankAccountID, ClientID: "client-1",
		ClientName: "Luis", ClientPhone: "5551234567", ClientEmail: "luis@example.com",
		Concept: "Mensualidad", AmountCents: 15000, DueDate: "2026-12-01",
		Method: "PAYMENT_LINK", Status: "PENDING_PAYMENT", PeyID: "pey-1",
		PaymentLinkURL: "https://pay.example/abc",
	}
	if _, err := repository.Save(payment); err != nil {
		t.Fatal(err)
	}
	if err := paymentsDB.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := database.OpenSQLite(paymentsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := NewSQLiteRepository(reopened); err != nil {
		t.Fatal(err)
	}
	var stored domain.Payment
	var due sql.NullString
	err = reopened.QueryRow(`SELECT id, vendor_id, bank_account_id, client_id, client_name, client_phone, client_email, concept, amount_cents, due_date, method, status, pey_id, payment_link_url, reminder_count, created_at FROM payments WHERE id = ?`, "pay-1").
		Scan(&stored.ID, &stored.VendorID, &stored.BankAccountID, &stored.ClientID, &stored.ClientName, &stored.ClientPhone, &stored.ClientEmail, &stored.Concept, &stored.AmountCents, &due, &stored.Method, &stored.Status, &stored.PeyID, &stored.PaymentLinkURL, &stored.ReminderCount, &stored.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !due.Valid || due.String != "2026-12-01" || stored.BankAccountID != "bank-vendor-1" || stored.ClientID != "client-1" || stored.AmountCents != 15000 || stored.CreatedAt == "" {
		t.Fatalf("reopened: due=%+v %+v", due, stored)
	}

	reader, err := NewSQLiteRepository(reopened)
	if err != nil {
		t.Fatal(err)
	}
	foundPayment, found, err := reader.Find("pay-1")
	if err != nil || !found || foundPayment.ClientID != "client-1" || foundPayment.DueDate != "2026-12-01" {
		t.Fatalf("find: found=%v err=%v %+v", found, err, foundPayment)
	}
	listed, err := reader.ListByVendor("vendor-1")
	if err != nil || len(listed) != 1 || listed[0].ID != "pay-1" {
		t.Fatalf("list: %v %+v", err, listed)
	}
	empty, err := reader.ListByVendor("missing")
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty list: %v %+v", err, empty)
	}
}

func TestVendorExistsWithoutBankReturnsEmptyAccount(t *testing.T) {
	vendors, err := database.OpenSQLite(filepath.Join(t.TempDir(), "vendors.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer vendors.Close()
	if _, err := onboardinginfra.NewSQLiteRepository(vendors); err != nil {
		t.Fatal(err)
	}
	if _, err := vendors.Exec(`INSERT INTO vendors (id, name, profession) VALUES (?, ?, ?)`,
		"vendor-soft", "Ana", "Diseñadora"); err != nil {
		t.Fatal(err)
	}

	source := NewSQLiteVendors(vendors)
	account, found, err := source.Account("vendor-soft")
	if err != nil || !found || account.BankAccountID != "" || account.CodiStatus != "" {
		t.Fatalf("expected found empty account, got found=%v err=%v %+v", found, err, account)
	}
	if _, found, err := source.Account("missing"); err != nil || found {
		t.Fatalf("missing vendor: found=%v err=%v", found, err)
	}
}

func TestOldPaymentsTableGainsClientID(t *testing.T) {
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "payments.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE payments (
		id TEXT PRIMARY KEY, vendor_id TEXT NOT NULL, bank_account_id TEXT NOT NULL,
		client_name TEXT NOT NULL, client_phone TEXT NOT NULL DEFAULT '', client_email TEXT NOT NULL DEFAULT '',
		concept TEXT NOT NULL, amount_cents INTEGER NOT NULL, due_date TEXT, method TEXT NOT NULL,
		status TEXT NOT NULL, pey_id TEXT NOT NULL, payment_link_url TEXT NOT NULL DEFAULT '',
		codi_qr_string TEXT NOT NULL DEFAULT '', codi_qr_image TEXT NOT NULL DEFAULT '',
		reminder_count INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, settled_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	repository, err := NewSQLiteRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Save(domain.Payment{
		ID: "pay-old", VendorID: "vendor-1", BankAccountID: "bank-1", ClientID: "client-9",
		ClientName: "Luis", Concept: "Mensualidad", AmountCents: 100, Method: "PAYMENT_LINK",
		Status: "PENDING_PAYMENT", PeyID: "pey-1", PaymentLinkURL: "https://pay.example/abc",
	}); err != nil {
		t.Fatal(err)
	}
	var clientID string
	if err := db.QueryRow(`SELECT client_id FROM payments WHERE id = ?`, "pay-old").Scan(&clientID); err != nil || clientID != "client-9" {
		t.Fatalf("client id: %s %v", clientID, err)
	}
}
