package aigateway

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ─── LlmPriceVersion.Validate ────────────────────────────────────

func TestPriceVersion_Validate_DefaultCurrency(t *testing.T) {
	pv := &LlmPriceVersion{
		ID:              "pv-1",
		OrganizationID:  "org-1",
		ModelID:         "m-1",
		InputPricePer1M: 5.0,
	}
	if err := pv.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if pv.Currency != "USD" {
		t.Errorf("expected default USD, got %s", pv.Currency)
	}
}

func TestPriceVersion_Validate_MissingID(t *testing.T) {
	pv := &LlmPriceVersion{OrganizationID: "o", ModelID: "m"}
	if err := pv.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestPriceVersion_Validate_NegativePrice(t *testing.T) {
	pv := &LlmPriceVersion{ID: "pv", OrganizationID: "o", ModelID: "m", InputPricePer1M: -1}
	if err := pv.Validate(); err == nil {
		t.Fatal("expected error for negative price")
	}
	pv.InputPricePer1M = 1
	pv.OutputPricePer1M = -1
	if err := pv.Validate(); err == nil {
		t.Fatal("expected error for negative output price")
	}
}

func TestPriceVersion_CalculateCost(t *testing.T) {
	pv := &LlmPriceVersion{
		InputPricePer1M:  10.0,
		OutputPricePer1M: 30.0,
	}
	cost := pv.CalculateCost(2000, 500)
	expected := float64(2000)/1_000_000*10.0 + float64(500)/1_000_000*30.0
	if cost != expected {
		t.Errorf("expected %.8f, got %.8f", expected, cost)
	}
	if pv.CalculateCost(0, 0) != 0 {
		t.Error("zero tokens → zero cost")
	}
}

// ─── LlmQuotaPolicy.CheckLimit ───────────────────────────────────

func TestQuotaPolicy_Active_CheckLimit_Exceeded(t *testing.T) {
	q := &LlmQuotaPolicy{
		ID:             "q-1",
		OrganizationID: "org-1",
		Scope:          QuotaScopeOrganization,
		LimitType:      LimitTypeCost,
		LimitValue:     100.0,
		Status:         QuotaStatusActive,
	}
	if err := q.CheckLimit(100.01); err == nil {
		t.Fatal("expected limit exceeded error")
	}
}

func TestQuotaPolicy_Active_CheckLimit_BelowCeiling(t *testing.T) {
	q := &LlmQuotaPolicy{
		ID:             "q-2",
		OrganizationID: "org-1",
		Scope:          QuotaScopeAPIKey,
		LimitType:      LimitTypeRPM,
		LimitValue:     60.0,
		Status:         QuotaStatusActive,
	}
	if err := q.CheckLimit(59.0); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestQuotaPolicy_Disabled_CheckLimit_Noop(t *testing.T) {
	q := &LlmQuotaPolicy{
		ID:             "q-3",
		OrganizationID: "org-1",
		Scope:          QuotaScopeAlias,
		LimitType:      LimitTypeTPM,
		LimitValue:     1000.0,
		Status:         QuotaStatusDisabled,
	}
	if err := q.CheckLimit(9999.0); err != nil {
		t.Fatalf("disabled policy should not enforce: %v", err)
	}
}

// ─── LlmQuotaPolicy.Validate ─────────────────────────────────────

func TestQuotaPolicy_Validate_Defaults(t *testing.T) {
	q := &LlmQuotaPolicy{
		ID:             "q-4",
		OrganizationID: "org-1",
		Scope:          QuotaScopeOrganization,
		LimitType:      LimitTypeCost,
		LimitValue:     10.0,
		ResetCycle:     "monthly",
	}
	if err := q.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if q.Status != QuotaStatusActive {
		t.Errorf("expected default active, got %s", q.Status)
	}
}

func TestQuotaPolicy_Validate_MissingID(t *testing.T) {
	q := &LlmQuotaPolicy{OrganizationID: "o", Scope: QuotaScopeOrganization, LimitType: LimitTypeCost, LimitValue: 1.0}
	if err := q.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestQuotaPolicy_Validate_InvalidScope(t *testing.T) {
	q := &LlmQuotaPolicy{
		ID:             "q",
		OrganizationID: "o",
		Scope:          "group",
		LimitType:      LimitTypeCost,
		LimitValue:     1.0,
		ResetCycle:     "monthly",
	}
	if err := q.Validate(); err == nil {
		t.Fatal("expected error for invalid scope")
	} else if appErr := apperrors.FromError(err); appErr.Code != apperrors.CodeUnprocessableEntity ||
		appErr.Meta["field"] != "scope" {
		t.Errorf("got code=%q meta=%v, want 422 with field=scope", appErr.Code, appErr.Meta)
	}
}

func TestQuotaPolicy_Validate_InvalidLimitType(t *testing.T) {
	q := &LlmQuotaPolicy{
		ID:             "q",
		OrganizationID: "o",
		Scope:          QuotaScopeOrganization,
		LimitType:      "daily",
		LimitValue:     1.0,
		ResetCycle:     "monthly",
	}
	if err := q.Validate(); err == nil {
		t.Fatal("expected error for invalid limit type")
	} else if appErr := apperrors.FromError(err); appErr.Code != apperrors.CodeUnprocessableEntity ||
		appErr.Meta["field"] != "limitType" {
		t.Errorf("got code=%q meta=%v, want 422 with field=limitType", appErr.Code, appErr.Meta)
	}
}

func TestQuotaPolicy_Validate_NonPositiveLimitValue(t *testing.T) {
	q := &LlmQuotaPolicy{
		ID:             "q",
		OrganizationID: "o",
		Scope:          QuotaScopeOrganization,
		LimitType:      LimitTypeCost,
		LimitValue:     0,
		ResetCycle:     "monthly",
	}
	if err := q.Validate(); err == nil {
		t.Fatal("expected error for zero limit value")
	}
}

// ─── LlmRoute.Validate ──────────────────────────────────────────

func TestLlmRoute_Validate_MissingAlias(t *testing.T) {
	r := &LlmRoute{ID: "r-1", ModelID: "m-1"}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for missing aliasId")
	}
}

func TestLlmRoute_Validate_Valid(t *testing.T) {
	r := &LlmRoute{ID: "r-1", OrganizationID: "org-1", AliasID: "a-1", ModelID: "m-1", Priority: 1}
	if err := r.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// ─── LlmModelAlias.Validate ──────────────────────────────────────

func TestModelAlias_Validate_MissingFields(t *testing.T) {
	a := &LlmModelAlias{}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

// ─── LlmModel.Validate ──────────────────────────────────────────

func TestLlmModel_Validate_Valid(t *testing.T) {
	m := &LlmModel{
		ID:                "m-1",
		OrganizationID:    "org-1",
		ProviderID:        "p-1",
		ProviderModelName: "gpt-4o",
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLlmModel_Validate_MissingProviderModelName(t *testing.T) {
	m := &LlmModel{ID: "m", OrganizationID: "o", ProviderID: "p"}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for missing ProviderModelName")
	}
}

var _ = time.Now
