package infrastructure

import (
	"contractors-platform/backend/internal/modules/accounts/domain"
	"database/sql"
	"errors"
	"strings"
	"time"
)

func accountsSchema() string {
	return `
CREATE TABLE IF NOT EXISTS accounts (
	id                TEXT PRIMARY KEY,
	email             TEXT NOT NULL,
	phone             TEXT NOT NULL,
	pin_hash          TEXT NOT NULL,
	contractor_id     TEXT,
	email_verified_at TEXT,
	failed_logins     INTEGER NOT NULL DEFAULT 0,
	locked_until      TEXT,
	is_demo           INTEGER NOT NULL DEFAULT 0,
	created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS accounts_email_unique ON accounts(email);
CREATE UNIQUE INDEX IF NOT EXISTS accounts_phone_unique ON accounts(phone);
CREATE UNIQUE INDEX IF NOT EXISTS accounts_contractor_unique ON accounts(contractor_id) WHERE contractor_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT PRIMARY KEY,
	account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX IF NOT EXISTS sessions_account_idx ON sessions(account_id);

CREATE TABLE IF NOT EXISTS auth_codes (
	id          TEXT PRIMARY KEY,
	account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	purpose     TEXT NOT NULL,
	code_hash   TEXT NOT NULL,
	salt        TEXT NOT NULL,
	attempts    INTEGER NOT NULL DEFAULT 0,
	expires_at  TEXT NOT NULL,
	consumed_at TEXT,
	created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX IF NOT EXISTS auth_codes_lookup_idx ON auth_codes(account_id, purpose, created_at);
`
}

// SQLiteRepository persists accounts, sessions, and auth codes in SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository ensures the accounts schema exists and returns a SQLiteRepository.
func NewSQLiteRepository(db *sql.DB) (*SQLiteRepository, error) {
	if _, err := db.Exec(accountsSchema()); err != nil {
		return nil, err
	}
	return &SQLiteRepository{db: db}, nil
}

func formatTimePtr(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339Nano), Valid: true}
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(raw string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, raw)
}

