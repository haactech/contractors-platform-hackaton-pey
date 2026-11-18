package domain

import (
	"context"
	"errors"
)

// ErrAlreadyOnboarded avisa que la cuenta ya tiene un registro de contratista.
var ErrAlreadyOnboarded = errors.New("cuenta con alta")

// ErrNotFound avisa que el registro de contratista no existe.
var ErrNotFound = errors.New("registro no encontrado")

// Owners ata cada alta a quien la pidió.
type Owners interface {
	Claimable(ctx context.Context) error
	Claim(ctx context.Context, contractorID string) error
}

// Customer es la primera persona a la que el usuario puede cobrar.
type Customer struct {
	Name  string
	Phone string
	Email string
}

// User es la cuenta creada a partir del formulario de alta.
type User struct {
	ID               string
	Name             string
	Profession       string
	BusinessName     string
	BusinessActivity string
	RFC              string
	TaxRegime        *string
	Customer         Customer
	CLABE            string
	CodiStatus       string
	BankAccountID    string
}

// BankAccount es la cuenta que Pey devolvió para una CLABE.
type BankAccount struct {
	ID     string
	Status string
}

// BankRegistrar envía el alta de la CLABE a Pey y devuelve el cuerpo de la respuesta.
type BankRegistrar interface {
	Register(context.Context, []byte) ([]byte, error)
}

// Repository guarda y recupera usuarios. No escribe clientes.
type Repository interface {
	Save(User) (User, error)
	Update(User) (User, error)
	Find(string) (User, bool)
	FindByCLABE(string) (User, bool)
	HasCLABE(string) (bool, error)
	HasOtherCLABE(clabe, excludeID string) (bool, error)
	RecordBankAccount(User) (User, error)
	Remove(id string) error
}

// ClientStore guarda y lee el primer cliente en el módulo de clientes.
type ClientStore interface {
	SaveFirst(vendorID string, customer Customer) error
	First(vendorID string) (Customer, bool, error)
	// RemoveForVendor deletes clients created for that vendor (claim rollback).
	RemoveForVendor(vendorID string) error
}
