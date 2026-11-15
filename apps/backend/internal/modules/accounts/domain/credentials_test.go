package domain

import (
	"contractors-platform/backend/internal/modules/accounts/definitions"
	"testing"
	"time"
)

func TestNormalizeEmail(t *testing.T) {
	email, ok := NormalizeEmail("  Ana.Garcia@Ejemplo.MX ")
	if !ok || email != "ana.garcia@ejemplo.mx" {
		t.Fatalf("unexpected email: ok=%v email=%q", ok, email)
	}
	for _, bad := range []string{"", "sin-arroba", "Ana <ana@ejemplo.mx>", "a@"} {
		if _, ok := NormalizeEmail(bad); ok {
			t.Errorf("expected invalid email for %q", bad)
		}
	}
}

func TestNormalizePhone(t *testing.T) {
	for _, raw := range []string{"5512345678", "55 1234 5678", "+52 55 1234-5678", "525512345678", "(55) 1234 5678"} {
		got, ok := NormalizePhone(raw)
		if !ok || got != "+525512345678" {
			t.Errorf("raw %q: got (%q, %v)", raw, got, ok)
		}
	}
	for _, bad := range []string{"", "123456789", "55123456789", "55abc45678", "+15512345678"} {
		if _, ok := NormalizePhone(bad); ok {
			t.Errorf("expected invalid phone for %q", bad)
		}
	}
}

func TestValidatePIN(t *testing.T) {
	if !ValidatePIN("042819") {
		t.Fatal("expected valid 6-digit PIN")
	}
	for _, bad := range []string{"", "12345", "1234567", "12345a", " 12345"} {
		if ValidatePIN(bad) {
			t.Errorf("expected invalid PIN for %q", bad)
		}
	}
}

func TestParseIdentifier(t *testing.T) {
	if kind, val, ok := ParseIdentifier("  USER@Example.com "); !ok || kind != IdentifierEmail || val != "user@example.com" {
		t.Fatalf("email identifier: %v %q %v", kind, val, ok)
	}
	if kind, val, ok := ParseIdentifier("55 9876 5432"); !ok || kind != IdentifierPhone || val != "+525598765432" {
		t.Fatalf("phone identifier: %v %q %v", kind, val, ok)
	}
	if _, _, ok := ParseIdentifier("not-a-phone"); ok {
		t.Fatal("expected invalid identifier")
	}
}

func TestLockoutRules(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	policy := StrictPolicy()
	acc := Account{ID: "acc-1"}

	for i := 1; i < definitions.MaxFailedLogins; i++ {
		var locked bool
		acc, locked = RegisterFailure(acc, now, policy)
		if locked || IsLocked(acc, now, policy) {
			t.Fatalf("locked too early on attempt %d", i)
		}
	}
	var locked bool
	acc, locked = RegisterFailure(acc, now, policy)
	if !locked || !IsLocked(acc, now, policy) {
		t.Fatal("expected account to lock on max failed logins")
	}
	if IsLocked(acc, now.Add(definitions.LockoutDuration), policy) {
		t.Fatal("expected lock to expire after LockoutDuration")
	}

	// Demo account never locks even under strict policy.
	demoAcc := Account{ID: "demo", IsDemo: true}
	for i := 0; i < 10; i++ {
		demoAcc, locked = RegisterFailure(demoAcc, now, policy)
		if locked {
			t.Fatal("demo account should never lock")
		}
	}
}

func TestPolicyFromEnv(t *testing.T) {
	strict, warnings := PolicyFromEnv(func(string) string { return "" })
	if !strict.RequireEmailVerification || !strict.Lockout || !strict.PINResetRequiresCode || strict.DemoAccount || strict.ExposeCodes {
		t.Fatalf("unexpected default strict policy: %+v", strict)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings in strict mode, got %v", warnings)
	}

	demo, demoWarnings := PolicyFromEnv(func(k string) string {
		if k == "AUTH_MODE" {
			return "demo"
		}
		return ""
	})
	if demo.RequireEmailVerification || demo.Lockout || demo.PINResetRequiresCode || !demo.DemoAccount || !demo.ExposeCodes {
		t.Fatalf("unexpected demo policy: %+v", demo)
	}
	if len(demoWarnings) != 5 {
		t.Fatalf("expected 5 warnings in demo mode, got %d: %v", len(demoWarnings), demoWarnings)
	}

	override, _ := PolicyFromEnv(func(k string) string {
		switch k {
		case "AUTH_MODE":
			return "demo"
		case "AUTH_LOCKOUT":
			return "true"
		default:
			return ""
		}
	})
	if !override.Lockout || override.RequireEmailVerification {
		t.Fatalf("override failed: %+v", override)
	}
}
