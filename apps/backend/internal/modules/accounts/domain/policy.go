package domain

import "strings"

// Policy controls authentication enforcement and demo conveniences.
type Policy struct {
	RequireEmailVerification bool
	Lockout                  bool
	PINResetRequiresCode     bool
	DemoAccount              bool
	ExposeCodes              bool
}

// StrictPolicy enforces email verification, login lockout, and email-code PIN recovery.
func StrictPolicy() Policy {
	return Policy{
		RequireEmailVerification: true,
		Lockout:                  true,
		PINResetRequiresCode:     true,
		DemoAccount:              false,
		ExposeCodes:              false,
	}
}

// DemoPolicy relaxes verification, lockout, and PIN recovery for demos and seeds the demo account.
func DemoPolicy() Policy {
	return Policy{
		RequireEmailVerification: false,
		Lockout:                  false,
		PINResetRequiresCode:     false,
		DemoAccount:              true,
		ExposeCodes:              true,
	}
}

// PolicyFromEnv resolves Policy from AUTH_MODE ("strict" default or "demo") plus
// optional per-flag boolean overrides, returning warnings for insecure settings.
func PolicyFromEnv(getenv func(string) string) (Policy, []string) {
	mode := strings.ToLower(strings.TrimSpace(getenv("AUTH_MODE")))
	var policy Policy
	switch mode {
	case "demo":
		policy = DemoPolicy()
	default:
		policy = StrictPolicy()
	}

	overrideBool(getenv("AUTH_REQUIRE_EMAIL_VERIFICATION"), &policy.RequireEmailVerification)
	overrideBool(getenv("AUTH_LOCKOUT"), &policy.Lockout)
	overrideBool(getenv("AUTH_PIN_RESET_REQUIRES_CODE"), &policy.PINResetRequiresCode)
	overrideBool(getenv("AUTH_DEMO_ACCOUNT"), &policy.DemoAccount)
	overrideBool(getenv("AUTH_EXPOSE_CODES"), &policy.ExposeCodes)

	var warnings []string
	if !policy.RequireEmailVerification {
		warnings = append(warnings, "AUTH_REQUIRE_EMAIL_VERIFICATION=false: accounts can use onboarding/dashboard without verifying email")
	}
	if !policy.Lockout {
		warnings = append(warnings, "AUTH_LOCKOUT=false: login attempts are not rate-limited against PIN brute force")
	}
	if !policy.PINResetRequiresCode {
		warnings = append(warnings, "AUTH_PIN_RESET_REQUIRES_CODE=false: PIN recovery resets PIN without email code (DO NOT USE WITH REAL DATA)")
	}
	if policy.DemoAccount {
		warnings = append(warnings, "AUTH_DEMO_ACCOUNT=true: demo@praxis.mx account is seeded and passwordless demo login is enabled")
	}
	if policy.ExposeCodes {
		warnings = append(warnings, "AUTH_EXPOSE_CODES=true: verification/recovery codes are returned in HTTP responses")
	}
	return policy, warnings
}

func overrideBool(raw string, target *bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		*target = true
	case "0", "false", "no", "off":
		*target = false
	}
}
