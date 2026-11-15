package domain

import "testing"

func TestCompanyWithoutLegalNameIsRejected(t *testing.T) {
	_, fieldErrors := BuildDashboard("vendor-1", Input{
		EntityType: "company", Name: "Luis", Phone: "5551234567", Email: "luis@example.com",
	})
	if fieldErrors[PhoneField()] != "" || fieldErrors["legalName"] == "" {
		t.Fatalf("errors: %v", fieldErrors)
	}
}

func TestRFCShapeFailsClosed(t *testing.T) {
	_, fieldErrors := BuildDashboard("vendor-1", Input{
		EntityType: "individual", Name: "Luis", Phone: "5551234567", Email: "luis@example.com", RFC: "NO",
	})
	if fieldErrors["rfc"] == "" {
		t.Fatal("accepted a short rfc")
	}
	client, fieldErrors := BuildDashboard("vendor-1", Input{
		EntityType: "company", Name: "Luis", Phone: "5551234567", Email: "luis@example.com",
		LegalName: "Abarrotes", RFC: "aaa010101aaa",
	})
	if len(fieldErrors) != 0 || client.RFC != "AAA010101AAA" {
		t.Fatalf("rfc: %+v %v", client, fieldErrors)
	}
}

func TestTenDigitPhoneMatchesStoredCountryCode(t *testing.T) {
	if !SamePhone("+52 555 123 4567", "5551234567") {
		t.Fatal("10 digits did not match the onboarding phone")
	}
	if PhoneKey("+52 555 123 4567") != "5551234567" {
		t.Fatalf("key: %s", PhoneKey("+52 555 123 4567"))
	}
	client, fieldErrors := BuildFirst("vendor-1", "Luis", "+52 555 123 4567", "luis@example.com")
	if len(fieldErrors) != 0 || client.EntityType != "individual" || client.Phone != "+52 555 123 4567" || client.LegalName != "" || client.PhoneKey != "5551234567" {
		t.Fatalf("first: %+v %v", client, fieldErrors)
	}
}

func TestSecondDashboardPhoneCollidesOnTheSameKey(t *testing.T) {
	first, _ := BuildFirst("vendor-1", "Luis", "+52 555 123 4567", "luis@example.com")
	second, fieldErrors := BuildDashboard("vendor-1", Input{EntityType: "individual", Name: "Ana", Phone: "55 5123 4567", Email: "ana@example.com"})
	if len(fieldErrors) != 0 || first.PhoneKey == "" || first.PhoneKey != second.PhoneKey {
		t.Fatalf("keys: %s %s %v", first.PhoneKey, second.PhoneKey, fieldErrors)
	}
}
