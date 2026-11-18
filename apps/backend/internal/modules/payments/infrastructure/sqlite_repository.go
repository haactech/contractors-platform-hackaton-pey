package infrastructure

import (
	"contractors-platform/backend/internal/modules/payments/domain"
	"database/sql"
	"errors"
	"time"
)

func paymentSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS payments (
			id               TEXT PRIMARY KEY,
			vendor_id        TEXT NOT NULL,
			bank_account_id  TEXT NOT NULL,
			client_id        TEXT NOT NULL DEFAULT '',
			client_name      TEXT NOT NULL,
			client_phone     TEXT NOT NULL DEFAULT '',
			client_email     TEXT NOT NULL DEFAULT '',
			concept          TEXT NOT NULL,
			amount_cents     INTEGER NOT NULL,
			due_date         TEXT,
			method           TEXT NOT NULL,
			status           TEXT NOT NULL,
			pey_id           TEXT NOT NULL,
			payment_link_url TEXT NOT NULL DEFAULT '',
			codi_qr_string   TEXT NOT NULL DEFAULT '',
			codi_qr_image    TEXT NOT NULL DEFAULT '',
			reminder_count   INTEGER NOT NULL DEFAULT 0,
			created_at       TEXT NOT NULL,
			settled_at       TEXT)`,
	}
}

// SQLiteRepository guarda cada cobro en payments.db.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository crea la tabla de cobros.
func NewSQLiteRepository(db *sql.DB) (*SQLiteRepository, error) {
	if err := execEach(db, paymentSchema()); err != nil {
		return nil, err
	}
	if err := ensureClientID(db); err != nil {
		return nil, err
	}
	return &SQLiteRepository{db: db}, nil
}

func ensureClientID(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(payments)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primary int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primary); err != nil {
			return err
		}
		if name == "client_id" {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(`ALTER TABLE payments ADD COLUMN client_id TEXT NOT NULL DEFAULT ''`)
	return err
}

func execEach(db *sql.DB, statements []string) error {
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

// Allocate reserva el id del cobro antes de llamar a Pey.
func (repository *SQLiteRepository) Allocate() (string, error) {
	return newID()
}

// Save inserta el cobro si el contrato lo permite. No escribe el alta del vendedor.
func (repository *SQLiteRepository) Save(payment domain.Payment) (domain.Payment, error) {
	if fieldErrors := domain.Allow(payment); len(fieldErrors) > 0 {
		return domain.Payment{}, errRejected
	}
	if payment.CreatedAt == "" {
		payment.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	var due any
	if payment.DueDate != "" {
		due = payment.DueDate
	}
	_, err := repository.db.Exec(`INSERT INTO payments (
		id, vendor_id, bank_account_id, client_id, client_name, client_phone, client_email,
		concept, amount_cents, due_date, method, status, pey_id, payment_link_url,
		codi_qr_string, codi_qr_image, reminder_count, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		payment.ID, payment.VendorID, payment.BankAccountID, payment.ClientID, payment.ClientName, payment.ClientPhone, payment.ClientEmail,
		payment.Concept, payment.AmountCents, due, payment.Method, payment.Status, payment.PeyID, payment.PaymentLinkURL,
		payment.CodiQRString, payment.CodiQRImage, payment.ReminderCount, payment.CreatedAt)
	if err != nil {
		return domain.Payment{}, err
	}
	return payment, nil
}

const paymentSelect = `SELECT id, vendor_id, bank_account_id, client_id, client_name, client_phone, client_email,
	concept, amount_cents, due_date, method, status, pey_id, payment_link_url,
	codi_qr_string, codi_qr_image, reminder_count, created_at FROM payments`

// Find lee un cobro por id.
func (repository *SQLiteRepository) Find(id string) (domain.Payment, bool, error) {
	payment, err := scanPayment(repository.db.QueryRow(paymentSelect+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Payment{}, false, nil
	}
	if err != nil {
		return domain.Payment{}, false, err
	}
	return payment, true, nil
}

// ListByVendor lista los cobros de un vendedor, más recientes primero.
func (repository *SQLiteRepository) ListByVendor(vendorID string) ([]domain.Payment, error) {
	rows, err := repository.db.Query(paymentSelect+` WHERE vendor_id = ? ORDER BY created_at DESC, id DESC`, vendorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	payments := []domain.Payment{}
	for rows.Next() {
		payment, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}
	return payments, rows.Err()
}

type paymentScanner interface {
	Scan(dest ...any) error
}

func scanPayment(row paymentScanner) (domain.Payment, error) {
	var payment domain.Payment
	var due sql.NullString
	err := row.Scan(
		&payment.ID, &payment.VendorID, &payment.BankAccountID, &payment.ClientID, &payment.ClientName, &payment.ClientPhone, &payment.ClientEmail,
		&payment.Concept, &payment.AmountCents, &due, &payment.Method, &payment.Status, &payment.PeyID, &payment.PaymentLinkURL,
		&payment.CodiQRString, &payment.CodiQRImage, &payment.ReminderCount, &payment.CreatedAt,
	)
	if err != nil {
		return domain.Payment{}, err
	}
	if due.Valid {
		payment.DueDate = due.String
	}
	return payment, nil
}
