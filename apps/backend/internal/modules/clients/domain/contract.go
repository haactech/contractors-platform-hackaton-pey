package domain

import (
	"contractors-platform/backend/internal/modules/clients/definitions"
	"net/mail"
	"strings"
	"unicode/utf8"
)

// BuildDashboard arma el cliente que captura la pantalla.
func BuildDashboard(vendorID string, input Input) (Client, map[string]string) {
	client := normalize(vendorID, input, false)
	if fieldErrors := Allow(client); len(fieldErrors) > 0 {
		return Client{}, fieldErrors
	}
	return client, nil
}

// BuildFirst arma el primer cliente que llega con el alta del vendedor.
func BuildFirst(vendorID, name, phone, email string) (Client, map[string]string) {
	client := normalize(vendorID, Input{EntityType: definitions.EntityIndividual, Name: name, Phone: phone, Email: email}, true)
	if fieldErrors := Allow(client); len(fieldErrors) > 0 {
		return Client{}, fieldErrors
	}
	return client, nil
}

// Allow repite las reglas antes de escribir.
func Allow(client Client) map[string]string {
	fieldErrors := map[string]string{}
	if client.VendorID == "" {
		fieldErrors[definitions.FieldName] = definitions.MessageRequired
	}
	if client.EntityType != definitions.EntityIndividual && client.EntityType != definitions.EntityCompany {
		fieldErrors[definitions.FieldEntityType] = definitions.MessageRequired
	}
	if client.Name == "" {
		fieldErrors[definitions.FieldName] = definitions.MessageRequired
	} else if utf8.RuneCountInString(client.Name) > definitions.MaxRunes {
		fieldErrors[definitions.FieldName] = definitions.MessageName
	}
	if client.FromOnboarding {
		if !onboardingPhone(client.Phone) {
			fieldErrors[definitions.FieldPhone] = definitions.MessagePhone
		}
	} else if len(digits(client.Phone)) != definitions.PhoneDigits {
		fieldErrors[definitions.FieldPhone] = definitions.MessagePhone
	}
	if client.Email == "" {
		fieldErrors[definitions.FieldEmail] = definitions.MessageRequired
	} else if !emailAllowed(client.Email) {
		fieldErrors[definitions.FieldEmail] = definitions.MessageEmail
	}
	if client.EntityType == definitions.EntityCompany && client.LegalName == "" {
		fieldErrors[definitions.FieldLegalName] = definitions.MessageLegalName
	}
	checkMax(fieldErrors, definitions.FieldLegalName, client.LegalName)
	checkMax(fieldErrors, definitions.FieldTradeName, client.TradeName)
	checkMax(fieldErrors, definitions.FieldActivity, client.Activity)
	checkMax(fieldErrors, definitions.FieldTaxRegime, client.TaxRegime)
	checkMax(fieldErrors, definitions.FieldFiscalAddress, client.FiscalAddress)
	if client.RFC != "" && !definitions.RFCPattern.MatchString(client.RFC) {
		fieldErrors[definitions.FieldRFC] = definitions.MessageRFC
	}
	return fieldErrors
}

// PhoneKey reduce el teléfono a la clave con la que se detecta un duplicado.
func PhoneKey(phone string) string {
	value := digits(phone)
	if len(value) > definitions.PhoneDigits && strings.HasPrefix(value, "52") {
		value = value[2:]
	}
	if len(value) > definitions.PhoneDigits {
		value = value[len(value)-definitions.PhoneDigits:]
	}
	return value
}

// SamePhone indica si dos teléfonos son el mismo número mexicano.
func SamePhone(left, right string) bool {
	a, b := PhoneKey(left), PhoneKey(right)
	return a != "" && a == b
}

func normalize(vendorID string, input Input, fromOnboarding bool) Client {
	phone := strings.TrimSpace(input.Phone)
	if !fromOnboarding {
		phone = dashboardPhone(phone)
	}
	return Client{
		VendorID:       strings.TrimSpace(vendorID),
		EntityType:     strings.TrimSpace(input.EntityType),
		Name:           strings.TrimSpace(input.Name),
		Phone:          phone,
		Email:          strings.TrimSpace(input.Email),
		LegalName:      strings.TrimSpace(input.LegalName),
		TradeName:      strings.TrimSpace(input.TradeName),
		RFC:            strings.ToUpper(strings.TrimSpace(input.RFC)),
		Activity:       strings.TrimSpace(input.Activity),
		TaxRegime:      strings.TrimSpace(input.TaxRegime),
		FiscalAddress:  strings.TrimSpace(input.FiscalAddress),
		PhoneKey:       PhoneKey(phone),
		FromOnboarding: fromOnboarding,
	}
}

func dashboardPhone(phone string) string {
	value := digits(phone)
	if len(value) > definitions.PhoneDigits && strings.HasPrefix(value, "52") {
		value = value[2:]
	}
	if len(value) > definitions.PhoneDigits {
		value = value[:definitions.PhoneDigits]
	}
	return value
}

func onboardingPhone(phone string) bool {
	count := len(digits(phone))
	if count < 7 || count > 15 {
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
	return phone != ""
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

func emailAllowed(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}

func checkMax(fieldErrors map[string]string, field, value string) {
	if utf8.RuneCountInString(value) > definitions.MaxRunes {
		fieldErrors[field] = definitions.MessageName
	}
}

// View es el JSON público del cliente.
type View struct {
	ClientID      string `json:"clientId"`
	VendorID      string `json:"vendorId"`
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
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

// Present proyecta el cliente guardado.
func Present(client Client) View {
	return View{
		ClientID: client.ID, VendorID: client.VendorID, EntityType: client.EntityType,
		Name: client.Name, Phone: client.Phone, Email: client.Email,
		LegalName: client.LegalName, TradeName: client.TradeName, RFC: client.RFC,
		Activity: client.Activity, TaxRegime: client.TaxRegime, FiscalAddress: client.FiscalAddress,
		CreatedAt: client.CreatedAt, UpdatedAt: client.UpdatedAt,
	}
}

func VendorMissingMessage() string { return definitions.MessageVendorMissing }
func ClientMissingMessage() string { return definitions.MessageClientMissing }
func SaveMessage() string          { return definitions.MessageSave }
func ReviewMessage() string        { return definitions.MessageReview }
func JSONMessage() string          { return definitions.MessageJSON }
func OneObjectMessage() string     { return definitions.MessageOneObject }
func ContentTypeMessage() string   { return definitions.MessageContentType }
func TooLargeMessage() string      { return definitions.MessageTooLarge }
func PhoneTakenMessage() string    { return definitions.MessagePhoneTaken }
func PhoneField() string           { return definitions.FieldPhone }
