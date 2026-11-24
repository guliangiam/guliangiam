package aigateway

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BE-C-04：RPM/TPM/MonthlyBudget 不允许为负。
func TestLlmApiKey_Validate_NegativeLimits(t *testing.T) {
	negInt := -1
	negFloat := -0.5
	cases := []struct {
		name   string
		mutate func(*LlmApiKey)
		field  string
	}{
		{"negative rpm", func(k *LlmApiKey) { k.RPMLimit = &negInt }, "rpmLimit"},
		{"negative tpm", func(k *LlmApiKey) { k.TPMLimit = &negInt }, "tpmLimit"},
		{"negative budget", func(k *LlmApiKey) { k.MonthlyBudget = &negFloat }, "monthlyBudget"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := &LlmApiKey{ID: "k-1", OrganizationID: "org-1", Name: "n", Status: APIKeyStatusActive}
			tc.mutate(k)
			err := k.Validate()
			if err == nil {
				t.Fatal("negative limit must be rejected")
			}
			e, ok := err.(*apperrors.Error)
			if !ok || e.Code != apperrors.CodeUnprocessableEntity {
				t.Fatalf("expected 422, got %v", err)
			}
			if e.Meta["field"] != tc.field {
				t.Fatalf("expected field %q, got %v", tc.field, e.Meta["field"])
			}
		})
	}
}

func TestLlmApiKey_Validate_ZeroLimitsAllowed(t *testing.T) {
	zeroInt := 0
	zeroFloat := 0.0
	k := &LlmApiKey{ID: "k-1", OrganizationID: "org-1", Name: "n", Status: APIKeyStatusActive,
		RPMLimit: &zeroInt, TPMLimit: &zeroInt, MonthlyBudget: &zeroFloat}
	if err := k.Validate(); err != nil {
		t.Fatalf("zero limits are non-negative and must pass: %v", err)
	}
}
