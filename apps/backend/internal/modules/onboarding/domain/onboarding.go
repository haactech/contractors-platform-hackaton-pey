package domain

// Client es la persona indicada en el formulario.
type Client struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// Input es el JSON que envía el formulario de alta.
type Input struct {
	Name             string  `json:"name"`
	Profession       string  `json:"profession"`
	Client           Client  `json:"client"`
	TaxRegime        *string `json:"taxRegime,omitempty"`
	BusinessName     string  `json:"businessName"`
	BusinessActivity string  `json:"businessActivity"`
	RFC              string  `json:"rfc"`
	// CLABE es la cuenta interbancaria de 18 dígitos donde el negocio recibe sus pagos.
	// Pey deposita ahí; la plataforma no custodia el dinero.
	CLABE string `json:"clabe"`
}

// Onboarding es la respuesta pública del alta. No incluye la CLABE completa ni
// el bankAccountId de Pey: solo últimos 4 dígitos y estado de activación.
type Onboarding struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Profession       string  `json:"profession"`
	Client           Client  `json:"client"`
	TaxRegime        *string `json:"taxRegime,omitempty"`
	BusinessName     string  `json:"businessName"`
	BusinessActivity string  `json:"businessActivity"`
	RFC              string  `json:"rfc"`
	ClabeLast4       string  `json:"clabeLast4"`
	CodiStatus       string  `json:"codiStatus"`
	BankReady        bool    `json:"bankReady"`
}

// ProfilePatch actualiza de forma parcial el perfil del contratista y su negocio/CLABE.
type ProfilePatch struct {
	Name             *string `json:"name,omitempty"`
	Profession       *string `json:"profession,omitempty"`
	BusinessName     *string `json:"businessName,omitempty"`
	BusinessActivity *string `json:"businessActivity,omitempty"`
	RFC              *string `json:"rfc,omitempty"`
	TaxRegime        *string `json:"taxRegime,omitempty"`
	CLABE            *string `json:"clabe,omitempty"`
}
