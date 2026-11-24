package aigateway

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestCanonicalRequest_Validate(t *testing.T) {
	mkMsg := func(role CanonicalMessageRole, text string) CanonicalMessage {
		return CanonicalMessage{Role: role, Content: []CanonicalContentBlock{{Type: ContentBlockText, Text: text}}}
	}

	cases := []struct {
		name    string
		req     *CanonicalRequest
		wantErr bool
	}{
		{"valid chat", &CanonicalRequest{ModelAlias: "m", Messages: []CanonicalMessage{mkMsg(RoleUser, "hi")}}, false},
		{"missing model alias", &CanonicalRequest{Messages: []CanonicalMessage{mkMsg(RoleUser, "hi")}}, true},
		{"missing messages", &CanonicalRequest{ModelAlias: "m"}, true},
		{"empty messages", &CanonicalRequest{ModelAlias: "m", Messages: []CanonicalMessage{}}, true},
		{"message with empty content", &CanonicalRequest{ModelAlias: "m", Messages: []CanonicalMessage{{Role: RoleUser}}}, true},
		{"message with empty role", &CanonicalRequest{ModelAlias: "m", Messages: []CanonicalMessage{{Content: []CanonicalContentBlock{{Type: ContentBlockText, Text: "hi"}}}}}, true},
		{"negative max_tokens", &CanonicalRequest{ModelAlias: "m", Messages: []CanonicalMessage{mkMsg(RoleUser, "hi")}, MaxTokens: -1}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.req.Validate()
			if (err != nil) != c.wantErr {
				t.Errorf("Validate() err=%v wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestLlmModel_Validate(t *testing.T) {
	ctx := 8192
	maxOut := 2048
	m := &LlmModel{ID: "m", OrganizationID: "o", ProviderID: "p", ProviderModelName: "gpt-4o", DisplayName: "GPT-4o", MaxContextTokens: &ctx, MaxOutputTokens: &maxOut, Status: "active"}
	if err := m.Validate(); err != nil {
		t.Errorf("unexpected: %v", err)
	}
	m.ID = ""
	if err := m.Validate(); err == nil {
		t.Error("expected error")
	}
}

func TestLlmModelAlias_Validate(t *testing.T) {
	a := &LlmModelAlias{ID: "a", OrganizationID: "o", Alias: "fast", Status: "active"}
	if err := a.Validate(); err != nil {
		t.Errorf("unexpected: %v", err)
	}
	a.Alias = ""
	if err := a.Validate(); err == nil {
		t.Error("expected error for empty alias")
	}
}

func TestLlmRoute_Validate(t *testing.T) {
	r := &LlmRoute{ID: "r", OrganizationID: "o", AliasID: "a", ModelID: "m", Status: "active"}
	if err := r.Validate(); err != nil {
		t.Errorf("unexpected: %v", err)
	}
	r.AliasID = ""
	if err := r.Validate(); err == nil {
		t.Error("expected error")
	}
}

func TestLlmQuotaPolicy_CheckLimit(t *testing.T) {
	q := &LlmQuotaPolicy{LimitValue: 100, LimitType: "requests", Status: QuotaStatusActive}
	if err := q.CheckLimit(50); err != nil {
		t.Error("50 < 100 should not exceed")
	}
	if err := q.CheckLimit(101); err == nil {
		t.Error("101 > 100 should exceed")
	}
	// disabled 不强制
	q.Status = QuotaStatusDisabled
	if err := q.CheckLimit(9999); err != nil {
		t.Error("disabled policy should not enforce")
	}
}

func TestLlmPriceVersion_CalculateCost(t *testing.T) {
	p := &LlmPriceVersion{InputPricePer1M: 3.0, OutputPricePer1M: 15.0} // $3/1M input, $15/1M output
	cost := p.CalculateCost(1000, 2000)
	want := float64(1000)*3.0/1_000_000 + float64(2000)*15.0/1_000_000
	if cost != want {
		t.Errorf("cost = %v, want %v", cost, want)
	}
}

func TestUsageLog_Validate(t *testing.T) {
	u := &UsageLog{ID: "u", OrganizationID: "o", AliasID: "a", ProviderID: "p", ModelID: "m", AttemptNo: 1, RequestID: "req-1", ProtocolFace: "openai", Status: UsageStatusSucceeded, CreatedAt: time.Now()}
	if err := u.Validate(); err != nil {
		t.Errorf("unexpected: %v", err)
	}
	u.ID = ""
	if err := u.Validate(); err == nil {
		t.Error("expected error")
	}
}

func TestAIGatewaySemanticRules_Answer422(t *testing.T) {
	checks := map[string]error{
		"canonical_messages": (&CanonicalRequest{ModelAlias: "m"}).Validate(),
		"quota_limit": (&LlmQuotaPolicy{
			ID: "q-1", OrganizationID: "org-1", Scope: QuotaScopeOrganization,
			LimitType: LimitTypeRPM, LimitValue: 0, ResetCycle: "daily",
		}).Validate(),
		"usage_tokens": (&UsageLog{
			ID: "u-1", OrganizationID: "org-1", AliasID: "a-1", ProviderID: "p-1",
			ModelID: "m-1", RequestID: "r-1", ProtocolFace: "openai",
			Status: UsageStatusSucceeded, InputTokens: -1,
		}).Validate(),
		"health_failures": (&ProviderHealth{
			ID: "h-1", ProviderID: "p-1", State: HealthStateHealthy, ConsecutiveFailures: -1,
		}).Validate(),
		"price_negative":     (&LlmPriceVersion{ID: "pv-1", OrganizationID: "org-1", ModelID: "m-1", InputPricePer1M: -1}).Validate(),
		"expired_key_revoke": (&LlmApiKey{Status: APIKeyStatusExpired}).Revoke(),
	}
	for name, err := range checks {
		t.Run(name, func(t *testing.T) {
			appErr := apperrors.FromError(err)
			if appErr.Code != apperrors.CodeUnprocessableEntity {
				t.Fatalf("code = %q, want 422 (err=%v)", appErr.Code, err)
			}
			if appErr.Meta["field"] == nil {
				t.Fatalf("meta = %v, want field", appErr.Meta)
			}
		})
	}
}
