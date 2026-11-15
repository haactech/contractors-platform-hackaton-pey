package infrastructure

import (
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"strings"
)

func vendorTables() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS vendors (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			profession TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')))`,
		`CREATE TABLE IF NOT EXISTS businesses (
			vendor_id         TEXT PRIMARY KEY,
			business_name     TEXT NOT NULL,
			business_activity TEXT NOT NULL,
			rfc               TEXT NOT NULL,
			tax_regime        TEXT)`,
		`CREATE TABLE IF NOT EXISTS bank_accounts (
			vendor_id       TEXT PRIMARY KEY,
			clabe           TEXT NOT NULL DEFAULT '',
			codi_status     TEXT NOT NULL DEFAULT '` + domain.NotRegisteredStatus() + `',
			bank_account_id TEXT NOT NULL DEFAULT '')`,
		`CREATE UNIQUE INDEX IF NOT EXISTS bank_accounts_clabe_unique ON bank_accounts(clabe) WHERE clabe <> ''`,
	}
}

// SQLiteRepository guarda la persona, su negocio y su cuenta. Los clientes viven en otro módulo.
type SQLiteRepository struct {
	vendors *sql.DB
}

// NewSQLiteRepository crea las tablas del vendedor, su negocio y su cuenta.
func NewSQLiteRepository(vendors *sql.DB) (*SQLiteRepository, error) {
	if err := execEach(vendors, vendorTables()); err != nil {
		return nil, err
	}
	return &SQLiteRepository{vendors: vendors}, nil
}

