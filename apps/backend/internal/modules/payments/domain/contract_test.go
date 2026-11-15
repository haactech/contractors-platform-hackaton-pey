package domain

import (
	"contractors-platform/backend/internal/modules/payments/definitions"
	"encoding/json"
	"testing"
)

func readyPayment(method string) Payment {
	payment, errors := Build(Request{
		VendorID:    "vendor-1",
		ClientName:  "Luis",
		ClientPhone: "5551234567",
		Concept:     "Mensualidad",
		AmountCents: 15000,
		DueDate:     "2026-12-01",
		Method:      method,
	})
	if len(errors) > 0 {
		panic(errors)
	}
	payment.ID = "pay-1"
	payment.BankAccountID = "bank-vendor-1"
	return payment
}

func TestEncodeQRCarriesThatVendorsBankID(t *testing.T) {
	path, payload, ok := Encode(readyPayment(definitions.MethodCodiQR), Links{})
	if !ok || path != definitions.PeyPathCodiQR {
		t.Fatalf("encode: ok=%v path=%s", ok, path)
	}
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatal(err)
	}
	if body[definitions.PeyBankAccountID] != "bank-vendor-1" {
		t.Fatalf("bank id: %v", body[definitions.PeyBankAccountID])
	}
	if body[definitions.PeyAmount] != float64(150) || body[definitions.PeyConcept] != "Mensualidad" || body[definitions.PeyExpiration] != float64(definitions.QRExpirationSeconds) {
		t.Fatalf("body: %s", payload)
	}
	metadata, _ := body[definitions.PeyMetadata].(map[string]any)
	if metadata[definitions.PeyMetadataPaymentID] != "pay-1" {
		t.Fatalf("metadata: %v", metadata)
	}
}

func TestEncodeLinkOmitsBankID(t *testing.T) {
	links := Links{Success: "https://example.com/ok", Back: "https://example.com/back"}
	path, payload, ok := Encode(readyPayment(definitions.MethodPaymentLink), links)
	if !ok || path != definitions.PeyPathPaymentLink {
		t.Fatalf("encode: ok=%v path=%s", ok, path)
	}
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatal(err)
	}
	if _, exists := body[definitions.PeyBankAccountID]; exists {
		t.Fatalf("link named a bank id: %s", payload)
	}
	if body[definitions.PeyClientReference] != "pay-1" || body[definitions.PeySuccessURL] != links.Success || body[definitions.PeyBackURL] != links.Back {
		t.Fatalf("body: %s", payload)
	}
}

func TestEncodeLinkRequiresHTTPS(t *testing.T) {
	if _, _, ok := Encode(readyPayment(definitions.MethodPaymentLink), Links{Success: "http://example.com/ok", Back: "https://example.com/back"}); ok {
		t.Fatal("accepted a non-https success url")
	}
	if _, payload, ok := Encode(readyPayment(definitions.MethodCodiQR), Links{}); !ok || len(payload) == 0 {
		t.Fatal("qr does not need the link urls")
	}
}

func TestAccountMustBeActiveForEitherMethod(t *testing.T) {
	if !AccountReady(Account{BankAccountID: "bank-vendor-1", CodiStatus: definitions.CodiActive}) {
		t.Fatal("active account was rejected")
	}
	for _, account := range []Account{
		{},
		{CodiStatus: definitions.CodiActive},
		{BankAccountID: "bank-vendor-1", CodiStatus: "PENDING"},
		{BankAccountID: "bank-vendor-1", CodiStatus: "not_registered"},
	} {
		if AccountReady(account) {
			t.Fatalf("accepted %+v", account)
		}
	}
	payment := readyPayment(definitions.MethodCodiQR)
	payment.BankAccountID = ""
	if _, _, ok := Encode(payment, Links{}); ok {
		t.Fatal("qr encoded without a bank id")
	}
}

func TestDecodeRejectsUnusablePayloads(t *testing.T) {
	link, ok := Decode(definitions.MethodPaymentLink, []byte(`{"id":"pey-1","url":"https://pay.example/abc"}`))
	if !ok || link.LinkURL == "" || link.PeyID != "pey-1" {
		t.Fatalf("link: %+v %v", link, ok)
	}
	qr, ok := Decode(definitions.MethodCodiQR, []byte(`{"id":"pey-qr","qrString":"QRDATA","qrImage":"img"}`))
	if !ok || qr.QR != "QRDATA" || qr.Image != "img" {
		t.Fatalf("qr: %+v %v", qr, ok)
	}
	for _, payload := range []string{`{}`, `{"id":"pey-1"}`, `{"id":"pey-1","url":""}`, `{"url":"https://pay.example/abc"}`, `not-json`, `{"id":"pey-qr"}`} {
		if _, ok := Decode(definitions.MethodPaymentLink, []byte(payload)); ok && payload != `{"id":"pey-qr"}` {
			t.Fatalf("accepted link %s", payload)
		}
	}
	if _, ok := Decode(definitions.MethodCodiQR, []byte(`{"id":"pey-qr"}`)); ok {
		t.Fatal("accepted a qr without qrString")
	}
}

