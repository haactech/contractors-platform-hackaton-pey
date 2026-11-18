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
// Si el vendedor existe pero aún no tiene cuenta usable, found=true con cuenta vacía (422).
// Solo un id desconocido en vendors da found=false (404).
func (source *SQLiteVendors) Account(vendorID string) (domain.Account, bool, error) {
	if vendorID == "" {
		return domain.Account{}, false, nil
	}
	var one int
	err := source.vendors.QueryRow(`SELECT 1 FROM vendors WHERE id = ?`, vendorID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, false, nil
	}
	if err != nil {
		return domain.Account{}, false, err
	}

	var account domain.Account
	err = source.vendors.QueryRow(`SELECT bank_account_id, codi_status FROM bank_accounts WHERE vendor_id = ?`, vendorID).
		Scan(&account.BankAccountID, &account.CodiStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, true, nil
	}
	if err != nil {
		return domain.Account{}, false, err
	}
	return account, true, nil
}
