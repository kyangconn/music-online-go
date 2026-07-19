package config

import (
	"math"
	"testing"
)

func TestValidateAdminBootstrapConfigAtStartup(t *testing.T) {
	valid := AdminBootstrapConfig{
		Enabled: true, Username: "admin", Email: "admin@example.com",
		Password: "correct horse battery staple", FullName: "Administrator",
	}
	if err := ValidateAdminBootstrapConfig(valid); err != nil {
		t.Fatalf("valid bootstrap config: %v", err)
	}

	invalidEmail := valid
	invalidEmail.Email = "Administrator <admin@example.com>"
	if err := ValidateAdminBootstrapConfig(invalidEmail); err == nil {
		t.Fatal("bootstrap display-name email should be rejected before database startup")
	}
	shortPassword := valid
	shortPassword.Password = "short"
	if err := ValidateAdminBootstrapConfig(shortPassword); err == nil {
		t.Fatal("bootstrap password policy should be checked during config validation")
	}
}

func TestValidateRateLimitRejectsNonFiniteRates(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled: true, GlobalRequestsPerSecond: math.Inf(1), GlobalBurst: 10,
		AuthRequestsPerSecond: 1, AuthBurst: 2,
	}
	if err := ValidateRateLimitConfig(cfg); err == nil {
		t.Fatal("infinite request rate should not reach the runtime limiter")
	}
}

func TestValidateLibraryRejectsDurationOverflow(t *testing.T) {
	cfg := LibraryConfig{Scanner: LibraryScannerConfig{
		MaxFileSizeMB: 1, MaxTagSizeMB: 1, HashRecheckHours: int(^uint(0) >> 1),
		RetryMaxAttempts: 1, RetryInitialSeconds: 1, RetryMaxSeconds: 1,
	}}
	if err := ValidateLibraryConfig(cfg); err == nil {
		t.Fatal("duration that cannot fit time.Duration should be rejected")
	}
}
