package app

import (
	"bytes"
	"context"
	"contractors-platform/backend/internal/modules/accounts"
	"contractors-platform/backend/internal/modules/accounts/definitions"
	accdomain "contractors-platform/backend/internal/modules/accounts/domain"
	accinfra "contractors-platform/backend/internal/modules/accounts/infrastructure"
	clientsinfra "contractors-platform/backend/internal/modules/clients/infrastructure"
	dashboarddomain "contractors-platform/backend/internal/modules/dashboard/domain"
	dashboardpresentation "contractors-platform/backend/internal/modules/dashboard/presentation"
	onbdefs "contractors-platform/backend/internal/modules/onboarding/definitions"
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"contractors-platform/backend/internal/modules/onboarding/infrastructure"
	paymentsinfra "contractors-platform/backend/internal/modules/payments/infrastructure"
	"contractors-platform/backend/internal/platform/database"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type sentMail struct {
	to, subject, body string
}

type spyMailer struct {
	mu   sync.Mutex
	sent []sentMail
}

func (m *spyMailer) Send(_ context.Context, to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMail{to: to, subject: subject, body: body})
	return nil
}

func (m *spyMailer) lastCode() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		return ""
	}
	re := regexp.MustCompile(`\b(\d{6})\b`)
	match := re.FindStringSubmatch(m.sent[len(m.sent)-1].body)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock() *testClock {
	return &testClock{now: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type testHarness struct {
	handler http.Handler
	onbRepo *infrastructure.MemoryRepository
	accRepo *accinfra.MemoryRepository
	mailer  *spyMailer
	clock   *testClock
}

func newHarness(policy accdomain.Policy, banks domain.BankRegistrar) *testHarness {
	if banks == nil {
		banks = infrastructure.NewUnavailablePey()
	}
	onbRepo := infrastructure.NewMemoryRepository()
	accRepo := accinfra.NewMemoryRepository()
	mailer := &spyMailer{}
	clock := newTestClock()
	handler, err := NewWith(Dependencies{
		Onboardings: onbRepo,
		Accounts:    accRepo,
		Banks:       banks,
		Hasher:      accounts.NewPBKDF2Hasher(1_000),
		Mailer:      mailer,
		Policy:      policy,
		Clock:       clock.Now,
	})
	if err != nil {
		panic(err)
	}
	return &testHarness{
		handler: handler,
		onbRepo: onbRepo,
		accRepo: accRepo,
		mailer:  mailer,
		clock:   clock,
	}
}

func validInput() map[string]any {
	return map[string]any{
		"name":             " Ana ",
		"profession":       "Electrician",
		"client":           map[string]any{"name": "Luis", "phone": "+52 555 123 4567", "email": "luis@example.com"},
		"businessName":     "Electrical Services",
		"businessActivity": "Electrical installations",
		"rfc":              " aaa010101aaa ",
		"clabe":            "002 010 077777777771",
	}
}

func doJSON(handler http.Handler, method, path string, payload any, cookie *http.Cookie) *httptest.ResponseRecorder {
	var bodyReader *bytes.Reader
	if payload != nil {
		body, _ := json.Marshal(payload)
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	request := httptest.NewRequest(method, path, bodyReader)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func sessionCookieFrom(response *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range response.Result().Cookies() {
		if c.Name == definitions.SessionCookieName {
			return c
		}
	}
	return nil
}

func signUpAndVerify(t *testing.T, h *testHarness, email, phone, pin string) *http.Cookie {
	t.Helper()
	res := doJSON(h.handler, http.MethodPost, "/api/auth/signup", map[string]string{
		"email": email,
		"phone": phone,
		"pin":   pin,
	}, nil)
	if res.Code != http.StatusCreated {
		t.Fatalf("signup %s: %d %s", email, res.Code, res.Body.String())
	}
	cookie := sessionCookieFrom(res)
	if cookie == nil {
		t.Fatalf("missing session cookie on signup")
	}
	code := h.mailer.lastCode()
	if code == "" {
		t.Fatalf("expected verification email code in strict mode")
	}
	confirm := doJSON(h.handler, http.MethodPost, "/api/auth/email-verification/confirm", map[string]string{
		"code": code,
	}, cookie)
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm email %s: %d %s", email, confirm.Code, confirm.Body.String())
	}
	return cookie
}

func submit(handler http.Handler, payload any, cookie *http.Cookie) *httptest.ResponseRecorder {
	return doJSON(handler, http.MethodPost, "/api/onboarding", payload, cookie)
}

func TestCreateAndRetrieve(t *testing.T) {
	for _, withTax := range []bool{false, true} {
		t.Run(map[bool]string{false: "without tax regime", true: "with tax regime"}[withTax], func(t *testing.T) {
			h := newHarness(accdomain.StrictPolicy(), nil)
			cookie := signUpAndVerify(t, h, "ana@praxis.mx", "5512345678", "123456")

			input := validInput()
			if withTax {
				input["taxRegime"] = "General"
			}
			response := submit(h.handler, input, cookie)
			if response.Code != http.StatusCreated {
				t.Fatalf("create: %d %s", response.Code, response.Body)
			}
			var result map[string]any
			_ = json.Unmarshal(response.Body.Bytes(), &result)
			if result["id"] == "" || result["name"] != "Ana" || result["rfc"] != "AAA010101AAA" || result["clabe"] != "002010077777777771" {
				t.Fatalf("unexpected record: %v", result)
			}
			if _, exists := result["taxRegime"]; exists != withTax {
				t.Fatalf("tax regime presence: %v", result)
			}
			location := response.Header().Get("Location")
			if location != "/api/onboarding/"+result["id"].(string) {
				t.Fatalf("invalid Location: %s", location)
			}
			req := httptest.NewRequest(http.MethodGet, location, nil)
			req.AddCookie(cookie)
			fetched := httptest.NewRecorder()
			h.handler.ServeHTTP(fetched, req)
			if fetched.Code != http.StatusOK || fetched.Body.String() != response.Body.String() {
				t.Fatalf("fetch: %d %s", fetched.Code, fetched.Body)
			}
		})
	}
}

func TestRequiredFields(t *testing.T) {
	for _, field := range []string{"name", "profession", "clabe", "client.name", "client.phone", "client.email"} {
		t.Run(field, func(t *testing.T) {
			h := newHarness(accdomain.StrictPolicy(), nil)
			cookie := signUpAndVerify(t, h, "req@praxis.mx", "5512345678", "123456")
			input := validInput()
			if strings.HasPrefix(field, "client.") {
				input["client"].(map[string]any)[strings.TrimPrefix(field, "client.")] = "  "
			} else {
				input[field] = "  "
			}
			response := submit(h.handler, input, cookie)
			if response.Code != 422 {
				t.Fatalf("expected 422: %d %s", response.Code, response.Body)
			}
			var result struct {
				Errors map[string]string `json:"errors"`
			}
			_ = json.Unmarshal(response.Body.Bytes(), &result)
			if result.Errors[field] == "" {
				t.Fatalf("missing field error: %s", response.Body)
			}
		})
	}
}

func TestInvalidFormats(t *testing.T) {
	for _, field := range []string{"email", "phone", "rfc", "clabe"} {
		t.Run(field, func(t *testing.T) {
			h := newHarness(accdomain.StrictPolicy(), nil)
			cookie := signUpAndVerify(t, h, "fmt@praxis.mx", "5512345678", "123456")
			input := validInput()
			if field == "rfc" || field == "clabe" {
				input[field] = "invalid"
			} else {
				input["client"].(map[string]any)[field] = "invalid"
			}
			if response := submit(h.handler, input, cookie); response.Code != 422 {
				t.Fatalf("expected 422: %d", response.Code)
			}
		})
	}
}

type scriptedPey struct {
	response []byte
	err      error
	calls    int
}

func (pey *scriptedPey) Register(context.Context, []byte) ([]byte, error) {
	pey.calls++
	return pey.response, pey.err
}

func TestPeyAccountIsStoredAndHidden(t *testing.T) {
	pey := &scriptedPey{response: []byte(`{"id":"01BANK","codiStatus":"PENDING"}`)}
	h := newHarness(accdomain.StrictPolicy(), pey)
	cookie := signUpAndVerify(t, h, "pey@praxis.mx", "5512345678", "123456")

	response := submit(h.handler, validInput(), cookie)
	if response.Code != http.StatusCreated || pey.calls != 1 {
		t.Fatalf("create: %d calls=%d %s", response.Code, pey.calls, response.Body)
	}
	if strings.Contains(response.Body.String(), "01BANK") || strings.Contains(response.Body.String(), "codiStatus") || strings.Contains(response.Body.String(), "PENDING") {
		t.Fatalf("status leaked: %s", response.Body)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &created)
	stored, found := h.onbRepo.Find(created.ID)
	if !found || stored.BankAccountID != "01BANK" || stored.CodiStatus != onbdefs.CodiPending {
		t.Fatalf("stored: found=%v %+v", found, stored)
	}
}

func TestPeyFailureKeepsSavedUser(t *testing.T) {
	pey := &scriptedPey{err: errors.New("pey down")}
	h := newHarness(accdomain.StrictPolicy(), pey)
	cookie := signUpAndVerify(t, h, "peydown@praxis.mx", "5512345678", "123456")

	response := submit(h.handler, validInput(), cookie)
	if response.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", response.Code, response.Body)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &created)
	stored, found := h.onbRepo.Find(created.ID)
	if !found || stored.BankAccountID != "" || stored.CodiStatus != domain.NotRegisteredStatus() {
		t.Fatalf("stored: found=%v %+v", found, stored)
	}
}

func TestUnknownPeyStatusDoesNotOverwriteUser(t *testing.T) {
	pey := &scriptedPey{response: []byte(`{"id":"01BANK","codiStatus":"MAYBE"}`)}
	h := newHarness(accdomain.StrictPolicy(), pey)
	cookie := signUpAndVerify(t, h, "peymaybe@praxis.mx", "5512345678", "123456")

	response := submit(h.handler, validInput(), cookie)
	if response.Code != http.StatusCreated || pey.calls != 1 {
		t.Fatalf("create: %d calls=%d %s", response.Code, pey.calls, response.Body)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &created)
	stored, found := h.onbRepo.Find(created.ID)
	if !found || stored.BankAccountID != "" || stored.CodiStatus != domain.NotRegisteredStatus() {
		t.Fatalf("stored: found=%v %+v", found, stored)
	}
}

func TestDuplicateCLABEDoesNotCallPey(t *testing.T) {
	pey := &scriptedPey{response: []byte(`{"id":"01BANK","codiStatus":"PENDING"}`)}
	h := newHarness(accdomain.StrictPolicy(), pey)
	cookie1 := signUpAndVerify(t, h, "c1@praxis.mx", "5512345671", "123456")
	cookie2 := signUpAndVerify(t, h, "c2@praxis.mx", "5512345672", "123456")

	if response := submit(h.handler, validInput(), cookie1); response.Code != http.StatusCreated || pey.calls != 1 {
		t.Fatalf("first: %d calls=%d %s", response.Code, pey.calls, response.Body)
	}
	again := validInput()
	again["name"] = "Otro"
	response := submit(h.handler, again, cookie2)
	if response.Code != http.StatusUnprocessableEntity || pey.calls != 1 {
		t.Fatalf("duplicate: %d calls=%d %s", response.Code, pey.calls, response.Body)
	}
}

func TestDuplicateCLABE(t *testing.T) {
	h := newHarness(accdomain.StrictPolicy(), nil)
	cookie1 := signUpAndVerify(t, h, "d1@praxis.mx", "5512345671", "123456")
	cookie2 := signUpAndVerify(t, h, "d2@praxis.mx", "5512345672", "123456")

	if response := submit(h.handler, validInput(), cookie1); response.Code != http.StatusCreated {
		t.Fatalf("first: %d %s", response.Code, response.Body)
	}
	again := validInput()
	again["name"] = "Otro"
	response := submit(h.handler, again, cookie2)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate: %d %s", response.Code, response.Body)
	}
	var result struct {
		Errors map[string]string `json:"errors"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &result)
	if result.Errors[onbdefs.FieldCLABE] != onbdefs.MessageCLABETaken {
		t.Fatalf("field error: %s", response.Body)
	}
	other := validInput()
	other["clabe"] = "012180001183597198"
	if response := submit(h.handler, other, cookie2); response.Code != http.StatusCreated {
		t.Fatalf("other clabe: %d %s", response.Code, response.Body)
	}
}

func TestOnboardingStep2OptionalAndProfileUpdate(t *testing.T) {
	pey := &scriptedPey{response: []byte(`{"id":"01BANK","codiStatus":"PENDING"}`)}
	h := newHarness(accdomain.StrictPolicy(), pey)
	cookieA := signUpAndVerify(t, h, "opt-a@praxis.mx", "5512345601", "123456")
	cookieB := signUpAndVerify(t, h, "opt-b@praxis.mx", "5512345602", "123456")

	minimalInput := func(email, clabe string) map[string]any {
		return map[string]any{
			"name":       "Luis Freelance",
			"profession": "Plomero",
			"clabe":      clabe,
			"client":     map[string]any{"name": "María", "phone": "5511223344", "email": email},
		}
	}

	// Required CLABE with omitted Step 2 business fields succeeds and registers CLABE in Pey.
	resA := submit(h.handler, minimalInput("maria@example.com", "002010077777777771"), cookieA)
	if resA.Code != http.StatusCreated {
		t.Fatalf("expected 201 omitting step 2: %d %s", resA.Code, resA.Body)
	}
	resB := submit(h.handler, minimalInput("marcos@example.com", "012180001183597198"), cookieB)
	if resB.Code != http.StatusCreated {
		t.Fatalf("second onboarding omitting step 2 failed: %d %s", resB.Code, resB.Body)
	}
	if pey.calls != 2 {
		t.Fatalf("expected pey called immediately for both onboardings with CLABE, got %d", pey.calls)
	}

	var createdA struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(resA.Body.Bytes(), &createdA)

	// Dashboard profile reflects incomplete step-2 fields while already masking saved CLABE.
	profPath := strings.Replace(dashboardpresentation.ProfilePath, "{"+dashboardpresentation.PathParamContractorID+"}", createdA.ID, 1)
	profReq := httptest.NewRequest(http.MethodGet, profPath, nil)
	profReq.AddCookie(cookieA)
	profRes := httptest.NewRecorder()
	h.handler.ServeHTTP(profRes, profReq)
	if profRes.Code != http.StatusOK {
		t.Fatalf("profile: %d %s", profRes.Code, profRes.Body)
	}
	var prof dashboardpresentation.ProfileResponse
	_ = json.Unmarshal(profRes.Body.Bytes(), &prof)
	if prof.Onboarding.Status != dashboarddomain.OnboardingStatusIncomplete || len(prof.Onboarding.PendingFields) != 4 || prof.Business.ClabeLast4 == nil || *prof.Business.ClabeLast4 != "7771" {
		t.Fatalf("unexpected minimal profile: %s", profRes.Body)
	}

	// Cross-account PATCH is rejected with 404.
	patchBody, _ := json.Marshal(map[string]any{"businessName": "Plomería Luis"})
	crossReq := httptest.NewRequest(http.MethodPatch, "/api/onboarding/"+createdA.ID, bytes.NewReader(patchBody))
	crossReq.Header.Set("Content-Type", "application/json")
	crossReq.AddCookie(cookieB)
	crossRes := httptest.NewRecorder()
	h.handler.ServeHTTP(crossRes, crossReq)
	if crossRes.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-account PATCH, got %d", crossRes.Code)
	}

	// Clearing required CLABE on PATCH is rejected with 422.
	clearClabe, _ := json.Marshal(map[string]any{"clabe": ""})
	clearReq := httptest.NewRequest(http.MethodPatch, "/api/onboarding/"+createdA.ID, bytes.NewReader(clearClabe))
	clearReq.Header.Set("Content-Type", "application/json")
	clearReq.AddCookie(cookieA)
	clearRes := httptest.NewRecorder()
	h.handler.ServeHTTP(clearRes, clearReq)
	if clearRes.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 clearing CLABE on PATCH, got %d %s", clearRes.Code, clearRes.Body)
	}

	// Account A completes optional business info later in profile -> becomes complete without extra Pey call when CLABE unchanged.
	fullPatch, _ := json.Marshal(map[string]any{
		"businessName":     "Plomería Luis",
		"businessActivity": "Electricidad y plomería",
		"rfc":              "lufp900101abc",
		"taxRegime":        "RESICO",
	})
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/onboarding/"+createdA.ID, bytes.NewReader(fullPatch))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.AddCookie(cookieA)
	patchRes := httptest.NewRecorder()
	h.handler.ServeHTTP(patchRes, patchReq)
	if patchRes.Code != http.StatusOK {
		t.Fatalf("expected 200 on profile patch: %d %s", patchRes.Code, patchRes.Body)
	}
	if pey.calls != 2 {
		t.Fatalf("expected no duplicate pey call when CLABE already registered, got %d", pey.calls)
	}

	// Account B trying to claim Account A's CLABE gets 422.
	var createdB struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(resB.Body.Bytes(), &createdB)
	dupPatch, _ := json.Marshal(map[string]any{"clabe": "002010077777777771"})
	dupReq := httptest.NewRequest(http.MethodPatch, "/api/onboarding/"+createdB.ID, bytes.NewReader(dupPatch))
	dupReq.Header.Set("Content-Type", "application/json")
	dupReq.AddCookie(cookieB)
	dupRes := httptest.NewRecorder()
	h.handler.ServeHTTP(dupRes, dupReq)
	if dupRes.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on duplicate CLABE patch, got %d %s", dupRes.Code, dupRes.Body)
	}

	// Dashboard profile for A now reports complete & masked CLABE.
	profReq2 := httptest.NewRequest(http.MethodGet, profPath, nil)
	profReq2.AddCookie(cookieA)
	profRes2 := httptest.NewRecorder()
	h.handler.ServeHTTP(profRes2, profReq2)
	var prof2 dashboardpresentation.ProfileResponse
	_ = json.Unmarshal(profRes2.Body.Bytes(), &prof2)
	if prof2.Onboarding.Status != dashboarddomain.OnboardingStatusComplete || prof2.Business.ClabeLast4 == nil || *prof2.Business.ClabeLast4 != "7771" {
		t.Fatalf("expected complete profile with last4=7771, got %s", profRes2.Body)
	}
}

func TestMalformedRequests(t *testing.T) {
	h := newHarness(accdomain.StrictPolicy(), nil)
	cookie := signUpAndVerify(t, h, "mal@praxis.mx", "5512345678", "123456")
	for _, body := range []string{`{`, `{"unknown":true}`, `{} {}`, `[]`, `{"client":{"unknown":true}}`, `{"name":42}`} {
		request := httptest.NewRequest(http.MethodPost, "/api/onboarding", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		h.handler.ServeHTTP(response, request)
		if response.Code != 400 {
			t.Errorf("body %s: expected 400, got %d", body, response.Code)
		}
	}
}

func TestHTTPStatusCodes(t *testing.T) {
	h := newHarness(accdomain.StrictPolicy(), nil)
	cookie := signUpAndVerify(t, h, "http@praxis.mx", "5512345678", "123456")

	for _, test := range []struct {
		method, path, contentType, body string
		withCookie                      bool
		status                          int
	}{
		{"GET", "/api/health", "", "", false, 200},
		{"GET", "/api/contractors", "", "", false, 200},
		{"GET", "/api/onboarding/missing", "", "", false, 401},
		{"GET", "/api/onboarding/missing", "", "", true, 404},
		{"PUT", "/api/onboarding", "", "", false, 405},
		{"POST", "/api/onboarding", "text/plain", "{}", true, 415},
		{"POST", "/api/onboarding", "application/json", strings.Repeat(" ", 1<<20) + "{}", true, 413},
	} {
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		request.Header.Set("Content-Type", test.contentType)
		if test.withCookie {
			request.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		h.handler.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Errorf("%s %s (cookie=%v): expected %d, got %d", test.method, test.path, test.withCookie, test.status, response.Code)
		}
	}
}

func TestCreateAndRetrieveWithSQLite(t *testing.T) {
	dir := t.TempDir()
	vendors, err := database.OpenSQLite(filepath.Join(dir, "vendors.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer vendors.Close()
	clientsDB, err := database.OpenSQLite(filepath.Join(dir, "clients.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer clientsDB.Close()
	paymentsDB, err := database.OpenSQLite(filepath.Join(dir, "payments.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer paymentsDB.Close()
	onbRepo, err := infrastructure.NewSQLiteRepository(vendors)
	if err != nil {
		t.Fatal(err)
	}
	accRepo, err := accinfra.NewSQLiteRepository(vendors)
	if err != nil {
		t.Fatal(err)
	}
	clientsRepo, err := clientsinfra.NewSQLiteRepository(clientsDB)
	if err != nil {
		t.Fatal(err)
	}
	paymentsRepo, err := paymentsinfra.NewSQLiteRepository(paymentsDB)
	if err != nil {
		t.Fatal(err)
	}
	mailer := &spyMailer{}
	handler, err := NewWith(Dependencies{
		Onboardings:    onbRepo,
		Accounts:       accRepo,
		Clients:        clientsRepo,
		ClientVendors:  clientsinfra.NewSQLiteVendors(vendors),
		Payments:       paymentsRepo,
		PaymentVendors: paymentsinfra.NewSQLiteVendors(vendors),
		Collector:      paymentsinfra.UnavailableCollector{},
		Banks:          infrastructure.NewUnavailablePey(),
		Hasher:         accounts.NewPBKDF2Hasher(1_000),
		Mailer:         mailer,
		Policy:         accdomain.StrictPolicy(),
		Clock:          time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}

	signup := doJSON(handler, http.MethodPost, "/api/auth/signup", map[string]string{
		"email": "sqlite@praxis.mx", "phone": "5512345678", "pin": "123456",
	}, nil)
	if signup.Code != http.StatusCreated {
		t.Fatalf("signup: %d %s", signup.Code, signup.Body)
	}
	cookie := sessionCookieFrom(signup)
	confirm := doJSON(handler, http.MethodPost, "/api/auth/email-verification/confirm", map[string]string{
		"code": mailer.lastCode(),
	}, cookie)
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm: %d %s", confirm.Code, confirm.Body)
	}

	response := submit(handler, validInput(), cookie)
	if response.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", response.Code, response.Body)
	}
	location := response.Header().Get("Location")
	req := httptest.NewRequest(http.MethodGet, location, nil)
	req.AddCookie(cookie)
	found := httptest.NewRecorder()
	handler.ServeHTTP(found, req)
	if found.Code != http.StatusOK || !strings.Contains(found.Body.String(), `"rfc":"AAA010101AAA"`) || !strings.Contains(found.Body.String(), `"email":"luis@example.com"`) {
		t.Fatalf("find: %d %s", found.Code, found.Body)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &created)
	listReq := httptest.NewRequest(http.MethodGet, "/api/contractors/"+created.ID+"/clients", nil)
	listReq.AddCookie(cookie)
	listed := httptest.NewRecorder()
	handler.ServeHTTP(listed, listReq)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"entityType":"individual"`) || !strings.Contains(listed.Body.String(), `"+52 555 123 4567"`) {
		t.Fatalf("clients: %d %s", listed.Code, listed.Body)
	}
	anonList := httptest.NewRecorder()
	handler.ServeHTTP(anonList, httptest.NewRequest(http.MethodGet, "/api/contractors/"+created.ID+"/clients", nil))
	if anonList.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on anonymous clients list, got %d", anonList.Code)
	}
	anonPay := httptest.NewRecorder()
	payReq := httptest.NewRequest(http.MethodPost, "/api/payments", strings.NewReader(`{"onboardingId":"`+created.ID+`","clientName":"Luis","concept":"Renta","amountCents":10000,"method":"PAYMENT_LINK"}`))
	payReq.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(anonPay, payReq)
	if anonPay.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on anonymous POST /api/payments, got %d", anonPay.Code)
	}
}

func TestDashboardProfile(t *testing.T) {
	for _, withTax := range []bool{false, true} {
		t.Run(map[bool]string{false: "incomplete onboarding", true: "complete onboarding"}[withTax], func(t *testing.T) {
			h := newHarness(accdomain.StrictPolicy(), nil)
			cookie := signUpAndVerify(t, h, "dash@praxis.mx", "5512345678", "123456")

			input := validInput()
			if withTax {
				input["taxRegime"] = "General"
			}
			var created struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(submit(h.handler, input, cookie).Body.Bytes(), &created)
			path := strings.Replace(dashboardpresentation.ProfilePath, "{"+dashboardpresentation.PathParamContractorID+"}", created.ID, 1)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.AddCookie(cookie)
			response := httptest.NewRecorder()
			h.handler.ServeHTTP(response, req)
			if response.Code != http.StatusOK {
				t.Fatalf("dashboard: %d %s", response.Code, response.Body)
			}
			var dashboard dashboardpresentation.ProfileResponse
			if err := json.Unmarshal(response.Body.Bytes(), &dashboard); err != nil {
				t.Fatal(err)
			}
			wantStatus, wantPending := dashboarddomain.OnboardingStatusIncomplete, 1
			if withTax {
				wantStatus, wantPending = dashboarddomain.OnboardingStatusComplete, 0
			}
			if dashboard.ContractorID != created.ID || dashboard.Name != "Ana" || dashboard.FirstCustomer.Email != "luis@example.com" ||
				dashboard.Business.TaxpayerType != dashboarddomain.TaxpayerTypeCompany ||
				dashboard.Onboarding.Status != wantStatus || len(dashboard.Onboarding.PendingFields) != wantPending ||
				(dashboard.Business.TaxRegime != nil) != withTax {
				t.Fatalf("unexpected dashboard: %s", response.Body)
			}
		})
	}
}

func TestDashboardProfileRequiresAuthAndMatchesOwner(t *testing.T) {
	h := newHarness(accdomain.StrictPolicy(), nil)
	path := strings.Replace(dashboardpresentation.ProfilePath, "{"+dashboardpresentation.PathParamContractorID+"}", "missing", 1)

	unauth := httptest.NewRecorder()
	h.handler.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, path, nil))
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", unauth.Code)
	}

	cookieA := signUpAndVerify(t, h, "a@praxis.mx", "5512345671", "123456")
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(cookieA)
	missing := httptest.NewRecorder()
	h.handler.ServeHTTP(missing, req)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown contractor, got %d", missing.Code)
	}
}

