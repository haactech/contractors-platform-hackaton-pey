package infrastructure

import (
	"contractors-platform/backend/internal/modules/clients/domain"
	"database/sql"
	"errors"
	"time"
)

func clientSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS clients (
			id              TEXT PRIMARY KEY,
			vendor_id       TEXT NOT NULL,
			entity_type     TEXT NOT NULL DEFAULT 'individual',
			name            TEXT NOT NULL,
			phone           TEXT NOT NULL,
			email           TEXT NOT NULL,
			legal_name      TEXT NOT NULL DEFAULT '',
			trade_name      TEXT NOT NULL DEFAULT '',
			rfc             TEXT NOT NULL DEFAULT '',
			activity        TEXT NOT NULL DEFAULT '',
			tax_regime      TEXT NOT NULL DEFAULT '',
			fiscal_address  TEXT NOT NULL DEFAULT '',
			phone_key       TEXT NOT NULL DEFAULT '',
			created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
			updated_at      TEXT NOT NULL DEFAULT '')`,
		`CREATE INDEX IF NOT EXISTS clients_vendor_id ON clients(vendor_id)`,
	}
}

// SQLiteRepository guarda cada cliente en clients.db.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository crea la tabla y completa las columnas que falten en un archivo anterior.
func NewSQLiteRepository(db *sql.DB) (*SQLiteRepository, error) {
	if err := execEach(db, clientSchema()); err != nil {
		return nil, err
	}
	if err := ensureColumns(db); err != nil {
		return nil, err
	}
	if err := backfillKeys(db); err != nil {
		return nil, err
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS clients_vendor_phone_key ON clients(vendor_id, phone_key) WHERE phone_key <> ''`); err != nil {
		return nil, err
	}
	return &SQLiteRepository{db: db}, nil
}

