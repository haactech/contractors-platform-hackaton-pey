package application

import (
	"contractors-platform/backend/internal/modules/payments/domain"
	"strings"
)

// ReadPayments lee cobros guardados sin llamar a Pey.
type ReadPayments struct {
	repository domain.Repository
}

// NewReadPayments prepara la lectura pública de cobros.
func NewReadPayments(repository domain.Repository) *ReadPayments {
	return &ReadPayments{repository: repository}
}

// Find lee un cobro de ese vendedor.
func (act *ReadPayments) Find(vendorID, paymentID string) (domain.View, bool, error) {
	vendorID = strings.TrimSpace(vendorID)
	paymentID = strings.TrimSpace(paymentID)
	if vendorID == "" || paymentID == "" {
		return domain.View{}, false, nil
	}
	payment, found, err := act.repository.Find(paymentID)
	if err != nil || !found {
		return domain.View{}, found, err
	}
	if payment.VendorID != vendorID {
		return domain.View{}, false, nil
	}
	return domain.Present(payment), true, nil
}

// ListByVendor lista los cobros de un vendedor, más recientes primero.
func (act *ReadPayments) ListByVendor(vendorID string) ([]domain.View, error) {
	vendorID = strings.TrimSpace(vendorID)
	if vendorID == "" {
		return []domain.View{}, nil
	}
	payments, err := act.repository.ListByVendor(vendorID)
	if err != nil {
		return nil, err
	}
	views := make([]domain.View, 0, len(payments))
	for _, payment := range payments {
		views = append(views, domain.Present(payment))
	}
	return views, nil
}
