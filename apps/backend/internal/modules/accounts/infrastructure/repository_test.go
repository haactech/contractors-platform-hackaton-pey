package infrastructure

import (
	"contractors-platform/backend/internal/modules/accounts/domain"
	"contractors-platform/backend/internal/platform/database"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func runRepositoryContractTests(t *testing.T, newRepo func(t *testing.T) domain.Repository) {
	t.Run("unique email and phone", func(t *testing.T) {
		repo := newRepo(t)
		_, err := repo.Create(domain.Account{Email: "a@x.mx", Phone: "+525511111111", PINHash: "h1"})
		if err != nil {
			t.Fatalf("create first: %v", err)
		}
		_, err = repo.Create(domain.Account{Email: "a@x.mx", Phone: "+525522222222", PINHash: "h2"})
		if !errors.Is(err, domain.ErrEmailTaken) {
			t.Fatalf("expected ErrEmailTaken, got %v", err)
		}
		_, err = repo.Create(domain.Account{Email: "b@x.mx", Phone: "+525511111111", PINHash: "h3"})
		if !errors.Is(err, domain.ErrPhoneTaken) {
			t.Fatalf("expected ErrPhoneTaken, got %v", err)
		}
	})

	t.Run("link contractor 1:1", func(t *testing.T) {
		repo := newRepo(t)
		a1, _ := repo.Create(domain.Account{Email: "a1@x.mx", Phone: "+525511111111", PINHash: "h"})
		a2, _ := repo.Create(domain.Account{Email: "a2@x.mx", Phone: "+525522222222", PINHash: "h"})

		if err := repo.LinkContractor(a1.ID, "con-1"); err != nil {
			t.Fatalf("first link: %v", err)
		}
		if err := repo.LinkContractor(a1.ID, "con-2"); !errors.Is(err, domain.ErrAlreadyLinked) {
			t.Fatalf("second link same account: expected ErrAlreadyLinked, got %v", err)
		}
		if err := repo.LinkContractor(a2.ID, "con-1"); !errors.Is(err, domain.ErrAlreadyLinked) {
			t.Fatalf("link same contractor to another account: expected ErrAlreadyLinked, got %v", err)
		}
	})

	t.Run("sessions expiry and bulk delete", func(t *testing.T) {
		repo := newRepo(t)
		acc, _ := repo.Create(domain.Account{Email: "s@x.mx", Phone: "+525533333333", PINHash: "h"})
		now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

		if err := repo.SaveSession(domain.Session{TokenHash: "t1", AccountID: acc.ID, ExpiresAt: now.Add(time.Hour)}, now); err != nil {
			t.Fatal(err)
		}
		if err := repo.SaveSession(domain.Session{TokenHash: "t2", AccountID: acc.ID, ExpiresAt: now.Add(-time.Minute)}, now); err != nil {
			t.Fatal(err)
		}
		if _, found, err := repo.FindSession("t1", now); err != nil || !found {
			t.Fatalf("expected t1 found: found=%v err=%v", found, err)
		}
		if _, found, err := repo.FindSession("t2", now); err != nil || found {
			t.Fatalf("expected expired t2 not found: found=%v err=%v", found, err)
		}
		if err := repo.DeleteSessionsFor(acc.ID); err != nil {
			t.Fatal(err)
		}
		if _, found, _ := repo.FindSession("t1", now); found {
			t.Fatal("expected t1 deleted after DeleteSessionsFor")
		}
	})

	t.Run("codes invalidate previous active code", func(t *testing.T) {
		repo := newRepo(t)
		acc, _ := repo.Create(domain.Account{Email: "c@x.mx", Phone: "+525544444444", PINHash: "h"})
		now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

		err := repo.SaveCode(domain.Code{
			AccountID: acc.ID, Purpose: "email_verification", CodeHash: "hash1", Salt: "s1",
			ExpiresAt: now.Add(15 * time.Minute), CreatedAt: now,
		})
		if err != nil {
			t.Fatal(err)
		}
		err = repo.SaveCode(domain.Code{
			AccountID: acc.ID, Purpose: "email_verification", CodeHash: "hash2", Salt: "s2",
			ExpiresAt: now.Add(15 * time.Minute), CreatedAt: now.Add(time.Minute),
		})
		if err != nil {
			t.Fatal(err)
		}
		latest, found, err := repo.LatestCode(acc.ID, "email_verification")
		if err != nil || !found || latest.CodeHash != "hash2" || latest.ConsumedAt != nil {
			t.Fatalf("unexpected latest code: found=%v err=%v code=%+v", found, err, latest)
		}
	})
}

func TestMemoryRepositoryContract(t *testing.T) {
	runRepositoryContractTests(t, func(t *testing.T) domain.Repository {
		return NewMemoryRepository()
	})
}

func TestSQLiteRepositoryContract(t *testing.T) {
	runRepositoryContractTests(t, func(t *testing.T) domain.Repository {
		db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "accounts.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		repo, err := NewSQLiteRepository(db)
		if err != nil {
			t.Fatal(err)
		}
		return repo
	})
}