func execEach(db *sql.DB, statements []string) error {
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func newID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

// HasCLABE indica si otra cuenta ya guardó esa CLABE.
func (repository *SQLiteRepository) HasCLABE(clabe string) (bool, error) {
	return repository.HasOtherCLABE(clabe, "")
}

// HasOtherCLABE indica si otro contratista distinto a excludeID ya guardó esa CLABE.
func (repository *SQLiteRepository) HasOtherCLABE(clabe, excludeID string) (bool, error) {
	if clabe == "" {
		return false, nil
	}
	var found int
	err := repository.vendors.QueryRow(
		`SELECT 1 FROM bank_accounts WHERE clabe = ? AND vendor_id <> ? LIMIT 1`,
		clabe, excludeID,
	).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Save guarda la persona, su negocio y su cuenta. El primer cliente lo escribe el módulo de clientes.
func (repository *SQLiteRepository) Save(user domain.User) (domain.User, error) {
	if fieldErrors := domain.Allow(user); len(fieldErrors) > 0 {
		return domain.User{}, errRejected
	}
	taken, err := repository.HasCLABE(user.CLABE)
	if err != nil {
		return domain.User{}, err
	}
	if taken {
		return domain.User{}, domain.ErrCLABETaken
	}
	user.ID, err = newID()
	if err != nil {
		return domain.User{}, err
	}
	transaction, err := repository.vendors.Begin()
	if err != nil {
		return domain.User{}, err
	}
	_, err = transaction.Exec(`INSERT INTO vendors (id, name, profession) VALUES (?, ?, ?)`,
		user.ID, user.Name, user.Profession)
	if err != nil {
		_ = transaction.Rollback()
		return domain.User{}, err
	}
	_, err = transaction.Exec(`INSERT INTO businesses (vendor_id, business_name, business_activity, rfc, tax_regime) VALUES (?, ?, ?, ?, ?)`,
		user.ID, user.BusinessName, user.BusinessActivity, user.RFC, user.TaxRegime)
	if err != nil {
		_ = transaction.Rollback()
		return domain.User{}, err
	}
	_, err = transaction.Exec(`INSERT INTO bank_accounts (vendor_id, clabe, codi_status, bank_account_id) VALUES (?, ?, ?, ?)`,
		user.ID, user.CLABE, user.CodiStatus, user.BankAccountID)
	if err != nil {
		_ = transaction.Rollback()
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return domain.User{}, domain.ErrCLABETaken
		}
		return domain.User{}, err
	}
	if err := transaction.Commit(); err != nil {
		return domain.User{}, err
	}
	return cloneUser(user), nil
}

// Update actualiza los campos editables de perfil, negocio y cuenta de depósito de un contratista existente.
func (repository *SQLiteRepository) Update(user domain.User) (domain.User, error) {
	if fieldErrors := domain.AllowVendor(user); len(fieldErrors) > 0 {
		return domain.User{}, errRejected
	}
	taken, err := repository.HasOtherCLABE(user.CLABE, user.ID)
	if err != nil {
		return domain.User{}, err
	}
	if taken {
		return domain.User{}, domain.ErrCLABETaken
	}
	transaction, err := repository.vendors.Begin()
	if err != nil {
		return domain.User{}, err
	}
	result, err := transaction.Exec(
		`UPDATE vendors SET name = ?, profession = ? WHERE id = ?`,
		user.Name, user.Profession, user.ID,
	)
	if err != nil {
		_ = transaction.Rollback()
		return domain.User{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		_ = transaction.Rollback()
		return domain.User{}, err
	}
	if rows == 0 {
		_ = transaction.Rollback()
		return domain.User{}, domain.ErrNotFound
	}
	if _, err := transaction.Exec(
		`UPDATE businesses SET business_name = ?, business_activity = ?, rfc = ?, tax_regime = ? WHERE vendor_id = ?`,
		user.BusinessName, user.BusinessActivity, user.RFC, user.TaxRegime, user.ID,
	); err != nil {
		_ = transaction.Rollback()
		return domain.User{}, err
	}
	if _, err := transaction.Exec(
		`UPDATE bank_accounts SET clabe = ?, codi_status = ?, bank_account_id = ? WHERE vendor_id = ?`,
		user.CLABE, user.CodiStatus, user.BankAccountID, user.ID,
	); err != nil {
		_ = transaction.Rollback()
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return domain.User{}, domain.ErrCLABETaken
		}
		return domain.User{}, err
	}
	if err := transaction.Commit(); err != nil {
		return domain.User{}, err
	}
	updated, found := repository.Find(user.ID)
	if !found {
		return domain.User{}, domain.ErrNotFound
	}
	return updated, nil
}

// Remove borra el alta que no pudo guardar su primer cliente.
func (repository *SQLiteRepository) Remove(id string) error {
	transaction, err := repository.vendors.Begin()
	if err != nil {
		return err
	}
	for _, statement := range []string{
		`DELETE FROM bank_accounts WHERE vendor_id = ?`,
		`DELETE FROM businesses WHERE vendor_id = ?`,
		`DELETE FROM vendors WHERE id = ?`,
	} {
		if _, err := transaction.Exec(statement, id); err != nil {
			_ = transaction.Rollback()
			return err
		}
	}
	return transaction.Commit()
}

// RecordBankAccount anota el id y el estado de Pey solo en la cuenta del vendedor.
func (repository *SQLiteRepository) RecordBankAccount(user domain.User) (domain.User, error) {
	if fieldErrors := domain.AllowVendor(user); len(fieldErrors) > 0 {
		return domain.User{}, errRejected
	}
	result, err := repository.vendors.Exec(`UPDATE bank_accounts SET bank_account_id = ?, codi_status = ? WHERE vendor_id = ?`,
		user.BankAccountID, user.CodiStatus, user.ID)
	if err != nil {
		return domain.User{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return domain.User{}, err
	}
	if rows == 0 {
		return domain.User{}, errRejected
	}
	recorded, found := repository.Find(user.ID)
	if !found {
		return domain.User{}, errRejected
	}
	return recorded, nil
}

// Find arma el vendedor desde su persona, su negocio y su cuenta. El cliente lo adjunta la lectura.
func (repository *SQLiteRepository) Find(id string) (domain.User, bool) {
	return repository.findOne(`vendors.id = ?`, id)
}

// FindByCLABE busca un usuario por CLABE normalizada.
func (repository *SQLiteRepository) FindByCLABE(clabe string) (domain.User, bool) {
	if clabe == "" {
		return domain.User{}, false
	}
	return repository.findOne(`bank_accounts.clabe = ? LIMIT 1`, clabe)
}

func (repository *SQLiteRepository) findOne(whereClause, arg string) (domain.User, bool) {
	var user domain.User
	var taxRegime sql.NullString
	err := repository.vendors.QueryRow(`SELECT vendors.id, vendors.name, vendors.profession,
		businesses.business_name, businesses.business_activity, businesses.rfc, businesses.tax_regime,
		bank_accounts.clabe, bank_accounts.codi_status, bank_accounts.bank_account_id
		FROM vendors
		JOIN businesses ON businesses.vendor_id = vendors.id
		JOIN bank_accounts ON bank_accounts.vendor_id = vendors.id
		WHERE `+whereClause, arg).Scan(
		&user.ID, &user.Name, &user.Profession, &user.BusinessName, &user.BusinessActivity, &user.RFC, &taxRegime,
		&user.CLABE, &user.CodiStatus, &user.BankAccountID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("onboarding: find vendor %s: %v", arg, err)
		}
		return domain.User{}, false
	}
	if taxRegime.Valid {
		user.TaxRegime = domain.CopyText(&taxRegime.String)
	}
	return cloneUser(user), true
}
