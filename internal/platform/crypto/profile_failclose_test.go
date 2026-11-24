package crypto

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestAssessProfileGMRefusesEachMissingRequiredCapability(t *testing.T) {
	for _, missing := range []string{
		CapabilitySM2Sign, CapabilitySM2Encrypt, CapabilitySM3, CapabilitySM4,
	} {
		t.Run(missing, func(t *testing.T) {
			report, err := AssessProfile("gm", capabilityStatuses(missing))
			if err == nil {
				t.Fatalf("gm profile with %s missing must refuse startup", missing)
			}
			var appErr *apperrors.Error
			if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnavailable {
				t.Fatalf("err = %v, want unavailable code", err)
			}
			if report.Available {
				t.Fatalf("report.Available = true, want false: %+v", report)
			}
			if len(report.Missing) != 1 || report.Missing[0] != missing {
				t.Fatalf("Missing = %v, want [%s]", report.Missing, missing)
			}
			for _, want := range []string{missing, "refusing to start", "no silent downgrade"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("err = %q, want substring %q", err.Error(), want)
				}
			}
		})
	}
}

func TestAssessProfileGMAllRequiredCapabilitiesAvailable(t *testing.T) {
	report, err := AssessProfile("gm", capabilityStatuses())
	if err != nil {
		t.Fatalf("full gm capability set must not refuse startup: %v", err)
	}
	if !report.Available {
		t.Fatalf("report.Available = false, want true: %+v", report)
	}
	if len(report.Missing) != 0 {
		t.Fatalf("Missing = %v, want none", report.Missing)
	}
	if len(report.Degraded) != 0 {
		t.Fatalf("Degraded = %v, want none", report.Degraded)
	}
	if report.Profile != string(ProfileGM) {
		t.Fatalf("Profile = %q, want gm", report.Profile)
	}
}

func TestAssessProfileGMGMTLSMissingOnlyDegrades(t *testing.T) {
	requiredOnly := []CapabilityStatus{
		{ID: CapabilitySM2Sign, Available: true},
		{ID: CapabilitySM2Encrypt, Available: true},
		{ID: CapabilitySM3, Available: true},
		{ID: CapabilitySM4, Available: true},
	}
	tests := []struct {
		name string
		caps []CapabilityStatus
	}{
		{name: "probe reports gm_tls unavailable", caps: capabilityStatuses(CapabilityGMTLS)},
		{name: "probe omits gm_tls entirely", caps: requiredOnly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := AssessProfile("gm", tt.caps)
			if err != nil {
				t.Fatalf("missing gm_tls must degrade, not refuse startup: %v", err)
			}
			if !report.Available {
				t.Fatalf("report.Available = false, want true: %+v", report)
			}
			if len(report.Missing) != 0 {
				t.Fatalf("Missing = %v, want none", report.Missing)
			}
			if len(report.Degraded) != 1 || report.Degraded[0] != CapabilityGMTLS {
				t.Fatalf("Degraded = %v, want [gm_tls]", report.Degraded)
			}
		})
	}
}

func TestAssessProfileDefaultNeverRefuses(t *testing.T) {
	tests := []struct {
		name string
		caps []CapabilityStatus
	}{
		{name: "all capabilities missing", caps: capabilityStatuses(registryCapabilityOrder...)},
		{name: "probe reports nothing", caps: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := AssessProfile("default", tt.caps)
			if err != nil {
				t.Fatalf("default profile must never refuse startup: %v", err)
			}
			if !report.Available {
				t.Fatalf("report.Available = false, want true: %+v", report)
			}
			if report.Profile != string(ProfileDefault) {
				t.Fatalf("Profile = %q, want default", report.Profile)
			}
			if len(report.Missing) != 0 || len(report.Degraded) != 0 {
				t.Fatalf("report = %+v, want no missing/degraded", report)
			}
		})
	}
}

func TestAssessProfileRejectsUnsupportedProfileWithoutFallback(t *testing.T) {
	report, err := AssessProfile("gm2", capabilityStatuses())
	if err == nil {
		t.Fatal("unsupported profile must not be accepted")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeInvalidArgument {
		t.Fatalf("err = %v, want invalid_argument code", err)
	}
	if !strings.Contains(err.Error(), "supported values: default, gm") {
		t.Fatalf("err = %q, want explicit supported values", err.Error())
	}
	if report.Profile != "" {
		t.Fatalf("unsupported profile silently fell back to %q", report.Profile)
	}
}

// GM-G8: gm 档禁止显式 AES。错误必须在启动裁决阶段（AssessProfile/
// ValidateRuntimeProfile）结构化返回，而不是等到首次请求才 panic。
func TestAssessProfileGMRejectsExplicitAES(t *testing.T) {
	for _, algorithm := range []string{"aes-256-gcm", "aes", "AES"} {
		t.Run(algorithm, func(t *testing.T) {
			t.Setenv("ENCRYPTION_ALGORITHM", algorithm)

			report, err := AssessProfile("gm", capabilityStatuses())
			if err == nil {
				t.Fatal("gm profile with an explicit AES algorithm must refuse startup")
			}
			var appErr *apperrors.Error
			if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeInvalidArgument {
				t.Fatalf("err = %v, want invalid_argument code", err)
			}
			if report.Available {
				t.Fatalf("report.Available = true, want false: %+v", report)
			}
			for _, want := range []string{"sm4-gcm", "no silent downgrade to AES"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("err = %q, want substring %q", err.Error(), want)
				}
			}
		})
	}
}

