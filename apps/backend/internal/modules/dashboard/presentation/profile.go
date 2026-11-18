package presentation

import "contractors-platform/backend/internal/modules/dashboard/domain"

type ProfileResponse struct {
	ContractorID  string                    `json:"contractorId"`
	Name          string                    `json:"name"`
	Profession    string                    `json:"profession"`
	Business      BusinessResponse          `json:"business"`
	FirstCustomer FirstCustomerResponse     `json:"firstCustomer"`
	Onboarding    OnboardingSummaryResponse `json:"onboarding"`
}

type BusinessResponse struct {
	Name         string              `json:"name"`
	Activity     string              `json:"activity"`
	RFC          string              `json:"rfc"`
	TaxRegime    *string             `json:"taxRegime,omitempty"`
	TaxpayerType domain.TaxpayerType `json:"taxpayerType"`
	ClabeLast4   *string             `json:"clabeLast4"`
	CodiStatus   string              `json:"codiStatus"`
	BankReady    bool                `json:"bankReady"`
}

type FirstCustomerResponse struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

type OnboardingSummaryResponse struct {
	Status        domain.OnboardingStatus `json:"status"`
	PendingFields []domain.Field          `json:"pendingFields"`
}

func toProfileResponse(profile domain.Profile) ProfileResponse {
	return ProfileResponse{
		ContractorID: profile.ContractorID,
		Name:         profile.Name,
		Profession:   profile.Profession,
		Business: BusinessResponse{
			Name:         profile.Business.Name,
			Activity:     profile.Business.Activity,
			RFC:          profile.Business.RFC,
			TaxRegime:    profile.Business.TaxRegime,
			TaxpayerType: profile.Business.TaxpayerType,
			ClabeLast4:   profile.Business.ClabeLast4,
			CodiStatus:   profile.Business.CodiStatus,
			BankReady:    profile.Business.BankReady,
		},
		FirstCustomer: FirstCustomerResponse{
			Name:  profile.FirstCustomer.Name,
			Phone: profile.FirstCustomer.Phone,
			Email: profile.FirstCustomer.Email,
		},
		Onboarding: OnboardingSummaryResponse{
			Status:        profile.Onboarding.Status,
			PendingFields: profile.Onboarding.PendingFields,
		},
	}
}
