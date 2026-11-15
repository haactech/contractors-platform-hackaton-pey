// Package definitions stores constants, limits, and human-facing Spanish
// messages for the accounts module.
package definitions

import "time"

const (
	SessionCookieName = "praxis_session"

	SessionTTL      = 30 * 24 * time.Hour
	CodeTTL         = 15 * time.Minute
	LockoutDuration = 15 * time.Minute
	ResendCooldown  = 60 * time.Second

	MaxFailedLogins = 5
	MaxCodeAttempts = 5

	PINLength        = 6
	CodeLength       = 6
	PBKDF2Iterations = 100_000
	MaxRunes         = 255
	MXPhoneDigits    = 10
)

const (
	PurposeEmailVerification = "email_verification"
	PurposePINRecovery       = "pin_recovery"
)

const (
	FieldEmail      = "email"
	FieldPhone      = "phone"
	FieldPIN        = "pin"
	FieldIdentifier = "identifier"
	FieldCode       = "code"
)

const (
	ErrorCodeUnauthenticated = "unauthenticated"
	ErrorCodeEmailUnverified = "email_unverified"
	ErrorCodeAccountLocked   = "account_locked"
	ErrorCodeDemoProtected   = "demo_protected"
)

const (
	DemoEmail        = "demo@praxis.mx"
	DemoPhone        = "+525500000000"
	DemoPhoneDisplay = "+52 55 0000 0000"
	DemoPIN          = "123456"
)

const (
	MessageRequired          = "Este campo es obligatorio."
	MessageMaxRunes          = "Usa como máximo 255 caracteres."
	MessageEmail             = "Ingresa un correo válido."
	MessagePhone             = "Ingresa un teléfono de 10 dígitos."
	MessagePIN               = "El PIN debe tener exactamente 6 dígitos."
	MessageIdentifier        = "Ingresa un correo o un teléfono de 10 dígitos."
	MessageCodeFormat        = "Ingresa el código de 6 dígitos."
	MessageEmailTaken        = "Este correo ya tiene una cuenta."
	MessagePhoneTaken        = "Este teléfono ya está registrado."
	MessageInvalidCredential = "Correo, teléfono o PIN incorrectos."
	MessageUnauthenticated   = "Inicia sesión para continuar."
	MessageEmailUnverified   = "Verifica tu correo para continuar."
	MessageInvalidCode       = "El código es inválido o expiró."
	MessageResendTooSoon     = "Espera un minuto antes de pedir otro código."
	MessageDemoProtected     = "No puedes cambiar el PIN de la cuenta demo."
	MessageInvalidJSON       = "El JSON es inválido o contiene campos desconocidos."
	MessageSingleJSON        = "Envía un único objeto JSON."
	MessageContentType       = "Usa Content-Type application/json."
	MessageBodyTooLarge      = "El cuerpo de la solicitud es demasiado grande."
	MessageValidation        = "Revisa los campos del formulario."
	MessageServerFailure     = "No fue posible completar la solicitud."
	MessageNotFound          = "No se encontró el registro."
)