func TestAssessProfileGMWithoutAlgorithmSelectsSM4(t *testing.T) {
	t.Setenv("ENCRYPTION_ALGORITHM", "")
	report, err := AssessProfile("gm", capabilityStatuses())
	if err != nil {
		t.Fatalf("gm profile with no explicit algorithm must be accepted: %v", err)
	}
	if !report.Available || report.Profile != string(ProfileGM) {
		t.Fatalf("report = %+v, want available/gm", report)
	}
}

func TestAssessProfileDefaultKeepsAES(t *testing.T) {
	t.Setenv("ENCRYPTION_ALGORITHM", "aes-256-gcm")
	report, err := AssessProfile("default", capabilityStatuses())
	if err != nil {
		t.Fatalf("default profile must keep accepting aes-256-gcm: %v", err)
	}
	if !report.Available {
		t.Fatalf("report.Available = false, want true: %+v", report)
	}
}

func TestValidateRuntimeProfileRefusesExplicitAESUnderGM(t *testing.T) {
	t.Setenv("CRYPTO_PROFILE", "gm")
	t.Setenv("ENCRYPTION_ALGORITHM", "aes-256-gcm")
	originalProbe := detectCapabilities
	t.Cleanup(func() { detectCapabilities = originalProbe })
	detectCapabilities = func() []CapabilityStatus {
		return capabilityStatuses()
	}

	report, err := ValidateRuntimeProfile()
	if err == nil {
		t.Fatal("ValidateRuntimeProfile must refuse gm + explicit AES before the listener starts")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeInvalidArgument {
		t.Fatalf("err = %v, want invalid_argument code", err)
	}
	if report.Available {
		t.Fatalf("report.Available = true, want false: %+v", report)
	}
}

// GM-G8: gm 档省略 ENCRYPTION_ALGORITHM 时，字段加密实际选择 SM4-GCM。
func TestFieldEncryptorGMDefaultsToSM4(t *testing.T) {
	key := make([]byte, 16)
	for i := range key {
		key[i] = byte(i + 1)
	}
	t.Setenv("CRYPTO_PROFILE", "gm")
	t.Setenv("ENCRYPTION_ALGORITHM", "")
	t.Setenv("ENCRYPTION_KEY_CURRENT", base64.StdEncoding.EncodeToString(key))
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	t.Setenv("ENCRYPTION_KEY_COMPAT", "")
	t.Setenv("IS_PRODUCTION", "false")

	encryptor := NewFieldEncryptor()
	if encryptor.Algorithm() != AlgorithmSM4GCM {
		t.Fatalf("gm profile without ENCRYPTION_ALGORITHM selected %s, want SM4-GCM", algName(encryptor.Algorithm()))
	}
}

func TestValidateRuntimeProfileRefusesStartupOnMissingGMPCapability(t *testing.T) {
	t.Setenv("CRYPTO_PROFILE", "gm")
	originalProbe := detectCapabilities
	t.Cleanup(func() { detectCapabilities = originalProbe })
	detectCapabilities = func() []CapabilityStatus {
		return capabilityStatuses(CapabilitySM4)
	}

	report, err := ValidateRuntimeProfile()
	if err == nil {
		t.Fatal("gm profile with sm4 missing must refuse startup")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnavailable {
		t.Fatalf("err = %v, want unavailable code", err)
	}
	if report.Available {
		t.Fatalf("report.Available = true, want false: %+v", report)
	}
	if len(report.Missing) != 1 || report.Missing[0] != CapabilitySM4 {
		t.Fatalf("Missing = %v, want [sm4]", report.Missing)
	}
}

func TestValidateRuntimeProfileDefaultToleratesMissingCapabilities(t *testing.T) {
	t.Setenv("CRYPTO_PROFILE", "default")
	originalProbe := detectCapabilities
	t.Cleanup(func() { detectCapabilities = originalProbe })
	detectCapabilities = func() []CapabilityStatus {
		return capabilityStatuses(registryCapabilityOrder...)
	}

	report, err := ValidateRuntimeProfile()
	if err != nil {
		t.Fatalf("default profile must tolerate missing capabilities: %v", err)
	}
	if !report.Available || report.Profile != string(ProfileDefault) {
		t.Fatalf("report = %+v, want available/default", report)
	}
}
