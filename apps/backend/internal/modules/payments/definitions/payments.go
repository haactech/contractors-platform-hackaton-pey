// Package definitions guarda los nombres, límites y frases del cobro.
package definitions

const (
	FieldVendor      = "onboardingId"
	FieldClientID    = "clientId"
	FieldClientName  = "clientName"
	FieldClientPhone = "clientPhone"
	FieldClientEmail = "clientEmail"
	FieldConcept     = "concept"
	FieldAmount      = "amountCents"
	FieldDueDate     = "dueDate"
	FieldMethod      = "method"
)

const (
	MessageRequired      = "Este campo es obligatorio."
	MessageConcept       = "Usa entre 1 y 40 caracteres."
	MessageAmount        = "Ingresa un monto mayor a $0.00."
	MessagePushAmount    = "Ingresa un monto de hasta $1,000,000.00."
	MessageMethod        = "Elige un método de cobro."
	MessagePhone         = "Ingresa un teléfono de 7 a 15 dígitos."
	MessagePushPhone     = "El cliente no tiene un teléfono de 10 dígitos."
	MessageEmail         = "Ingresa un email válido."
	MessageDueDate       = "Elige una fecha válida."
	MessageClientMissing = "Este cliente no está registrado."
	MessageAccount       = "La cuenta todavía no puede recibir CoDi."
	MessageVendorMissing = "No se encontró el vendedor."
	MessageSave          = "No fue posible crear el cobro."
	MessagePey           = "No fue posible crear el cobro en Pey."
	MessageReview        = "Revisa los campos del formulario."
	MessageJSON          = "El JSON es inválido o contiene campos desconocidos."
	MessageOneObject     = "Envía un único objeto JSON."
	MessageContentType   = "Usa Content-Type application/json."
	MessageTooLarge      = "El cuerpo de la solicitud es demasiado grande."
)

const (
	MethodPaymentLink = "PAYMENT_LINK"
	MethodCodiQR      = "CODI_QR"
	MethodCodiPush    = "CODI_PUSH"
	StatusPending     = "PENDING_PAYMENT"
	CodiActive        = "ACTIVE"
)

const (
	PeyAmount             = "amount"
	PeySuccessURL         = "successUrl"
	PeyBackURL            = "backUrl"
	PeyClientReference    = "clientReferenceId"
	PeyMetadata           = "metadata"
	PeyMetadataPaymentID  = "paymentId"
	PeyMetadataConcept    = "concept"
	PeyConcept            = "concept"
	PeyExpiration         = "expiration"
	PeyBankAccountID      = "bankAccountId"
	PeyPhoneNumber        = "phoneNumber"
	PeyID                 = "id"
	PeyTransactionID      = "transactionId"
	PeyURL                = "url"
	PeyQRString           = "qrString"
	PeyQRImage            = "qrImage"
	PeyPathPaymentLink    = "/payment-link/create"
	PeyPathCodiQR         = "/codi/qr"
	PeyPathCodiPush       = "/codi/pn"
	ConceptMaxRunes       = 40
	PhoneMinDigits        = 7
	PhoneMaxDigits        = 15
	PushPhoneDigits       = 10
	QRExpirationSeconds   = 86400
	PushExpirationSeconds = 360
	PushMaxCents          = 100_000_000
)

const DueDateLayout = "2006-01-02"