func execEach(db *sql.DB, statements []string) error {
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func ensureColumns(db *sql.DB) error {
	present, err := columnSet(db)
	if err != nil {
		return err
	}
	missing := []string{
		`ALTER TABLE clients ADD COLUMN entity_type TEXT NOT NULL DEFAULT 'individual'`,
		`ALTER TABLE clients ADD COLUMN legal_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE clients ADD COLUMN trade_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE clients ADD COLUMN rfc TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE clients ADD COLUMN activity TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE clients ADD COLUMN tax_regime TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE clients ADD COLUMN fiscal_address TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE clients ADD COLUMN phone_key TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE clients ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''`,
	}
	names := []string{"entity_type", "legal_name", "trade_name", "rfc", "activity", "tax_regime", "fiscal_address", "phone_key", "updated_at"}
	for index, name := range names {
		if present[name] {
			continue
		}
		if _, err := db.Exec(missing[index]); err != nil {
			return err
		}
	}
	return nil
}

func columnSet(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`PRAGMA table_info(clients)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, kind string
		var notNull int
		var defaultValue any
		var primary int
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primary); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

func backfillKeys(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, phone, created_at, phone_key, updated_at FROM clients`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct{ id, phone, createdAt, phoneKey, updatedAt string }
	var pending []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.phone, &item.createdAt, &item.phoneKey, &item.updatedAt); err != nil {
			return err
		}
		pending = append(pending, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range pending {
		phoneKey := item.phoneKey
		if phoneKey == "" {
			phoneKey = domain.PhoneKey(item.phone)
		}
		updatedAt := item.updatedAt
		if updatedAt == "" {
			updatedAt = item.createdAt
		}
		if phoneKey == item.phoneKey && updatedAt == item.updatedAt {
			continue
		}
		if _, err := db.Exec(`UPDATE clients SET phone_key = ?, updated_at = ? WHERE id = ?`, phoneKey, updatedAt, item.id); err != nil {
			return err
		}
	}
	return nil
}

// Save inserta el cliente si el contrato y el teléfono lo permiten.
func (repository *SQLiteRepository) Save(client domain.Client) (domain.Client, error) {
	if fieldErrors := domain.Allow(client); len(fieldErrors) > 0 {
		return domain.Client{}, errRejected
	}
	if taken, err := repository.phoneTaken(client.VendorID, client.PhoneKey, ""); err != nil {
		return domain.Client{}, err
	} else if taken {
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
	_, err = repository.db.Exec(`INSERT INTO clients (
		id, vendor_id, entity_type, name, phone, email, legal_name, trade_name, rfc, activity, tax_regime, fiscal_address, phone_key, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		client.ID, client.VendorID, client.EntityType, client.Name, client.Phone, client.Email,
		client.LegalName, client.TradeName, client.RFC, client.Activity, client.TaxRegime, client.FiscalAddress,
		client.PhoneKey, client.CreatedAt, client.UpdatedAt)
	if err != nil {
		return domain.Client{}, err
	}
	return client, nil
}

// Update reemplaza el perfil editable.
func (repository *SQLiteRepository) Update(client domain.Client) (domain.Client, error) {
	if fieldErrors := domain.Allow(client); len(fieldErrors) > 0 {
		return domain.Client{}, errRejected
	}
	current, found, err := repository.Find(client.VendorID, client.ID)
	if err != nil {
		return domain.Client{}, err
	}
	if !found {
		return domain.Client{}, domain.ErrClientNotFound
	}
	if taken, err := repository.phoneTaken(client.VendorID, client.PhoneKey, client.ID); err != nil {
		return domain.Client{}, err
	} else if taken {
		return domain.Client{}, domain.ErrPhoneTaken
	}
	client.CreatedAt = current.CreatedAt
	client.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	result, err := repository.db.Exec(`UPDATE clients SET entity_type = ?, name = ?, phone = ?, email = ?, legal_name = ?, trade_name = ?, rfc = ?, activity = ?, tax_regime = ?, fiscal_address = ?, phone_key = ?, updated_at = ? WHERE id = ? AND vendor_id = ?`,
		client.EntityType, client.Name, client.Phone, client.Email, client.LegalName, client.TradeName, client.RFC, client.Activity, client.TaxRegime, client.FiscalAddress, client.PhoneKey, client.UpdatedAt, client.ID, client.VendorID)
	if err != nil {
		return domain.Client{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return domain.Client{}, err
	}
	if rows == 0 {
		return domain.Client{}, domain.ErrClientNotFound
	}
	return client, nil
}

// List devuelve los clientes del vendedor, del más antiguo al más nuevo.
func (repository *SQLiteRepository) List(vendorID string) ([]domain.Client, error) {
	rows, err := repository.db.Query(`SELECT id, vendor_id, entity_type, name, phone, email, legal_name, trade_name, rfc, activity, tax_regime, fiscal_address, phone_key, created_at, updated_at FROM clients WHERE vendor_id = ? ORDER BY created_at, id`, vendorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var clients []domain.Client
	for rows.Next() {
		client, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		clients = append(clients, client)
	}
	return clients, rows.Err()
}

// Find busca un cliente de ese vendedor.
func (repository *SQLiteRepository) Find(vendorID, clientID string) (domain.Client, bool, error) {
	row := repository.db.QueryRow(`SELECT id, vendor_id, entity_type, name, phone, email, legal_name, trade_name, rfc, activity, tax_regime, fiscal_address, phone_key, created_at, updated_at FROM clients WHERE vendor_id = ? AND id = ?`, vendorID, clientID)
	client, err := scanClient(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Client{}, false, nil
	}
	if err != nil {
		return domain.Client{}, false, err
	}
	return client, true, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanClient(row scanner) (domain.Client, error) {
	var client domain.Client
	err := row.Scan(&client.ID, &client.VendorID, &client.EntityType, &client.Name, &client.Phone, &client.Email,
		&client.LegalName, &client.TradeName, &client.RFC, &client.Activity, &client.TaxRegime, &client.FiscalAddress,
		&client.PhoneKey, &client.CreatedAt, &client.UpdatedAt)
	return client, err
}

func (repository *SQLiteRepository) phoneTaken(vendorID, phoneKey, exceptID string) (bool, error) {
	if phoneKey == "" {
		return false, nil
	}
	var id string
	err := repository.db.QueryRow(`SELECT id FROM clients WHERE vendor_id = ? AND phone_key = ? AND id <> ? LIMIT 1`, vendorID, phoneKey, exceptID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
