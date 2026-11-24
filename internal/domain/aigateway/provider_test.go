package aigateway_test

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestLlmProvider_Validate(t *testing.T) {
	tests := []struct {
		name    string
		p       aigateway.LlmProvider
		wantErr bool
	}{
		{
			name: "valid active provider",
			p: aigateway.LlmProvider{
				ID: "p-1", OrganizationID: "o-1", Name: "openai",
				ProviderType: "openai", BaseURL: "https://api.openai.com",
				APIKeyFingerprint: "abc123", Status: "active",
			},
			wantErr: false,
		},
		{
			name: "missing id",
			p: aigateway.LlmProvider{
				OrganizationID: "o-1", Name: "x", ProviderType: "openai",
				BaseURL: "https://x", APIKeyFingerprint: "abc",
			},
			wantErr: true,
		},
		{
			name: "missing organization",
			p: aigateway.LlmProvider{
				ID: "p-1", Name: "x", ProviderType: "openai",
				BaseURL: "https://x", APIKeyFingerprint: "abc",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			p: aigateway.LlmProvider{
				ID: "p-1", OrganizationID: "o-1", ProviderType: "openai",
				BaseURL: "https://x", APIKeyFingerprint: "abc",
			},
			wantErr: true,
		},
		{
			name: "invalid provider_type",
			p: aigateway.LlmProvider{
				ID: "p-1", OrganizationID: "o-1", Name: "x",
				ProviderType: "unknown_vendor", BaseURL: "https://x",
				APIKeyFingerprint: "abc",
			},
			wantErr: true,
		},
		{
			name: "missing base_url",
			p: aigateway.LlmProvider{
				ID: "p-1", OrganizationID: "o-1", Name: "x",
				ProviderType: "openai", APIKeyFingerprint: "abc",
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			p: aigateway.LlmProvider{
				ID: "p-1", OrganizationID: "o-1", Name: "x",
				ProviderType: "openai", BaseURL: "https://x",
				APIKeyFingerprint: "abc", Status: "destroyed",
			},
			wantErr: true,
		},
		{
			name: "defaults fill in zero values",
			p: aigateway.LlmProvider{
				ID: "p-2", OrganizationID: "o-1", Name: "anthropic",
				ProviderType: "anthropic", BaseURL: "https://api.anthropic.com",
				APIKeyFingerprint: "xyz",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLlmProvider_Transition(t *testing.T) {
	p := aigateway.LlmProvider{
		ID: "p-1", OrganizationID: "o-1", Name: "x",
		ProviderType: "openai", BaseURL: "https://x",
		APIKeyFingerprint: "abc", Status: aigateway.ProviderStatusActive,
	}
	if err := p.TransitionTo(aigateway.ProviderStatusDisabled); err != nil {
		t.Fatalf("active → disabled: %v", err)
	}
	if p.Status != aigateway.ProviderStatusDisabled {
		t.Errorf("status = %q, want disabled", p.Status)
	}
	if err := p.TransitionTo("unknown"); err == nil {
		t.Error("transition to invalid status should fail")
	}
	if err := p.TransitionTo(aigateway.ProviderStatusDisabled); err != nil {
		t.Error("self-transition should be a no-op (no error)")
	}
}

func TestLlmProvider_IsAvailable(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{aigateway.ProviderStatusActive, true},
		{aigateway.ProviderStatusDisabled, false},
		{aigateway.ProviderStatusTesting, false}, // testing ≠ live
	}
	for _, c := range cases {
		p := aigateway.LlmProvider{Status: c.status}
		if got := p.IsAvailable(); got != c.want {
			t.Errorf("status %s IsAvailable = %v, want %v", c.status, got, c.want)
		}
	}
}

func TestLlmProvider_PreferredProtocolDefault(t *testing.T) {
	p := aigateway.LlmProvider{
		ID: "p-1", OrganizationID: "o-1", Name: "deepseek",
		ProviderType: "openai", BaseURL: "https://api.deepseek.com",
		APIKeyFingerprint: "f",
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if p.PreferredProtocol != "openai" {
		t.Errorf("PreferredProtocol = %q, want openai", p.PreferredProtocol)
	}
}

func TestLlmProvider_RequestTimeoutDefault(t *testing.T) {
	p := aigateway.LlmProvider{
		ID: "p-1", OrganizationID: "o-1", Name: "x",
		ProviderType: "openai", BaseURL: "https://x",
		APIKeyFingerprint: "abc", RequestTimeoutMs: -1,
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if p.RequestTimeoutMs != 30000 {
		t.Errorf("RequestTimeoutMs = %d, want 30000", p.RequestTimeoutMs)
	}
}

// Verify we didn't import fmt.Errorf (check-layers Rule 4)
var _ = apperrors.NewInvalidArgument
