package infrastructure

import (
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"contractors-platform/backend/internal/platform/database"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSQLiteRepositoryPersistsAcrossReopen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	taxRegime := "General"
	input := domain.Input{Name: "Ana", Profession: "Electrician", Client: domain.Client{Name: "Luis", Phone: "+52 555 123 4567", Email: "luis@example.com"},
		TaxRegime: &taxRegime, BusinessName: "Electrical Services", BusinessActivity: "Electrical installations", RFC: "AAA010101AAA", CLABE: "002010077777777771"}

	repository, vendors := openStores(t, dir)
	created, err := repository.Save(mustUser(t, input))
	if err != nil {
		t.Fatal(err)
	}
	created.BankAccountID = "01BANK"
	created.CodiStatus = "PENDING"
	created.Customer.Name = "Otra persona"
	created, err = repository.RecordBankAccount(created)
	if err != nil {
		t.Fatal(err)
	}
	if created.Customer.Name != "" || created.BankAccountID != "01BANK" || created.BusinessName != "Electrical Services" {
		t.Fatalf("bank attach rewrote another table or kept a client row: %+v", created)
	}
	assertSplit(t, vendors, created.ID, "01BANK", "Electrical Services")
	withoutTax := input
	withoutTax.TaxRegime = nil
	withoutTax.CLABE = "012180001183597198"
	other, err := repository.Save(mustUser(t, withoutTax))
	if err != nil {
		t.Fatal(err)
	}
	other.Customer = domain.Customer{}
	_ = vendors.Close()

	repository, vendors = openStores(t, dir)
	defer vendors.Close()
	for _, want := range []domain.User{created, other} {
		got, found := repository.Find(want.ID)
		if !found || !reflect.DeepEqual(got, want) {
			t.Fatalf("find %s: found=%v got=%+v want=%+v", want.ID, found, got, want)
		}
	}
	if _, found := repository.Find("missing"); found {
		t.Fatal("expected missing record")
	}
	if _, err := repository.Save(mustUser(t, input)); !errors.Is(err, domain.ErrCLABETaken) {
		t.Fatalf("duplicate clabe: %v", err)
	}
}

func openStores(t *testing.T, dir string) (*SQLiteRepository, *sql.DB) {
	t.Helper()
	vendors, err := database.OpenSQLite(filepath.Join(dir, "vendors.db"))
	if err != nil {
		t.Fatal(err)
	}
	repository, err := NewSQLiteRepository(vendors)
	if err != nil {
		t.Fatal(err)
	}
	return repository, vendors
}

func assertSplit(t *testing.T, vendors *sql.DB, vendorID, bankID, businessName string) {
	t.Helper()
	var gotBank, gotBusiness, gotPerson string
	if err := vendors.QueryRow(`SELECT bank_account_id FROM bank_accounts WHERE vendor_id = ?`, vendorID).Scan(&gotBank); err != nil {
		t.Fatal(err)
	}
	if gotBank != bankID {
		t.Fatalf("bank id: %s", gotBank)
	}
	if err := vendors.QueryRow(`SELECT business_name FROM businesses WHERE vendor_id = ?`, vendorID).Scan(&gotBusiness); err != nil {
		t.Fatal(err)
	}
	if gotBusiness != businessName {
		t.Fatalf("business name: %s", gotBusiness)
	}
	if err := vendors.QueryRow(`SELECT name FROM vendors WHERE id = ?`, vendorID).Scan(&gotPerson); err != nil {
		t.Fatal(err)
	}
	if gotPerson == "" {
		t.Fatal("missing vendor person")
	}
	columns, err := columnSet(vendors, "vendors")
	if err != nil {
		t.Fatal(err)
	}
	if columns["clabe"] || columns["business_name"] || columns["bank_account_id"] {
		t.Fatalf("vendor table still mixes concerns: %v", columns)
	}
	var clientsTable string
	err = vendors.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'clients'`).Scan(&clientsTable)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("vendors db still has a clients table: %v %s", err, clientsTable)
	}
}

func columnSet(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

func TestRemoveDeletesTheVendor(t *testing.T) {
	repository, vendors := openStores(t, t.TempDir())
	defer vendors.Close()
	created, err := repository.Save(mustUser(t, domain.Input{Name: "Ana", Profession: "Electrician", Client: domain.Client{Name: "Luis", Phone: "5551234567", Email: "luis@example.com"}, BusinessName: "Electrical Services", BusinessActivity: "Electrical installations", RFC: "AAA010101AAA", CLABE: "002010077777777771"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Remove(created.ID); err != nil {
		t.Fatal(err)
	}
	if _, found := repository.Find(created.ID); found {
		t.Fatal("vendor remained after remove")
	}
}

func mustUser(t *testing.T, input domain.Input) domain.User {
	t.Helper()
	user, fieldErrors := domain.Build(input)
	if len(fieldErrors) > 0 {
		t.Fatalf("build: %v", fieldErrors)
	}
	return user
}