func parseTimePtr(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil
	}
	t, err := parseTime(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func nullStringPtr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func (r *SQLiteRepository) Create(account domain.Account) (domain.Account, error) {
	id, err := randomID()
	if err != nil {
		return domain.Account{}, err
	}
	account.ID = id
	isDemo := 0
	if account.IsDemo {
		isDemo = 1
	}

	_, err = r.db.Exec(
		`INSERT INTO accounts (id, email, phone, pin_hash, contractor_id, email_verified_at, failed_logins, locked_until, is_demo)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		account.ID,
		account.Email,
		account.Phone,
		account.PINHash,
		nullStringPtr(account.ContractorID),
		formatTimePtr(account.EmailVerifiedAt),
		account.FailedLogins,
		formatTimePtr(account.LockedUntil),
		isDemo,
	)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "UNIQUE constraint failed") {
			switch {
			case strings.Contains(msg, "accounts.email"):
				return domain.Account{}, domain.ErrEmailTaken
			case strings.Contains(msg, "accounts.phone"):
				return domain.Account{}, domain.ErrPhoneTaken
			case strings.Contains(msg, "accounts.contractor_id"):
				return domain.Account{}, domain.ErrAlreadyLinked
			}
			// Fallback check if SQLite omits column name in constraint error.
			if _, found, _ := r.FindByEmail(account.Email); found {
				return domain.Account{}, domain.ErrEmailTaken
			}
			if _, found, _ := r.FindByPhone(account.Phone); found {
				return domain.Account{}, domain.ErrPhoneTaken
			}
			return domain.Account{}, domain.ErrAlreadyLinked
		}
		return domain.Account{}, err
	}
	return domain.CloneAccount(account), nil
}

func (r *SQLiteRepository) scanAccount(row *sql.Row) (domain.Account, bool, error) {
	var account domain.Account
	var contractorID, verifiedAt, lockedUntil sql.NullString
	var isDemo int
	err := row.Scan(
		&account.ID,
		&account.Email,
		&account.Phone,
		&account.PINHash,
		&contractorID,
		&verifiedAt,
		&account.FailedLogins,
		&lockedUntil,
		&isDemo,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, false, nil
	}
	if err != nil {
		return domain.Account{}, false, err
	}
	if contractorID.Valid {
		account.ContractorID = domain.CopyText(&contractorID.String)
	}
	if account.EmailVerifiedAt, err = parseTimePtr(verifiedAt); err != nil {
		return domain.Account{}, false, err
	}
	if account.LockedUntil, err = parseTimePtr(lockedUntil); err != nil {
		return domain.Account{}, false, err
	}
	account.IsDemo = isDemo != 0
	return account, true, nil
}

const accountSelectCols = `id, email, phone, pin_hash, contractor_id, email_verified_at, failed_logins, locked_until, is_demo`

func (r *SQLiteRepository) Find(id string) (domain.Account, bool, error) {
	return r.scanAccount(r.db.QueryRow(`SELECT `+accountSelectCols+` FROM accounts WHERE id = ?`, id))
}

func (r *SQLiteRepository) FindByEmail(email string) (domain.Account, bool, error) {
	return r.scanAccount(r.db.QueryRow(`SELECT `+accountSelectCols+` FROM accounts WHERE email = ?`, email))
}

func (r *SQLiteRepository) FindByPhone(phone string) (domain.Account, bool, error) {
	return r.scanAccount(r.db.QueryRow(`SELECT `+accountSelectCols+` FROM accounts WHERE phone = ?`, phone))
}

func (r *SQLiteRepository) LinkContractor(accountID, contractorID string) error {
	result, err := r.db.Exec(
		`UPDATE accounts SET contractor_id = ? WHERE id = ? AND contractor_id IS NULL`,
		contractorID, accountID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return domain.ErrAlreadyLinked
		}
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrAlreadyLinked
	}
	return nil
}

func (r *SQLiteRepository) UpdateSecurity(account domain.Account) error {
	result, err := r.db.Exec(
		`UPDATE accounts SET pin_hash = ?, failed_logins = ?, locked_until = ?, email_verified_at = ? WHERE id = ?`,
		account.PINHash,
		account.FailedLogins,
		formatTimePtr(account.LockedUntil),
		formatTimePtr(account.EmailVerifiedAt),
		account.ID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errNotFound
	}
	return nil
}

func (r *SQLiteRepository) SaveSession(session domain.Session, now time.Time) error {
	_, _ = r.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, formatTime(now))
	_, err := r.db.Exec(
		`INSERT INTO sessions (token_hash, account_id, expires_at) VALUES (?, ?, ?)`,
		session.TokenHash, session.AccountID, formatTime(session.ExpiresAt),
	)
	return err
}

func (r *SQLiteRepository) FindSession(tokenHash string, now time.Time) (domain.Session, bool, error) {
	var session domain.Session
	var expiresRaw string
	err := r.db.QueryRow(
		`SELECT token_hash, account_id, expires_at FROM sessions WHERE token_hash = ?`,
		tokenHash,
	).Scan(&session.TokenHash, &session.AccountID, &expiresRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Session{}, false, nil
	}
	if err != nil {
		return domain.Session{}, false, err
	}
	expiresAt, err := parseTime(expiresRaw)
	if err != nil {
		return domain.Session{}, false, err
	}
	if !now.Before(expiresAt) {
		return domain.Session{}, false, nil
	}
	session.ExpiresAt = expiresAt
	return session, true, nil
}

func (r *SQLiteRepository) DeleteSession(tokenHash string) error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (r *SQLiteRepository) DeleteSessionsFor(accountID string) error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE account_id = ?`, accountID)
	return err
}

func (r *SQLiteRepository) SaveCode(code domain.Code) error {
	id, err := randomID()
	if err != nil {
		return err
	}
	code.ID = id
	nowStr := formatTime(code.CreatedAt)

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM auth_codes WHERE expires_at <= ?`, nowStr); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE auth_codes SET consumed_at = ? WHERE account_id = ? AND purpose = ? AND consumed_at IS NULL`,
		nowStr, code.AccountID, code.Purpose,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO auth_codes (id, account_id, purpose, code_hash, salt, attempts, expires_at, consumed_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		code.ID, code.AccountID, code.Purpose, code.CodeHash, code.Salt,
		code.Attempts, formatTime(code.ExpiresAt), formatTimePtr(code.ConsumedAt), nowStr,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLiteRepository) LatestCode(accountID, purpose string) (domain.Code, bool, error) {
	var code domain.Code
	var expiresRaw, createdRaw string
	var consumedRaw sql.NullString
	err := r.db.QueryRow(
		`SELECT id, account_id, purpose, code_hash, salt, attempts, expires_at, consumed_at, created_at
		 FROM auth_codes
		 WHERE account_id = ? AND purpose = ?
		 ORDER BY created_at DESC, rowid DESC
		 LIMIT 1`,
		accountID, purpose,
	).Scan(
		&code.ID, &code.AccountID, &code.Purpose, &code.CodeHash, &code.Salt,
		&code.Attempts, &expiresRaw, &consumedRaw, &createdRaw,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Code{}, false, nil
	}
	if err != nil {
		return domain.Code{}, false, err
	}
	if code.ExpiresAt, err = parseTime(expiresRaw); err != nil {
		return domain.Code{}, false, err
	}
	if code.CreatedAt, err = parseTime(createdRaw); err != nil {
		return domain.Code{}, false, err
	}
	if code.ConsumedAt, err = parseTimePtr(consumedRaw); err != nil {
		return domain.Code{}, false, err
	}
	return code, true, nil
}

func (r *SQLiteRepository) UpdateCode(code domain.Code) error {
	result, err := r.db.Exec(
		`UPDATE auth_codes SET attempts = ?, consumed_at = ? WHERE id = ?`,
		code.Attempts, formatTimePtr(code.ConsumedAt), code.ID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errNotFound
	}
	return nil
}
