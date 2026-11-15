package infrastructure

import (
	"contractors-platform/backend/internal/modules/clients/domain"
	"contractors-platform/backend/internal/platform/database"
	"path/filepath"
	"testing"
)

func TestNewColumnsReopenAndOldTableGainsThem(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clients.db")
	db, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	repository, err := NewSQLiteRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := repository.Save(person("vendor-1", "Luis", "5551234567"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Save(person("vendor-1", "Ana", "5551234567")); err != domain.ErrPhoneTaken {
		t.Fatalf("duplicate: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	repository, err = NewSQLiteRepository(reopened)
	if err != nil {
		t.Fatal(err)
	}
	loaded, found, err := repository.Find("vendor-1", saved.ID)
	if err != nil || !found || loaded.EntityType != "individual" || loaded.PhoneKey != "5551234567" || loaded.LegalName != "" {
		t.Fatalf("reopen: found=%v err=%v %+v", found, err, loaded)
	}

	oldPath := filepath.Join(dir, "old.db")
	oldDB, err := database.OpenSQLite(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	defer oldDB.Close()
	if _, err := oldDB.Exec(`CREATE TABLE clients (
		id TEXT PRIMARY KEY, vendor_id TEXT NOT NULL, name TEXT NOT NULL, phone TEXT NOT NULL, email TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')))`); err != nil {
		t.Fatal(err)
	}
	if _, err := oldDB.Exec(`INSERT INTO clients (id, vendor_id, name, phone, email) VALUES (?, ?, ?, ?, ?)`,
		"client-old", "vendor-1", "Luis", "+52 555 123 4567", "luis@example.com"); err != nil {
		t.Fatal(err)
	}
	oldRepository, err := NewSQLiteRepository(oldDB)
	if err != nil {
		t.Fatal(err)
	}
	people, err := oldRepository.List("vendor-1")
	if err != nil || len(people) != 1 || people[0].Name != "Luis" || people[0].EntityType != "individual" || people[0].Phone != "+52 555 123 4567" || people[0].PhoneKey != "5551234567" {
		t.Fatalf("old table: %v %+v", err, people)
	}
	if _, err := oldRepository.Save(person("vendor-1", "Ana", "5551234567")); err != domain.ErrPhoneTaken {
		t.Fatalf("old duplicate: %v", err)
	}
}

func person(vendorID, name, phone string) domain.Client {
	client, fieldErrors := domain.BuildDashboard(vendorID, domain.Input{EntityType: "individual", Name: name, Phone: phone, Email: "luis@example.com"})
	if len(fieldErrors) > 0 {
		panic(fieldErrors)
	}
	return client
}
