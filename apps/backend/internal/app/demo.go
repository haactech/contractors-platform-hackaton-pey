package app

import (
	"context"
	"contractors-platform/backend/internal/modules/accounts/application"
	"contractors-platform/backend/internal/modules/onboarding"
)

func demoContractorInput() onboarding.Input {
	taxRegime := "RESICO"
	return onboarding.Input{
		Name:       "Ana García López",
		Profession: "Electricista",
		Client: onboarding.Client{
			Name:  "Carlos Mendoza",
			Phone: "5512345678",
			Email: "carlos@empresa.mx",
		},
		TaxRegime:        &taxRegime,
		BusinessName:     "Servicios Eléctricos García",
		BusinessActivity: "Electricidad y plomería",
		RFC:              "GALA900101AB1",
		CLABE:            "002180000000000012",
	}
}

func seedDemo(ctx context.Context, demo *application.Demo, seedContractor func(onboarding.Input) (string, error)) error {
	return demo.EnsureDemoAccount(ctx, func() (string, error) {
		return seedContractor(demoContractorInput())
	})
}
