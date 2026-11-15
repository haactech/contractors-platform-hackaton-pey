package domain

import "errors"

// ErrVendorNotFound avisa que el vendedor no existe.
var ErrVendorNotFound = errors.New("vendedor ausente")

// ErrClientNotFound avisa que el cliente no existe para ese vendedor.
var ErrClientNotFound = errors.New("cliente ausente")

// ErrPhoneTaken avisa que ese vendedor ya tiene el teléfono.
var ErrPhoneTaken = errors.New("teléfono tomado")

// Client es la persona o el comercio al que el vendedor cobra.
type Client struct {
	ID             string
	VendorID       string
	EntityType     string
	Name           string
	Phone          string
	Email          string
	LegalName      string
	TradeName      string
	RFC            string
	Activity       string
	TaxRegime      string
	FiscalAddress  string
	PhoneKey       string
	FromOnboarding bool
	CreatedAt      string
	UpdatedAt      string
}

// Input es el perfil que envía la pantalla.
type Input struct {
	EntityType    string `json:"entityType"`
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	LegalName     string `json:"legalName,omitempty"`
	TradeName     string `json:"tradeName,omitempty"`
	RFC           string `json:"rfc,omitempty"`
	Activity      string `json:"activity,omitempty"`
	TaxRegime     string `json:"taxRegime,omitempty"`
	FiscalAddress string `json:"fiscalAddress,omitempty"`
}

// Repository guarda los clientes de cada vendedor.
type Repository interface {
	Save(Client) (Client, error)
	Update(Client) (Client, error)
	List(vendorID string) ([]Client, error)
	Find(vendorID, clientID string) (Client, bool, error)
}

// Vendors indica si el alta del vendedor existe. No la modifica.
type Vendors interface {
	Exists(id string) (bool, error)
}
