package domain

import "context"

// Account es la cuenta Pey del vendedor leída de su alta.
type Account struct {
	BankAccountID string
	CodiStatus    string
}

// Client es un cliente guardado para ese vendedor.
type Client struct {
	ID    string
	Name  string
	Phone string
	Email string
}

// VendorSource lee la cuenta del alta. No la modifica y no lee clientes.
type VendorSource interface {
	Account(vendorID string) (Account, bool, error)
}

// ClientDirectory lee los clientes de ese vendedor. No los modifica.
type ClientDirectory interface {
	Find(vendorID, clientID string) (Client, bool, error)
	List(vendorID string) ([]Client, error)
}

// Collector envía a Pey el cuerpo que el dominio ya armó.
type Collector interface {
	Collect(context.Context, string, []byte) ([]byte, error)
}

// Repository guarda el cobro después de que Pey respondió.
type Repository interface {
	Allocate() (string, error)
	Save(Payment) (Payment, error)
}

// Links son las páginas HTTPS a las que Pey devuelve al pagador.
type Links struct {
	Success string
	Back    string
}

// Request es el cobro que pide la pantalla.
type Request struct {
	VendorID    string `json:"onboardingId"`
	ClientID    string `json:"clientId,omitempty"`
	ClientName  string `json:"clientName"`
	ClientPhone string `json:"clientPhone,omitempty"`
	ClientEmail string `json:"clientEmail,omitempty"`
	Concept     string `json:"concept"`
	AmountCents int64  `json:"amountCents"`
	DueDate     string `json:"dueDate,omitempty"`
	Method      string `json:"method"`
}

// Payment es el cobro dirigido a la cuenta del vendedor.
type Payment struct {
	ID             string
	VendorID       string
	BankAccountID  string
	ClientID       string
	ClientName     string
	ClientPhone    string
	ClientEmail    string
	Concept        string
	AmountCents    int64
	DueDate        string
	Method         string
	Status         string
	PeyID          string
	PaymentLinkURL string
	CodiQRString   string
	CodiQRImage    string
	ReminderCount  int
	CreatedAt      string
}

// Result es el cobro que Pey aceptó.
type Result struct {
	PeyID   string
	LinkURL string
	QR      string
	Image   string
}
