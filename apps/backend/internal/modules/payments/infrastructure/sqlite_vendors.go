package infrastructure

import (
	"contractors-platform/backend/internal/modules/payments/domain"
	"database/sql"
	"errors"
)

// SQLiteVendors lee la cuenta del alta. No la modifica y no lee clientes.
type SQLiteVendors struct {
	vendors *sql.DB
}

// NewSQLiteVendors usa la base de vendedores que el alta ya abrió.
func NewSQLiteVendors(vendors *sql.DB) *SQLiteVendors {
	return &SQLiteVendors{vendors: vendors}
}

// Account lee bank_account_id y codi_status de ese vendedor.
func (source *SQLiteVendors) Account(vendorID string) (domain.Account, bool, error) {
	var account domain.Account
	err := source.vendors.QueryRow(`SELECT bank_account_id, codi_status FROM bank_accounts WHERE vendor_id = ?`, vendorID).
		Scan(&account.BankAccountID, &account.CodiStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, false, nil
	}
	if err != nil {
		return domain.Account{}, false, err
	}
	return account, true, nil
}
