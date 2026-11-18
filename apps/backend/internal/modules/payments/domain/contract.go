package domain

import (
	"contractors-platform/backend/internal/modules/payments/definitions"
	"encoding/json"
	"errors"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrVendorNotFound avisa que el alta no tiene al vendedor.
var ErrVendorNotFound = errors.New("vendedor ausente")

// ErrCollector avisa que Pey no entregó un cobro usable.
var ErrCollector = errors.New("pey no entregó el cobro")

// Build normaliza el pedido y, si las reglas pasan, devuelve el cobro sin id.
func Build(input Request) (Payment, map[string]string) {
	payment := Payment{
		VendorID:    strings.TrimSpace(input.VendorID),
		ClientID:    strings.TrimSpace(input.ClientID),
		ClientName:  strings.TrimSpace(input.ClientName),
		ClientPhone: strings.TrimSpace(input.ClientPhone),
		ClientEmail: strings.TrimSpace(input.ClientEmail),
		Concept:     strings.TrimSpace(input.Concept),
		AmountCents: input.AmountCents,
		DueDate:     strings.TrimSpace(input.DueDate),
		Method:      strings.TrimSpace(input.Method),
		Status:      definitions.StatusPending,
	}
	if fieldErrors := validate(payment); len(fieldErrors) > 0 {
		return Payment{}, fieldErrors
	}
	return payment, nil
}

func validate(payment Payment) map[string]string {
	errors := map[string]string{}
	if payment.VendorID == "" {
		errors[definitions.FieldVendor] = definitions.MessageRequired
	}
	if payment.ClientName == "" {
		errors[definitions.FieldClientName] = definitions.MessageRequired
	}
	runes := utf8.RuneCountInString(payment.Concept)
	if payment.Concept == "" || runes > definitions.ConceptMaxRunes {
		errors[definitions.FieldConcept] = definitions.MessageConcept
	}
	if payment.AmountCents <= 0 {
		errors[definitions.FieldAmount] = definitions.MessageAmount
	} else if payment.Method == definitions.MethodCodiPush && payment.AmountCents > definitions.PushMaxCents {
		errors[definitions.FieldAmount] = definitions.MessagePushAmount
	}
	if payment.Method != definitions.MethodPaymentLink && payment.Method != definitions.MethodCodiQR && payment.Method != definitions.MethodCodiPush {
		errors[definitions.FieldMethod] = definitions.MessageMethod
	}
	if payment.ClientPhone != "" && !phoneAllowed(payment.ClientPhone) {
		errors[definitions.FieldClientPhone] = definitions.MessagePhone
	}
	if payment.ClientEmail != "" && !emailAllowed(payment.ClientEmail) {
		errors[definitions.FieldClientEmail] = definitions.MessageEmail
	}
	if payment.DueDate != "" {
		if _, err := time.Parse(definitions.DueDateLayout, payment.DueDate); err != nil {
			errors[definitions.FieldDueDate] = definitions.MessageDueDate
		}
	}
	return errors
}

// Allow repite las reglas sobre un cobro ya armado para Pey.
func Allow(payment Payment) map[string]string {
	errors := validate(payment)
	if payment.BankAccountID == "" {
		errors[definitions.FieldMethod] = definitions.MessageAccount
	}
	if payment.Status != definitions.StatusPending {
		errors[definitions.FieldMethod] = definitions.MessageMethod
	}
	switch payment.Method {
	case definitions.MethodPaymentLink:
		if payment.PaymentLinkURL == "" || payment.PeyID == "" {
			errors[definitions.FieldMethod] = definitions.MessageMethod
		}
	case definitions.MethodCodiQR:
		if payment.PeyID == "" || payment.CodiQRString == "" {
			errors[definitions.FieldMethod] = definitions.MessageMethod
		}
	case definitions.MethodCodiPush:
		if payment.PeyID == "" {
			errors[definitions.FieldMethod] = definitions.MessageMethod
		}
	}
	return errors
}

// AccountReady indica si esa cuenta puede recibir el cobro.
func AccountReady(account Account) bool {
	return account.BankAccountID != "" && account.CodiStatus == definitions.CodiActive
}

// MatchClient elige al cliente del alta que corresponde al pedido.
func MatchClient(clients []Client, name, phone string) (Client, bool) {
	name = strings.TrimSpace(name)
	var found []Client
	for _, client := range clients {
		if !strings.EqualFold(strings.TrimSpace(client.Name), name) {
			continue
		}
		if !samePhone(client.Phone, phone) {
			continue
		}
		found = append(found, client)
	}
	if len(found) != 1 {
		return Client{}, false
	}
	return found[0], true
}

// ApplyClient copia el cliente guardado. El teléfono del pedido no se conserva.
func ApplyClient(payment Payment, client Client) Payment {
	payment.ClientID = client.ID
	payment.ClientName = strings.TrimSpace(client.Name)
	payment.ClientPhone = strings.TrimSpace(client.Phone)
	payment.ClientEmail = strings.TrimSpace(client.Email)
	return payment
}

// DirectPhoneReady rechaza un cobro directo cuyo teléfono guardado no tiene 10 dígitos.
func DirectPhoneReady(payment Payment) map[string]string {
	if payment.Method != definitions.MethodCodiPush {
		return nil
	}
	if _, ok := pushPhone(payment.ClientPhone); !ok {
		return map[string]string{definitions.FieldClientPhone: definitions.MessagePushPhone}
	}
	return nil
}

// Encode arma la ruta y el cuerpo de Pey para el método del cobro.
func Encode(payment Payment, links Links) (string, []byte, bool) {
	metadata := map[string]string{
		definitions.PeyMetadataPaymentID: payment.ID,
		definitions.PeyMetadataConcept:   payment.Concept,
	}
	amount := float64(payment.AmountCents) / 100
	switch payment.Method {
	case definitions.MethodPaymentLink:
		if !httpsURL(links.Success) || !httpsURL(links.Back) {
			return "", nil, false
		}
		body, err := json.Marshal(map[string]any{
			definitions.PeyAmount:          amount,
			definitions.PeySuccessURL:      links.Success,
			definitions.PeyBackURL:         links.Back,
			definitions.PeyClientReference: payment.ID,
			definitions.PeyMetadata:        metadata,
		})
		if err != nil {
			return "", nil, false
		}
		return definitions.PeyPathPaymentLink, body, true
	case definitions.MethodCodiQR:
		if payment.BankAccountID == "" {
			return "", nil, false
		}
		body, err := json.Marshal(map[string]any{
			definitions.PeyAmount:        amount,
			definitions.PeyConcept:       payment.Concept,
			definitions.PeyExpiration:    definitions.QRExpirationSeconds,
			definitions.PeyBankAccountID: payment.BankAccountID,
			definitions.PeyMetadata:      metadata,
		})
		if err != nil {
			return "", nil, false
		}
		return definitions.PeyPathCodiQR, body, true
	case definitions.MethodCodiPush:
		phone, ok := pushPhone(payment.ClientPhone)
		if !ok || payment.BankAccountID == "" || payment.AmountCents > definitions.PushMaxCents {
			return "", nil, false
		}
		body, err := json.Marshal(map[string]any{
			definitions.PeyAmount:        amount,
			definitions.PeyPhoneNumber:   phone,
			definitions.PeyConcept:       payment.Concept,
			definitions.PeyBankAccountID: payment.BankAccountID,
			definitions.PeyExpiration:    definitions.PushExpirationSeconds,
			definitions.PeyMetadata:      metadata,
		})
		if err != nil {
			return "", nil, false
		}
		return definitions.PeyPathCodiPush, body, true
	default:
		return "", nil, false
	}
}

// Decode lee la respuesta de Pey según el método.
func Decode(method string, payload []byte) (Result, bool) {
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return Result{}, false
	}
	if method == definitions.MethodCodiPush {
		id, _ := body[definitions.PeyTransactionID].(string)
		if id == "" {
			return Result{}, false
		}
		return Result{PeyID: id}, true
	}
	id, _ := body[definitions.PeyID].(string)
	if id == "" {
		return Result{}, false
	}
	switch method {
	case definitions.MethodPaymentLink:
		link, _ := body[definitions.PeyURL].(string)
		if link == "" {
			return Result{}, false
		}
		return Result{PeyID: id, LinkURL: link}, true
	case definitions.MethodCodiQR:
		qr, _ := body[definitions.PeyQRString].(string)
		if qr == "" {
			return Result{}, false
		}
		image, _ := body[definitions.PeyQRImage].(string)
		return Result{PeyID: id, QR: qr, Image: image}, true
	default:
		return Result{}, false
	}
}

