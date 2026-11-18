// Package definitions guarda los nombres, límites y frases del alta.
package definitions

import "regexp"

const (
	FieldName             = "name"
	FieldProfession       = "profession"
	FieldClientName       = "client.name"
	FieldClientPhone      = "client.phone"
	FieldClientEmail      = "client.email"
	FieldBusinessName     = "businessName"
	FieldBusinessActivity = "businessActivity"
	FieldRFC              = "rfc"
	FieldTaxRegime        = "taxRegime"
	FieldCLABE            = "clabe"
)

const (
	MessageRequired   = "Este campo es obligatorio."
	MessageMaxRunes   = "Usa como máximo 255 caracteres."
	MessageEmail      = "Ingresa un email válido."
	MessagePhone      = "Ingresa un teléfono de 7 a 15 dígitos."
	MessageRFC        = "Ingresa un RFC válido de 12 o 13 caracteres."
	MessageCLABE      = "Ingresa una CLABE válida de 18 dígitos."
	MessageCLABETaken = "Esta CLABE ya está registrada."
)

const (
	MaxRunes = 255
	// PhoneMin/MaxDigits apply to the nested first client on the onboarding form.
	// Dashboard client writes require exactly 10 Mexican digits (clients.PhoneDigits).
	// payments CODI_PUSH needs a 10-digit national number after stripping leading 52.
	PhoneMinDigits = 7
	PhoneMaxDigits = 15
)

const (
	CodiNotRegistered = "not_registered"
	CodiPending       = "PENDING"
	CodiProcessing    = "PROCESSING"
	CodiActive        = "ACTIVE"
	CodiError         = "ERROR"
)

// DemoBankAccountID is the synthetic Pey bank id used when seeding the demo
// contractor without calling api.pey.mx. Not a real Banxico account.
const DemoBankAccountID = "demo-bank-account"

const (
	PeyAlias      = "alias"
	PeyName       = "name"
	PeyCLABE      = "clabe"
	PeyEntity     = "entity"
	PeyTaxID      = "taxId"
	PeyID         = "id"
	PeyCodiStatus = "codiStatus"
)

const (
	PeyEntityPerson   = "PERSON"
	PeyEntityBusiness = "BUSINESS"
)

const (
	RFCLengthPerson   = 13
	RFCLengthBusiness = 12
)

var RFCPattern = regexp.MustCompile(`^[A-ZÑ&]{3,4}[0-9]{6}[A-Z0-9]{3}$`)
var PhonePattern = regexp.MustCompile(`^\+?[0-9 ()-]+$`)

// CLABE layout: 3-digit bank, 3-digit branch plaza, 11-digit account and a
// check digit computed with the weights 3, 7, 1 (Banxico).
const CLABELength = 18

var CLABEPattern = regexp.MustCompile(`^[0-9]{18}$`)
var CLABEWeights = [3]int{3, 7, 1}
