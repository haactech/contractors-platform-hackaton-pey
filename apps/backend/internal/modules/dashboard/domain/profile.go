// Package domain models the read views of the contractor dashboard. It has no
// knowledge of HTTP, storage or of the modules that own the source data.
package domain

import (
	"context"
	"errors"
	"unicode/utf8"
)

// ErrContractorNotFound is returned by a ProfileSource when no contractor
// matches the ID.
var ErrContractorNotFound = errors.New("contractor not found")

// RFC lengths defined by SAT: individuals (personas físicas) have 13
// characters and companies (personas morales) have 12.
const (
	rfcLengthIndividual = 13
	rfcLengthCompany    = 12
)

// TaxpayerType classifies the contractor from the length of its RFC.
type TaxpayerType string

const (
	TaxpayerTypeIndividual TaxpayerType = "individual"
	TaxpayerTypeCompany    TaxpayerType = "company"
	TaxpayerTypeUnknown    TaxpayerType = "unknown"
)

// OnboardingStatus tells whether every optional onboarding field was filled.
type OnboardingStatus string

const (
	OnboardingStatusComplete   OnboardingStatus = "complete"
	OnboardingStatusIncomplete OnboardingStatus = "incomplete"
)

// Field identifies an onboarding field still to be provided. Values match the
// JSON names of the onboarding contract.
type Field string

const (
	FieldBusinessName     Field = "businessName"
	FieldBusinessActivity Field = "businessActivity"
	FieldRFC              Field = "rfc"
	FieldTaxRegime        Field = "taxRegime"
	FieldClabe            Field = "clabe"
)

// ClabeVisibleDigits is how many trailing CLABE digits the dashboard shows.
const ClabeVisibleDigits = 4

// Contractor is the data the profile needs, independent of its origin.
type Contractor struct {
	ID               string
	Name             string
	Profession       string
	BusinessName     string
	BusinessActivity string
	RFC              string
	TaxRegime        *string
	CLABE            string
	FirstCustomer    FirstCustomer
}

// FirstCustomer is the customer captured during onboarding (`client`): the
// person who receives the business's first charge.
type FirstCustomer struct {
	Name  string
	Phone string
	Email string
}

// ProfileSource provides contractor data. Implementations return
// ErrContractorNotFound when the ID does not exist.
type ProfileSource interface {
	FindContractor(ctx context.Context, id string) (Contractor, error)
}

// Profile is the dashboard header: who the contractor is and what is missing
// from their onboarding.
type Profile struct {
	ContractorID  string
	Name          string
	Profession    string
	Business      Business
	FirstCustomer FirstCustomer
	Onboarding    OnboardingSummary
}

type Business struct {
	Name         string
	Activity     string
	RFC          string
	TaxRegime    *string
	TaxpayerType TaxpayerType
	// ClabeLast4 masks the deposit account; nil when no CLABE is on file.
	ClabeLast4 *string
}

type OnboardingSummary struct {
	Status        OnboardingStatus
	PendingFields []Field
}

// NewProfile derives the profile view from a contractor.
func NewProfile(contractor Contractor) Profile {
	pending := pendingFields(contractor)
	status := OnboardingStatusComplete
	if len(pending) > 0 {
		status = OnboardingStatusIncomplete
	}
	return Profile{
		ContractorID: contractor.ID,
		Name:         contractor.Name,
		Profession:   contractor.Profession,
		Business: Business{
			Name:         contractor.BusinessName,
			Activity:     contractor.BusinessActivity,
			RFC:          contractor.RFC,
			TaxRegime:    copyString(contractor.TaxRegime),
			TaxpayerType: taxpayerTypeFromRFC(contractor.RFC),
			ClabeLast4:   clabeLast4(contractor.CLABE),
		},
		FirstCustomer: contractor.FirstCustomer,
		Onboarding:    OnboardingSummary{Status: status, PendingFields: pending},
	}
}

func pendingFields(contractor Contractor) []Field {
	pending := []Field{}
	if contractor.BusinessName == "" {
		pending = append(pending, FieldBusinessName)
	}
	if contractor.BusinessActivity == "" {
		pending = append(pending, FieldBusinessActivity)
	}
	if contractor.RFC == "" {
		pending = append(pending, FieldRFC)
	}
	if contractor.TaxRegime == nil {
		pending = append(pending, FieldTaxRegime)
	}
	if contractor.CLABE == "" {
		pending = append(pending, FieldClabe)
	}
	return pending
}

func clabeLast4(clabe string) *string {
	if len(clabe) < ClabeVisibleDigits {
		return nil
	}
	last := clabe[len(clabe)-ClabeVisibleDigits:]
	return &last
}

func taxpayerTypeFromRFC(rfc string) TaxpayerType {
	switch utf8.RuneCountInString(rfc) {
	case rfcLengthIndividual:
		return TaxpayerTypeIndividual
	case rfcLengthCompany:
		return TaxpayerTypeCompany
	default:
		return TaxpayerTypeUnknown
	}
}

func copyString(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