func TestStrictAuthFlowLockoutAndRecovery(t *testing.T) {
	h := newHarness(accdomain.StrictPolicy(), nil)

	// Unverified account is blocked from onboarding (403 email_unverified).
	signup := doJSON(h.handler, http.MethodPost, "/api/auth/signup", map[string]string{
		"email": "strict@praxis.mx", "phone": "55 2222 3333", "pin": "135790",
	}, nil)
	if signup.Code != http.StatusCreated {
		t.Fatalf("signup: %d %s", signup.Code, signup.Body)
	}
	cookie := sessionCookieFrom(signup)
	blockedOnb := submit(h.handler, validInput(), cookie)
	if blockedOnb.Code != http.StatusForbidden || !strings.Contains(blockedOnb.Body.String(), definitions.ErrorCodeEmailUnverified) {
		t.Fatalf("expected 403 email_unverified, got %d %s", blockedOnb.Code, blockedOnb.Body)
	}

	// Resend before 60s returns 429; after 60s succeeds.
	resendEarly := doJSON(h.handler, http.MethodPost, "/api/auth/email-verification", nil, cookie)
	if resendEarly.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 resend cooldown, got %d", resendEarly.Code)
	}
	h.clock.Advance(61 * time.Second)
	resendOk := doJSON(h.handler, http.MethodPost, "/api/auth/email-verification", nil, cookie)
	if resendOk.Code != http.StatusAccepted {
		t.Fatalf("expected 202 resend, got %d %s", resendOk.Code, resendOk.Body)
	}
	if strings.Contains(resendOk.Body.String(), "devCode") {
		t.Fatalf("strict mode must not expose devCode: %s", resendOk.Body)
	}

	// Verify email, create onboarding, verify 409 on second onboarding, and verify session reflects contractorId.
	confirm := doJSON(h.handler, http.MethodPost, "/api/auth/email-verification/confirm", map[string]string{
		"code": h.mailer.lastCode(),
	}, cookie)
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm: %d %s", confirm.Code, confirm.Body)
	}
	onbRes := submit(h.handler, validInput(), cookie)
	if onbRes.Code != http.StatusCreated {
		t.Fatalf("onboarding: %d %s", onbRes.Code, onbRes.Body)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(onbRes.Body.Bytes(), &created)

	secondOnb := submit(h.handler, validInput(), cookie)
	if secondOnb.Code != http.StatusConflict {
		t.Fatalf("expected 409 on second onboarding, got %d %s", secondOnb.Code, secondOnb.Body)
	}

	sessRes := doJSON(h.handler, http.MethodGet, "/api/auth/session", nil, cookie)
	if sessRes.Code != http.StatusOK || !strings.Contains(sessRes.Body.String(), created.ID) {
		t.Fatalf("session missing contractorId: %d %s", sessRes.Code, sessRes.Body)
	}

	// Cross-account isolation: account B gets 404 on account A's onboarding & dashboard.
	cookieB := signUpAndVerify(t, h, "other@praxis.mx", "5599998888", "654321")
	crossOnb := doJSON(h.handler, http.MethodGet, "/api/onboarding/"+created.ID, nil, cookieB)
	if crossOnb.Code != http.StatusNotFound {
		t.Fatalf("cross onboarding expected 404, got %d", crossOnb.Code)
	}
	dashPath := strings.Replace(dashboardpresentation.ProfilePath, "{"+dashboardpresentation.PathParamContractorID+"}", created.ID, 1)
	crossDash := doJSON(h.handler, http.MethodGet, dashPath, nil, cookieB)
	if crossDash.Code != http.StatusNotFound {
		t.Fatalf("cross dashboard expected 404, got %d", crossDash.Code)
	}

	// Logout revokes the cookie.
	logout := doJSON(h.handler, http.MethodPost, "/api/auth/logout", nil, cookie)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout expected 204, got %d", logout.Code)
	}
	afterLogout := doJSON(h.handler, http.MethodGet, "/api/auth/session", nil, cookie)
	if afterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", afterLogout.Code)
	}

	// Login by phone & email; missing account has identical 401 message as wrong PIN.
	missingLogin := doJSON(h.handler, http.MethodPost, "/api/auth/login", map[string]string{
		"identifier": "nobody@praxis.mx", "pin": "135790",
	}, nil)
	wrongPin := doJSON(h.handler, http.MethodPost, "/api/auth/login", map[string]string{
		"identifier": "5522223333", "pin": "000000",
	}, nil)
	if missingLogin.Code != http.StatusUnauthorized || wrongPin.Code != http.StatusUnauthorized || missingLogin.Body.String() != wrongPin.Body.String() {
		t.Fatalf("login errors diverged: missing=%d %s wrong=%d %s", missingLogin.Code, missingLogin.Body, wrongPin.Code, wrongPin.Body)
	}

	// 4 more wrong attempts lock the account (total 5).
	for i := 2; i <= definitions.MaxFailedLogins; i++ {
		res := doJSON(h.handler, http.MethodPost, "/api/auth/login", map[string]string{
			"identifier": "strict@praxis.mx", "pin": "000000",
		}, nil)
		if i < definitions.MaxFailedLogins && res.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i, res.Code)
		}
		if i == definitions.MaxFailedLogins && res.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt %d: expected 429 lock, got %d %s", i, res.Code, res.Body)
		}
	}
	// Even the right PIN is rejected while locked.
	lockedRight := doJSON(h.handler, http.MethodPost, "/api/auth/login", map[string]string{
		"identifier": "strict@praxis.mx", "pin": "135790",
	}, nil)
	if lockedRight.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 while locked, got %d", lockedRight.Code)
	}
	// Advance past lockout -> succeeds.
	h.clock.Advance(definitions.LockoutDuration + time.Second)
	loginOk := doJSON(h.handler, http.MethodPost, "/api/auth/login", map[string]string{
		"identifier": "55 2222 3333", "pin": "135790",
	}, nil)
	if loginOk.Code != http.StatusOK {
		t.Fatalf("login after lockout expired: %d %s", loginOk.Code, loginOk.Body)
	}
	oldCookie := sessionCookieFrom(loginOk)

	// PIN recovery: unknown identifier still returns 202.
	recUnknown := doJSON(h.handler, http.MethodPost, "/api/auth/pin-recovery", map[string]string{
		"identifier": "unknown@praxis.mx",
	}, nil)
	if recUnknown.Code != http.StatusAccepted {
		t.Fatalf("unknown pin recovery expected 202, got %d", recUnknown.Code)
	}

	recStart := doJSON(h.handler, http.MethodPost, "/api/auth/pin-recovery", map[string]string{
		"identifier": "5522223333",
	}, nil)
	if recStart.Code != http.StatusAccepted {
		t.Fatalf("pin recovery start: %d %s", recStart.Code, recStart.Body)
	}
	recCode := h.mailer.lastCode()

	// 5 wrong recovery codes invalidate the code.
	for i := 0; i < definitions.MaxCodeAttempts; i++ {
		bad := doJSON(h.handler, http.MethodPost, "/api/auth/pin-recovery/confirm", map[string]string{
			"identifier": "strict@praxis.mx", "code": "999999", "pin": "246810",
		}, nil)
		if bad.Code != http.StatusUnprocessableEntity {
			t.Fatalf("wrong recovery code %d: expected 422, got %d", i, bad.Code)
		}
	}
	// Now even the real code is rejected because max attempts invalidated it.
	exhausted := doJSON(h.handler, http.MethodPost, "/api/auth/pin-recovery/confirm", map[string]string{
		"identifier": "strict@praxis.mx", "code": recCode, "pin": "246810",
	}, nil)
	if exhausted.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected exhausted code 422, got %d", exhausted.Code)
	}

	h.clock.Advance(61 * time.Second)
	recStart2 := doJSON(h.handler, http.MethodPost, "/api/auth/pin-recovery", map[string]string{
		"identifier": "strict@praxis.mx",
	}, nil)
	if recStart2.Code != http.StatusAccepted {
		t.Fatalf("second recovery start: %d", recStart2.Code)
	}
	recConfirm := doJSON(h.handler, http.MethodPost, "/api/auth/pin-recovery/confirm", map[string]string{
		"identifier": "strict@praxis.mx", "code": h.mailer.lastCode(), "pin": "246810",
	}, nil)
	if recConfirm.Code != http.StatusOK {
		t.Fatalf("recovery confirm: %d %s", recConfirm.Code, recConfirm.Body)
	}
	newCookie := sessionCookieFrom(recConfirm)
	if newCookie == nil {
		t.Fatal("expected fresh cookie after recovery")
	}
	// Old session must be revoked.
	oldSess := doJSON(h.handler, http.MethodGet, "/api/auth/session", nil, oldCookie)
	if oldSess.Code != http.StatusUnauthorized {
		t.Fatalf("expected old session revoked (401), got %d", oldSess.Code)
	}
	// Login with new PIN works.
	loginNew := doJSON(h.handler, http.MethodPost, "/api/auth/login", map[string]string{
		"identifier": "strict@praxis.mx", "pin": "246810",
	}, nil)
	if loginNew.Code != http.StatusOK {
		t.Fatalf("login with new PIN failed: %d %s", loginNew.Code, loginNew.Body)
	}
}

