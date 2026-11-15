package domain

import (
	"contractors-platform/backend/internal/modules/onboarding/definitions"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"
)

// ErrCLABETaken avisa al acto que la CLABE ya pertenece a otro usuario.
var ErrCLABETaken = errors.New("clabe tomada")

// Build normaliza el formulario y, si las reglas pasan, devuelve el usuario.
func Build(input Input) (User, map[string]string) {
	normalized := normalize(input)
	user := User{
		Name:             normalized.Name,
		Profession:       normalized.Profession,
		BusinessName:     normalized.BusinessName,
		BusinessActivity: normalized.BusinessActivity,
		RFC:              normalized.RFC,
		TaxRegime:        CopyText(normalized.TaxRegime),
		CLABE:            normalized.CLABE,
		Customer: Customer{
			Name:  normalized.Client.Name,
			Phone: normalized.Client.Phone,
			Email: normalized.Client.Email,
		},
		CodiStatus: NotRegisteredStatus(),
	}
	if fieldErrors := Allow(user); len(fieldErrors) > 0 {
		return User{}, fieldErrors
	}
	return user, nil
}

// Allow repite las reglas del formulario completo sobre un usuario ya armado.
func Allow(user User) map[string]string {
	errors := AllowVendor(user)
	checkText(errors, definitions.FieldClientName, user.Customer.Name)
	checkText(errors, definitions.FieldClientPhone, user.Customer.Phone)
	checkText(errors, definitions.FieldClientEmail, user.Customer.Email)
	if user.Customer.Email != "" {
		address, err := mail.ParseAddress(user.Customer.Email)
		if err != nil || address.Address != user.Customer.Email {
			errors[definitions.FieldClientEmail] = definitions.MessageEmail
		}
	}
	if user.Customer.Phone != "" && !phoneAllowed(user.Customer.Phone) {
		errors[definitions.FieldClientPhone] = definitions.MessagePhone
	}
	return errors
}

// AllowVendor valida los campos propios del contratista, negocio y CLABE sin depender del primer cliente.
func AllowVendor(user User) map[string]string {
	errors := map[string]string{}
	checkText(errors, definitions.FieldName, user.Name)
	checkText(errors, definitions.FieldProfession, user.Profession)
	checkText(errors, definitions.FieldCLABE, user.CLABE)
	checkOptionalText(errors, definitions.FieldBusinessName, user.BusinessName)
	checkOptionalText(errors, definitions.FieldBusinessActivity, user.BusinessActivity)
	checkOptionalText(errors, definitions.FieldRFC, user.RFC)
	if user.RFC != "" && !definitions.RFCPattern.MatchString(user.RFC) {
		errors[definitions.FieldRFC] = definitions.MessageRFC
	}
	if user.CLABE != "" && !ValidClabe(user.CLABE) {
		errors[definitions.FieldCLABE] = definitions.MessageCLABE
	}
	if user.TaxRegime != nil && utf8.RuneCountInString(*user.TaxRegime) > definitions.MaxRunes {
		errors[definitions.FieldTaxRegime] = definitions.MessageMaxRunes
	}
	return errors
}

// ApplyProfilePatch aplica una edición parcial al contratista, normaliza los valores y valida reglas de dominio.
func ApplyProfilePatch(existing User, patch ProfilePatch) (User, map[string]string) {
	updated := existing
	updated.TaxRegime = CopyText(existing.TaxRegime)
	if patch.Name != nil {
		updated.Name = strings.TrimSpace(*patch.Name)
	}
	if patch.Profession != nil {
		updated.Profession = strings.TrimSpace(*patch.Profession)
	}
	if patch.BusinessName != nil {
		updated.BusinessName = strings.TrimSpace(*patch.BusinessName)
	}
	if patch.BusinessActivity != nil {
		updated.BusinessActivity = strings.TrimSpace(*patch.BusinessActivity)
	}
	if patch.RFC != nil {
		updated.RFC = strings.ToUpper(strings.TrimSpace(*patch.RFC))
	}
	if patch.TaxRegime != nil {
		value := strings.TrimSpace(*patch.TaxRegime)
		if value == "" {
			updated.TaxRegime = nil
		} else {
			updated.TaxRegime = &value
		}
	}
	if patch.CLABE != nil {
		updated.CLABE = clabeSeparators.Replace(strings.TrimSpace(*patch.CLABE))
	}
	if fieldErrors := AllowVendor(updated); len(fieldErrors) > 0 {
		return User{}, fieldErrors
	}
	return updated, nil
}

// TakenCLABE informa el error del formulario cuando esa CLABE ya fue registrada.
func TakenCLABE(clabe string, taken bool) map[string]string {
	if !taken || clabe == "" {
		return map[string]string{}
	}
	return map[string]string{definitions.FieldCLABE: definitions.MessageCLABETaken}
}

// NotRegisteredStatus es el estado CoDi de un usuario que todavía no se registra en Pey.
func NotRegisteredStatus() string {
	return definitions.CodiNotRegistered
}

