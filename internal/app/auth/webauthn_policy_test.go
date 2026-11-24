package auth

import (
	"context"
	"testing"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func policyTestOrg(policy map[string]any) *orgdomain.Organization {
	return &orgdomain.Organization{
		ID:       "org-policy",
		Slug:     "acme",
		Security: orgdomain.OrganizationSecurityPolicy{MfaPolicy: policy},
	}
}

func policyTestService(organization *orgdomain.Organization) *Service {
	return &Service{UOW: loginTestUnitOfWork{
		orgRepo: &loginTestOrgRepository{orgs: map[string]*orgdomain.Organization{"acme": organization}},
	}}
}

func TestPasskeyEnabledPolicyParsing(t *testing.T) {
	cases := []struct {
		name   string
		policy map[string]any
		want   bool
	}{
		{name: "absent defaults to enabled", policy: nil, want: true},
		{name: "bool false", policy: map[string]any{"passkeyEnabled": false}, want: false},
		{name: "string false", policy: map[string]any{"webauthnEnabled": "false"}, want: false},
		{name: "int zero", policy: map[string]any{"passkeyEnabled": 0}, want: false},
		{name: "string true", policy: map[string]any{"passkeyEnabled": "true"}, want: true},
		{name: "bool true", policy: map[string]any{"passkeyEnabled": true}, want: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := passkeyEnabled(policyTestOrg(testCase.policy)); got != testCase.want {
				t.Fatalf("passkeyEnabled() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestWebAuthnUVRequiredMergesOrganizationAndDeploymentPolicy(t *testing.T) {
	svc := &Service{}
	if svc.webAuthnUVRequired(policyTestOrg(nil)) {
		t.Fatalf("absent policy must not require UV")
	}
	if !svc.webAuthnUVRequired(policyTestOrg(map[string]any{"requireUV": true})) {
		t.Fatalf("organization requireUV=true must require UV")
	}
	svc.WebAuthnRequireUV = true
	if !svc.webAuthnUVRequired(policyTestOrg(map[string]any{"requireUV": false})) {
		t.Fatalf("deployment switch must not be relaxed by the organization policy")
	}
}

func TestWebAuthnRegistrationBeginRejectedWhenPasskeyDisabled(t *testing.T) {
	svc := policyTestService(policyTestOrg(map[string]any{"passkeyEnabled": false}))

	_, err := svc.WebAuthnRegistrationBegin(context.Background(), WebAuthnRegistrationBeginCommand{
		OrganizationSlug: "acme",
		Username:         "jane",
	})
	if err == nil || apperrors.FromError(err).Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestWebAuthnAuthenticationBeginRejectedWhenPasskeyDisabled(t *testing.T) {
	svc := policyTestService(policyTestOrg(map[string]any{"webauthnEnabled": false}))

	_, err := svc.WebAuthnAuthenticationBegin(context.Background(), WebAuthnAuthenticationBeginCommand{
		OrganizationSlug: "acme",
		Username:         "jane",
	})
	if err == nil || apperrors.FromError(err).Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