func TestMatchClientAcceptsMexicanPhoneSuffix(t *testing.T) {
	clients := []Client{{Name: " Luis ", Phone: "+52 555 123 4567", Email: "luis@example.com"}, {Name: "Ana", Phone: "5511111111", Email: "ana@example.com"}}
	matched, ok := MatchClient(clients, "luis", "5551234567")
	if !ok || matched.Email != "luis@example.com" {
		t.Fatalf("match: %+v %v", matched, ok)
	}
	if _, ok := MatchClient(clients, "Nadie", ""); ok {
		t.Fatal("matched an unknown client")
	}
	if _, ok := MatchClient([]Client{{Name: "Luis", Phone: "1"}, {Name: "Luis", Phone: "2"}}, "Luis", ""); ok {
		t.Fatal("matched two clients with the same name")
	}
}

func TestPresentHidesBankID(t *testing.T) {
	payment := Attach(readyPayment(definitions.MethodCodiQR), Result{PeyID: "pey-qr", QR: "QRDATA", Image: "img"})
	view := Present(payment)
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if view.CodiQRID != "pey-qr" || view.CodiQRString != "QRDATA" {
		t.Fatalf("view: %+v", view)
	}
	var body map[string]any
	_ = json.Unmarshal(encoded, &body)
	if _, exists := body["bankAccountId"]; exists || body["onboardingId"] != "vendor-1" {
		t.Fatalf("json: %s", encoded)
	}
}

func TestEncodePushUsesTheStoredPhoneAndBankID(t *testing.T) {
	payment := readyPayment(definitions.MethodCodiPush)
	payment.ClientPhone = "+52 555 123 4567"
	path, payload, ok := Encode(payment, Links{})
	if !ok || path != definitions.PeyPathCodiPush {
		t.Fatalf("encode: ok=%v path=%s", ok, path)
	}
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatal(err)
	}
	if body[definitions.PeyPhoneNumber] != "5551234567" || body[definitions.PeyBankAccountID] != "bank-vendor-1" || body[definitions.PeyExpiration] != float64(definitions.PushExpirationSeconds) || body[definitions.PeyAmount] != float64(150) {
		t.Fatalf("body: %s", payload)
	}
	if _, exists := body["numRef"]; exists || body["id"] != nil {
		t.Fatalf("unexpected keys: %s", payload)
	}
	payment.ClientPhone = "12345"
	if _, _, ok := Encode(payment, Links{}); ok {
		t.Fatal("encoded a push without 10 digits")
	}
	over, errors := Build(Request{VendorID: "vendor-1", ClientName: "Luis", Concept: "Mensualidad", AmountCents: definitions.PushMaxCents + 1, Method: definitions.MethodCodiPush})
	if over.ID != "" || errors[definitions.FieldAmount] != definitions.MessagePushAmount {
		t.Fatalf("amount: %+v %v", over, errors)
	}
}

func TestDecodePushUsesTransactionID(t *testing.T) {
	result, ok := Decode(definitions.MethodCodiPush, []byte(`{"transactionId":"txn-1","codiId":"folio"}`))
	if !ok || result.PeyID != "txn-1" || result.LinkURL != "" || result.QR != "" {
		t.Fatalf("push: %+v %v", result, ok)
	}
	if _, ok := Decode(definitions.MethodCodiPush, []byte(`{"id":"pey-qr"}`)); ok {
		t.Fatal("accepted a push response without transactionId")
	}
	payment := Attach(readyPayment(definitions.MethodCodiPush), result)
	view := Present(payment)
	if view.CodiQRID != "" || view.PaymentLinkURL != "" || view.CodiQRString != "" {
		t.Fatalf("present leaked a link or qr: %+v", view)
	}
}

func TestDirectPhoneReadyRejectsAShortStoredPhone(t *testing.T) {
	payment := readyPayment(definitions.MethodCodiPush)
	payment.ClientPhone = "+52 555 123 4567"
	if errors := DirectPhoneReady(payment); len(errors) != 0 {
		t.Fatalf("ready phone rejected: %v", errors)
	}
	payment.ClientPhone = "555"
	if errors := DirectPhoneReady(payment); errors[definitions.FieldClientPhone] != definitions.MessagePushPhone {
		t.Fatalf("short phone: %v", errors)
	}
}
