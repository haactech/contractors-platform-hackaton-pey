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

// Onboarding es la respuesta que el formulario ya sabe leer.
type Onboarding struct {
	ID string `json:"id"`
	Input
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