func TestDemoModeAndSeedIdempotency(t *testing.T) {
	h := newHarness(accdomain.DemoPolicy(), nil)

	// Config exposes demo credentials & flags.
	cfg := doJSON(h.handler, http.MethodGet, "/api/auth/config", nil, nil)
	if cfg.Code != http.StatusOK || !strings.Contains(cfg.Body.String(), definitions.DemoEmail) {
		t.Fatalf("config: %d %s", cfg.Code, cfg.Body)
	}

	// Demo login works without PIN and points to seeded contractor.
	demoRes := doJSON(h.handler, http.MethodPost, "/api/auth/demo", nil, nil)
	if demoRes.Code != http.StatusOK {
		t.Fatalf("demo login: %d %s", demoRes.Code, demoRes.Body)
	}
	demoCookie := sessionCookieFrom(demoRes)
	var demoPayload struct {
		Account struct {
			ContractorID string `json:"contractorId"`
			Demo         bool   `json:"demo"`
		} `json:"account"`
	}
	_ = json.Unmarshal(demoRes.Body.Bytes(), &demoPayload)
	if demoPayload.Account.ContractorID == "" || !demoPayload.Account.Demo {
		t.Fatalf("unexpected demo account: %s", demoRes.Body)
	}
	dashPath := strings.Replace(dashboardpresentation.ProfilePath, "{"+dashboardpresentation.PathParamContractorID+"}", demoPayload.Account.ContractorID, 1)
	profileRes := doJSON(h.handler, http.MethodGet, dashPath, nil, demoCookie)
	if profileRes.Code != http.StatusOK || !strings.Contains(profileRes.Body.String(), "Ana García López") {
		t.Fatalf("demo profile: %d %s", profileRes.Code, profileRes.Body)
	}

	// Demo account cannot change PIN via recovery.
	demoReset := doJSON(h.handler, http.MethodPost, "/api/auth/pin-recovery/confirm", map[string]string{
		"identifier": definitions.DemoEmail, "pin": "999999",
	}, nil)
	if demoReset.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on demo PIN reset, got %d %s", demoReset.Code, demoReset.Body)
	}

	// New account can onboard without verifying email, and devCode is exposed on demand.
	signup := doJSON(h.handler, http.MethodPost, "/api/auth/signup", map[string]string{
		"email": "nuevo@praxis.mx", "phone": "5588887777", "pin": "112233",
	}, nil)
	if signup.Code != http.StatusCreated {
		t.Fatalf("demo signup: %d %s", signup.Code, signup.Body)
	}
	cookie := sessionCookieFrom(signup)
	verif := doJSON(h.handler, http.MethodPost, "/api/auth/email-verification", nil, cookie)
	if verif.Code != http.StatusAccepted || !strings.Contains(verif.Body.String(), "devCode") {
		t.Fatalf("expected devCode in demo mode: %d %s", verif.Code, verif.Body)
	}
	onb := submit(h.handler, validInput(), cookie)
	if onb.Code != http.StatusCreated {
		t.Fatalf("demo onboarding without verification: %d %s", onb.Code, onb.Body)
	}

	// 10 wrong PIN attempts do not lock in demo mode.
	for i := 0; i < 10; i++ {
		bad := doJSON(h.handler, http.MethodPost, "/api/auth/login", map[string]string{
			"identifier": "nuevo@praxis.mx", "pin": "000000",
		}, nil)
		if bad.Code != http.StatusUnauthorized {
			t.Fatalf("demo wrong pin %d: expected 401, got %d", i, bad.Code)
		}
	}

	// PIN recovery works without code in demo mode.
	reset := doJSON(h.handler, http.MethodPost, "/api/auth/pin-recovery/confirm", map[string]string{
		"identifier": "5588887777", "pin": "445566",
	}, nil)
	if reset.Code != http.StatusOK {
		t.Fatalf("demo pin reset: %d %s", reset.Code, reset.Body)
	}

	// Idempotent SQLite seeding across restarts.
	dir := t.TempDir()
	db, err := database.OpenSQLite(filepath.Join(dir, "demo.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	clientsDB, err := database.OpenSQLite(filepath.Join(dir, "clients.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer clientsDB.Close()
	onbSQLite, err := infrastructure.NewSQLiteRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	accSQLite, err := accinfra.NewSQLiteRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	clientsSQLite, err := clientsinfra.NewSQLiteRepository(clientsDB)
	if err != nil {
		t.Fatal(err)
	}
	for run := 1; run <= 2; run++ {
		_, err := NewWith(Dependencies{
			Onboardings:   onbSQLite,
			Accounts:      accSQLite,
			Clients:       clientsSQLite,
			ClientVendors: clientsinfra.NewSQLiteVendors(db),
			Banks:         infrastructure.NewUnavailablePey(),
			Hasher:        accounts.NewPBKDF2Hasher(1_000),
			Mailer:        accounts.NewLogMailer(),
			Policy:        accdomain.DemoPolicy(),
			Clock:         time.Now,
		})
		if err != nil {
			t.Fatalf("seed run %d: %v", run, err)
		}
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM accounts WHERE email = ?`, definitions.DemoEmail).Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected exactly 1 demo account, got count=%d err=%v", count, err)
	}
	var onbCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM bank_accounts WHERE clabe = ?`, demoContractorInput().CLABE).Scan(&onbCount); err != nil || onbCount != 1 {
		t.Fatalf("expected exactly 1 demo bank account, got count=%d err=%v", onbCount, err)
	}
	var clientCount int
	if err := clientsDB.QueryRow(`SELECT COUNT(*) FROM clients`).Scan(&clientCount); err != nil || clientCount != 1 {
		t.Fatalf("expected exactly 1 demo first client, got count=%d err=%v", clientCount, err)
	}
	_ = fmt.Sprintf("verified")
}

func TestPaymentRefreshesPendingCodiStatusFromPey(t *testing.T) {
	var lookedUp bool
	peyServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/bank-account":
			_, _ = writer.Write([]byte(`{"id":"01BANKPENDING","codiStatus":"PENDING"}`))
		case request.Method == http.MethodGet && request.URL.Path == "/bank-account/01BANKPENDING":
			lookedUp = true
			_, _ = writer.Write([]byte(`{"id":"01BANKPENDING","codiStatus":"ACTIVE"}`))
		case request.Method == http.MethodPost && request.URL.Path == "/codi/qr":
			_, _ = writer.Write([]byte(`{"id":"01QR","qrString":"{\"TYP\":19}","qrImage":"data:image/png;base64,AA=="}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer peyServer.Close()

	peyClient := infrastructure.NewPeyClient(peyServer.URL, "secret")
	onbRepo := infrastructure.NewMemoryRepository()
	handler, err := NewWith(Dependencies{
		Onboardings: onbRepo,
		Accounts:    accinfra.NewMemoryRepository(),
		Banks:       peyClient,
		Collector:   peyClient,
		Hasher:      accounts.NewPBKDF2Hasher(1_000),
		Mailer:      accounts.NewLogMailer(),
		Policy:      accdomain.DemoPolicy(),
		Clock:       time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}

	signup := doJSON(handler, http.MethodPost, "/api/auth/signup", map[string]string{
		"email": "david-refresh@praxis.mx", "phone": "5512345678", "pin": "123456",
	}, nil)
	if signup.Code != http.StatusCreated {
		t.Fatalf("signup: %d %s", signup.Code, signup.Body.String())
	}
	cookie := sessionCookieFrom(signup)
	onbBody := `{"name":"David","profession":"Diseñador","clabe":"002010077777777771","client":{"name":"Roberto","phone":"5512345678","email":"roberto@empresa.mx"}}`
	onbReq := httptest.NewRequest(http.MethodPost, "/api/onboarding", strings.NewReader(onbBody))
	onbReq.Header.Set("Content-Type", "application/json")
	onbReq.AddCookie(cookie)
	onbRec := httptest.NewRecorder()
	handler.ServeHTTP(onbRec, onbReq)
	if onbRec.Code != http.StatusCreated {
		t.Fatalf("onboarding expected 201, got %d: %s", onbRec.Code, onbRec.Body.String())
	}
	var created domain.Onboarding
	if err := json.Unmarshal(onbRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	payBody := `{"onboardingId":"` + created.ID + `","clientName":"Roberto","concept":"Anticipo","amountCents":1,"method":"CODI_QR"}`
	payReq := httptest.NewRequest(http.MethodPost, "/api/payments", strings.NewReader(payBody))
	payReq.Header.Set("Content-Type", "application/json")
	payReq.AddCookie(cookie)
	payRec := httptest.NewRecorder()
	handler.ServeHTTP(payRec, payReq)
	if payRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 after refreshing ACTIVE status, got %d: %s", payRec.Code, payRec.Body.String())
	}
	if !lookedUp {
		t.Fatal("expected GET /bank-account/01BANKPENDING lookup")
	}
	stored, found := onbRepo.Find(created.ID)
	if !found || stored.CodiStatus != onbdefs.CodiActive {
		t.Fatalf("expected persisted codiStatus=ACTIVE, got found=%v status=%q", found, stored.CodiStatus)
	}
}