// Attach copia el resultado de Pey sobre el cobro.
func Attach(payment Payment, result Result) Payment {
	payment.PeyID = result.PeyID
	payment.PaymentLinkURL = result.LinkURL
	payment.CodiQRString = result.QR
	payment.CodiQRImage = result.Image
	return payment
}

// View es el JSON que la pantalla ya espera.
type View struct {
	ID             string `json:"id"`
	VendorID       string `json:"onboardingId"`
	ClientID       string `json:"clientId,omitempty"`
	ClientName     string `json:"clientName"`
	ClientPhone    string `json:"clientPhone,omitempty"`
	ClientEmail    string `json:"clientEmail,omitempty"`
	Concept        string `json:"concept"`
	AmountCents    int64  `json:"amountCents"`
	DueDate        string `json:"dueDate,omitempty"`
	Method         string `json:"method"`
	Status         string `json:"status"`
	PaymentLinkURL string `json:"paymentLinkUrl,omitempty"`
	CodiQRID       string `json:"codiQrId,omitempty"`
	CodiQRString   string `json:"codiQrString,omitempty"`
	CodiQRImage    string `json:"codiQrImage,omitempty"`
	ReminderCount  int    `json:"reminderCount"`
	CreatedAt      string `json:"createdAt"`
}

// Present proyecta el cobro guardado. El bank id no sale en el JSON.
func Present(payment Payment) View {
	view := View{
		ID:             payment.ID,
		VendorID:       payment.VendorID,
		ClientID:       payment.ClientID,
		ClientName:     payment.ClientName,
		ClientPhone:    payment.ClientPhone,
		ClientEmail:    payment.ClientEmail,
		Concept:        payment.Concept,
		AmountCents:    payment.AmountCents,
		DueDate:        payment.DueDate,
		Method:         payment.Method,
		Status:         payment.Status,
		PaymentLinkURL: payment.PaymentLinkURL,
		CodiQRString:   payment.CodiQRString,
		CodiQRImage:    payment.CodiQRImage,
		ReminderCount:  payment.ReminderCount,
		CreatedAt:      payment.CreatedAt,
	}
	if payment.Method == definitions.MethodCodiQR {
		view.CodiQRID = payment.PeyID
	}
	return view
}