// EncodeRegistration arma el cuerpo que Pey espera para registrar la CLABE guardada.
func EncodeRegistration(user User) ([]byte, bool) {
	if user.CLABE == "" {
		return nil, false
	}
	name, entity, ok := accountHolder(user)
	if !ok {
		return nil, false
	}
	alias := user.BusinessName
	if alias == "" {
		alias = user.Name
	}
	body, err := json.Marshal(map[string]string{
		definitions.PeyAlias:  alias,
		definitions.PeyName:   name,
		definitions.PeyCLABE:  user.CLABE,
		definitions.PeyEntity: entity,
	})
	if err != nil {
		return nil, false
	}
	return body, true
}

// DecodeBankAccount lee el id y el estado que Pey devolvió.
func DecodeBankAccount(body []byte) (BankAccount, bool) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return BankAccount{}, false
	}
	id, _ := payload[definitions.PeyID].(string)
	status, _ := payload[definitions.PeyCodiStatus].(string)
	account := BankAccount{ID: id, Status: status}
	_, ok := AttachBankAccount(User{}, account)
	return account, ok
}

// AttachBankAccount copia el id y el estado de Pey al usuario cuando ambos son válidos.
func AttachBankAccount(user User, account BankAccount) (User, bool) {
	if account.ID == "" || !knownCodiStatus(account.Status) {
		return user, false
	}
	user.BankAccountID = account.ID
	user.CodiStatus = account.Status
	return user, true
}

func accountHolder(user User) (name, entity string, ok bool) {
	switch utf8.RuneCountInString(user.RFC) {
	case definitions.RFCLengthBusiness:
		if user.BusinessName != "" {
			return user.BusinessName, definitions.PeyEntityBusiness, true
		}
		return user.Name, definitions.PeyEntityBusiness, true
	case definitions.RFCLengthPerson, 0:
		return user.Name, definitions.PeyEntityPerson, true
	default:
		return "", "", false
	}
}

func knownCodiStatus(status string) bool {
	switch status {
	case definitions.CodiPending, definitions.CodiProcessing, definitions.CodiActive, definitions.CodiError:
		return true
	default:
		return false
	}
}

// View proyecta el usuario al JSON que el formulario ya recibe.
func View(user User) Onboarding {
	return Onboarding{
		ID: user.ID,
		Input: Input{
			Name:             user.Name,
			Profession:       user.Profession,
			Client:           Client{Name: user.Customer.Name, Phone: user.Customer.Phone, Email: user.Customer.Email},
			TaxRegime:        CopyText(user.TaxRegime),
			BusinessName:     user.BusinessName,
			BusinessActivity: user.BusinessActivity,
			RFC:              user.RFC,
			CLABE:            user.CLABE,
		},
	}
}

func normalize(input Input) Input {
	input.Name = strings.TrimSpace(input.Name)
	input.Profession = strings.TrimSpace(input.Profession)
	input.Client.Name = strings.TrimSpace(input.Client.Name)
	input.Client.Phone = strings.TrimSpace(input.Client.Phone)
	input.Client.Email = strings.TrimSpace(input.Client.Email)
	input.BusinessName = strings.TrimSpace(input.BusinessName)
	input.BusinessActivity = strings.TrimSpace(input.BusinessActivity)
	input.RFC = strings.ToUpper(strings.TrimSpace(input.RFC))
	input.CLABE = clabeSeparators.Replace(strings.TrimSpace(input.CLABE))
	if input.TaxRegime != nil {
		value := strings.TrimSpace(*input.TaxRegime)
		if value == "" {
			input.TaxRegime = nil
		} else {
			input.TaxRegime = &value
		}
	}
	return input
}

// clabeSeparators quita espacios y guiones de una CLABE pegada.
var clabeSeparators = strings.NewReplacer(" ", "", "-", "")

// ValidClabe indica si la CLABE tiene 18 dígitos y el dígito verificador correcto.
func ValidClabe(clabe string) bool {
	if !definitions.CLABEPattern.MatchString(clabe) {
		return false
	}
	sum := 0
	for index := 0; index < definitions.CLABELength-1; index++ {
		sum += int(clabe[index]-'0') * definitions.CLABEWeights[index%len(definitions.CLABEWeights)] % 10
	}
	return int(clabe[definitions.CLABELength-1]-'0') == (10-sum%10)%10
}

func checkText(errors map[string]string, field, value string) {
	if value == "" {
		errors[field] = definitions.MessageRequired
		return
	}
	checkOptionalText(errors, field, value)
}

func checkOptionalText(errors map[string]string, field, value string) {
	if value != "" && utf8.RuneCountInString(value) > definitions.MaxRunes {
		errors[field] = definitions.MessageMaxRunes
	}
}

func phoneAllowed(phone string) bool {
	if !definitions.PhonePattern.MatchString(phone) {
		return false
	}
	digits := 0
	for _, char := range phone {
		if char >= '0' && char <= '9' {
			digits++
		}
	}
	return digits >= definitions.PhoneMinDigits && digits <= definitions.PhoneMaxDigits
}

// CopyText devuelve una copia del texto para que el puntero no se comparta.
func CopyText(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
