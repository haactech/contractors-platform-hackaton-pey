package infrastructure

import (
	"database/sql"
	"errors"
)

// SQLiteVendors lee si el vendedor existe. No modifica su alta.
type SQLiteVendors struct {
	db *sql.DB
}

// NewSQLiteVendors usa la base de vendedores que el alta ya abrió.
func NewSQLiteVendors(db *sql.DB) *SQLiteVendors {
	return &SQLiteVendors{db: db}
}

// Exists indica si hay una persona con ese id.
func (vendors *SQLiteVendors) Exists(id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	var one int
	err := vendors.db.QueryRow(`SELECT 1 FROM vendors WHERE id = ?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