// LinksReady indica si las URLs de retorno son HTTPS usables por Pey.
func LinksReady(links Links) bool {
	return httpsURL(links.Success) && httpsURL(links.Back)
}

func VendorMissingMessage() string  { return definitions.MessageVendorMissing }
func PaymentMissingMessage() string { return definitions.MessagePaymentMissing }
func ClientMissingMessage() string  { return definitions.MessageClientMissing }
func AccountMessage() string        { return definitions.MessageAccount }
func LinksMessage() string          { return definitions.MessageLinks }
func SaveMessage() string          { return definitions.MessageSave }
func PeyMessage() string           { return definitions.MessagePey }
func ReviewMessage() string        { return definitions.MessageReview }
func JSONMessage() string          { return definitions.MessageJSON }
func OneObjectMessage() string     { return definitions.MessageOneObject }
func ContentTypeMessage() string   { return definitions.MessageContentType }
func TooLargeMessage() string      { return definitions.MessageTooLarge }
func ClientField() string          { return definitions.FieldClientName }
func ClientIDField() string        { return definitions.FieldClientID }
func MethodField() string          { return definitions.FieldMethod }

func samePhone(stored, requested string) bool {
	if strings.TrimSpace(requested) == "" {
		return true
	}
	left, right := digits(stored), digits(requested)
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	if len(right) >= 10 && strings.HasSuffix(left, right) {
		return true
	}
	return len(left) >= 10 && strings.HasSuffix(right, left)
}

func pushPhone(phone string) (string, bool) {
	value := digits(phone)
	if len(value) > definitions.PushPhoneDigits && strings.HasPrefix(value, "52") {
		value = value[2:]
	}
	if len(value) > definitions.PushPhoneDigits {
		value = value[len(value)-definitions.PushPhoneDigits:]
	}
	if len(value) != definitions.PushPhoneDigits {
		return "", false
	}
	return value, true
}

func digits(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if char >= '0' && char <= '9' {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func phoneAllowed(phone string) bool {
	count := len(digits(phone))
	if count < definitions.PhoneMinDigits || count > definitions.PhoneMaxDigits {
		return false
	}
	for _, char := range phone {
		switch {
		case char >= '0' && char <= '9':
		case char == '+' || char == ' ' || char == '(' || char == ')' || char == '-':
		default:
			return false
		}
	}
	return true
}

func emailAllowed(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}

func httpsURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}
