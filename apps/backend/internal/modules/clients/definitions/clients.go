// Package definitions guarda los nombres, límites y frases del cliente.
package definitions

import "regexp"

const (
	FieldEntityType    = "entityType"
	FieldName          = "name"
	FieldPhone         = "phone"
	FieldEmail         = "email"
	FieldLegalName     = "legalName"
	FieldTradeName     = "tradeName"
	FieldRFC           = "rfc"
	FieldActivity      = "activity"
	FieldTaxRegime     = "taxRegime"
	FieldFiscalAddress = "fiscalAddress"
)

const (
	MessageRequired         = "Este campo es obligatorio."
	MessageName             = "Usa un máximo de 255 caracteres."
	MessagePhone            = "Escribe un teléfono de 10 dígitos."
	MessagePhoneOnboarding  = "Ingresa un teléfono de 7 a 15 dígitos."
	MessageEmail            = "Escribe un correo válido."
	MessageLegalName        = "Ingresa la razón social o nombre del negocio."
	MessageRFC              = "Escribe un RFC válido de 12 (empresa) o 13 caracteres (persona física)."
	MessagePhoneTaken       = "Este teléfono ya está registrado."
	MessageVendorMissing    = "No se encontró el vendedor."
	MessageClientMissing    = "No se encontró el cliente."
	MessageSave             = "No fue posible guardar el cliente."
	MessageReview           = "Revisa los campos del formulario."
	MessageJSON             = "El JSON es inválido o contiene campos desconocidos."
	MessageOneObject        = "Envía un único objeto JSON."
	MessageContentType      = "Usa Content-Type application/json."
	MessageTooLarge         = "El cuerpo de la solicitud es demasiado grande."
)

const (
	EntityIndividual = "individual"
	EntityCompany    = "company"
	MaxRunes         = 255
	// PhoneDigits is the Mexican national length for dashboard writes and for
	// payments CODI_PUSH after stripping a leading 52 country code.
	PhoneDigits = 10
	// Onboarding first-client phones mirror onboarding.PhoneMin/MaxDigits (7–15).
	// A saved phone whose PhoneKey is not 10 digits is rejected by CODI_PUSH.
	OnboardingPhoneMinDigits = 7
	OnboardingPhoneMaxDigits = 15
)

var RFCPattern = regexp.MustCompile(`^[A-ZÑ&]{3,4}\d{6}[A-Z0-9]{3}$`)
