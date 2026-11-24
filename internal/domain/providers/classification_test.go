package providers

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func wantProviderError(t *testing.T, err error) *apperrors.Error {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	appErr, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if appErr.Code != apperrors.CodeUnprocessableEntity || appErr.HTTPStatus() != 422 {
		t.Fatalf("error = %q/%d, want unprocessable_entity/422", appErr.Code, appErr.HTTPStatus())
	}
	return appErr
}

func TestFamiliesMatchValidator(t *testing.T) {
	for _, family := range Families {
		if !family.Valid() {
			t.Errorf("Families declares %q but Valid() rejects it", family)
		}
	}
	if Family("unknown").Valid() {
		t.Error("Valid() accepted an unknown family")
	}
}

func TestProviderStatusesMatchValidator(t *testing.T) {
	for _, status := range ProviderStatuses {
		if !status.Valid() {
			t.Errorf("ProviderStatuses declares %q but Valid() rejects it", status)
		}
	}
	if Status("unknown").Valid() {
		t.Error("Valid() accepted an unknown status")
	}
}

func TestValidateFamilyRejectsUnknownWith422(t *testing.T) {
	if err := ValidateFamily(FamilyIdentity); err != nil {
		t.Fatalf("ValidateFamily(identity) = %v", err)
	}
	appErr := wantProviderError(t, ValidateFamily(Family("legacy")))
	if appErr.Meta["field"] != "family" || appErr.Meta["value"] != "legacy" {
		t.Fatalf("expected family metadata, got %v", appErr.Meta)
	}
	allowed, ok := appErr.Meta["allowed"].([]string)
	if !ok || len(allowed) != len(Families) {
		t.Fatalf("expected allowed=%v, got %v", Families, appErr.Meta["allowed"])
	}
}

func TestRegistrationValidate(t *testing.T) {
	base := Registration{
		OrganizationID: "org-1", Name: "p-1", DisplayName: "P", Kind: "github",
		Family: FamilyIdentity, Status: StatusDraft,
	}

	t.Run("valid identity", func(t *testing.T) {
		registration := base
		registration.Normalize()
		if err := registration.Validate(); err != nil {
			t.Fatalf("Validate() = %v", err)
		}
	})

	t.Run("valid payment", func(t *testing.T) {
		registration := base
		registration.Family = FamilyPayment
		registration.Variant = VariantSandbox
		if err := registration.Validate(); err != nil {
			t.Fatalf("Validate() = %v", err)
		}
	})

	t.Run("missing organizationId stays 400", func(t *testing.T) {
		registration := base
		registration.OrganizationID = ""
		err := registration.Validate()
		appErr, ok := err.(*apperrors.Error)
		if !ok || appErr.Code != apperrors.CodeInvalidArgument {
			t.Fatalf("expected invalid_argument, got %v", err)
		}
	})

	t.Run("unknown family", func(t *testing.T) {
		registration := base
		registration.Family = Family("legacy")
		appErr := wantProviderError(t, registration.Validate())
		if appErr.Meta["field"] != "family" {
			t.Fatalf("expected family metadata, got %v", appErr.Meta)
		}
	})

	t.Run("unknown status", func(t *testing.T) {
		registration := base
		registration.Status = Status("unknown")
		appErr := wantProviderError(t, registration.Validate())
		if appErr.Meta["field"] != "status" {
			t.Fatalf("expected status metadata, got %v", appErr.Meta)
		}
	})

	t.Run("payment variant missing", func(t *testing.T) {
		registration := base
		registration.Family = FamilyPayment
		appErr := wantProviderError(t, registration.Validate())
		if appErr.Meta["field"] != "variant" || len(appErr.Meta["allowed"].([]string)) != 2 {
			t.Fatalf("expected variant allowed metadata, got %v", appErr.Meta)
		}
	})

	t.Run("variant on non-payment family", func(t *testing.T) {
		registration := base
		registration.Variant = VariantProduction
		appErr := wantProviderError(t, registration.Validate())
		if appErr.Meta["field"] != "variant" || appErr.Meta["expected"] != "" {
			t.Fatalf("expected variant expected=\"\" metadata, got %v", appErr.Meta)
		}
	})
}

func TestRegistrationTransitionTo(t *testing.T) {
	if err := (&Registration{Status: StatusDraft}).TransitionTo(StatusTesting); err != nil {
		t.Fatalf("draft → testing rejected: %v", err)
	}

	appErr := wantProviderError(t, (&Registration{Status: StatusActive}).TransitionTo(StatusTesting))
	if appErr.Meta["field"] != "status" || appErr.Meta["transition"] != "testing" {
		t.Fatalf("expected status/transition metadata, got %v", appErr.Meta)
	}
	allowed, ok := appErr.Meta["allowed"].([]string)
	if !ok || len(allowed) != 1 || allowed[0] != string(StatusDisabled) {
		t.Fatalf("expected allowed=[disabled], got %v", appErr.Meta["allowed"])
	}

	appErr = wantProviderError(t, (&Registration{Status: StatusDraft}).TransitionTo(Status("unknown")))
	allowedList, ok := appErr.Meta["allowed"].([]string)
	if !ok || len(allowedList) != len(ProviderStatuses) {
		t.Fatalf("expected full status set, got %v", appErr.Meta["allowed"])
	}
}

func TestAllowedTransitionsMatchCanTransitionTo(t *testing.T) {
	for _, from := range ProviderStatuses {
		registration := Registration{Status: from}
		allowed := registration.AllowedTransitions()
		for _, to := range ProviderStatuses {
			if to == from {
				continue
			}
			want := false
			for _, value := range allowed {
				if string(to) == value {
					want = true
				}
			}
			if got := registration.CanTransitionTo(to); got != want {
				t.Errorf("CanTransitionTo(%s→%s)=%v, AllowedTransitions=%v", from, to, got, allowed)
			}
		}
	}
}
