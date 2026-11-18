package domain

import (
	"encoding/json"
	"testing"
)

func TestEncodeRegistrationChoosesEntityFromRFC(t *testing.T) {
	business := registrationUser(t, "AAA010101AAA")
	body, ok := EncodeRegistration(business)
	if !ok {
		t.Fatal("expected business registration")
	}
	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["entity"] != "BUSINESS" || payload["name"] != business.BusinessName || payload["alias"] != business.BusinessName || payload["clabe"] != business.CLABE || payload["taxId"] != business.RFC {
		t.Fatalf("business body: %s", body)
	}
	person := registrationUser(t, "AAAA010101AAA")
	body, ok = EncodeRegistration(person)
	if !ok {
		t.Fatal("expected person registration")
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["entity"] != "PERSON" || payload["name"] != person.Name || payload["alias"] != person.BusinessName || payload["taxId"] != person.RFC {
		t.Fatalf("person body: %s", body)
	}
	noClabe := person
	noClabe.CLABE = ""
	if _, ok := EncodeRegistration(noClabe); ok {
		t.Fatal("expected empty CLABE to skip pey registration")
	}
	noBusiness := person
	noBusiness.RFC = ""
	noBusiness.BusinessName = ""
	body, ok = EncodeRegistration(noBusiness)
	if !ok {
		t.Fatal("expected registration without RFC/business name to register as PERSON with CLABE")
	}
	payload = map[string]string{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["entity"] != "PERSON" || payload["name"] != noBusiness.Name || payload["alias"] != noBusiness.Name || payload["clabe"] != noBusiness.CLABE {
		t.Fatalf("no-business body: %s", body)
	}
	if _, hasTax := payload["taxId"]; hasTax {
		t.Fatalf("taxId must be omitted without RFC: %s", body)
	}
}

func TestDecodeBankAccountAcceptsPeyStatus(t *testing.T) {
	account, ok := DecodeBankAccount([]byte(`{"id":"01BANK","codiStatus":"PENDING"}`))
	if !ok || account.ID != "01BANK" || account.Status != "PENDING" {
		t.Fatalf("got %+v ok=%v", account, ok)
	}
	user := registrationUser(t, "AAA010101AAA")
	updated, ok := AttachBankAccount(user, account)
	if !ok || updated.BankAccountID != "01BANK" || updated.CodiStatus != "PENDING" || updated.CLABE != user.CLABE {
		t.Fatalf("attach: %+v", updated)
	}
}

func TestDecodeBankAccountRejectsUnknownStatus(t *testing.T) {
	if _, ok := DecodeBankAccount([]byte(`{"id":"01BANK","codiStatus":"MAYBE"}`)); ok {
		t.Fatal("accepted unknown status")
	}
	user := registrationUser(t, "AAA010101AAA")
	updated, ok := AttachBankAccount(user, BankAccount{ID: "01BANK", Status: "MAYBE"})
	if ok || updated.BankAccountID != "" || updated.CodiStatus != NotRegisteredStatus() {
		t.Fatalf("changed user: %+v ok=%v", updated, ok)
	}
}

func registrationUser(t *testing.T, rfc string) User {
	t.Helper()
	user, fieldErrors := Build(Input{
		Name: "Ana", Profession: "Electrician",
		Client:           Client{Name: "Luis", Phone: "+52 555 123 4567", Email: "luis@example.com"},
		BusinessName:     "Electrical Services",
		BusinessActivity: "Electrical installations",
		RFC:              rfc,
		CLABE:            "002010077777777771",
	})
	if len(fieldErrors) > 0 {
		t.Fatalf("build: %v", fieldErrors)
	}
	user.ID = "user-1"
	return user
}
