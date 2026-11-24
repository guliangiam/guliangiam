package auth

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	webauthnmock "github.com/guliangiam/guliangiam/internal/domain/webauthn/mock"
)

func totpOnlyMethods() []userdomain.UserMfaMethod {
	return []userdomain.UserMfaMethod{{
		ID:         "mfa-totp-1",
		MethodType: "totp",
		Enabled:    true,
		Verified:   true,
	}}
}

func activePasskeyCredential(userID string) *webauthndomain.Credential {
	return &webauthndomain.Credential{
		ID:           "cred-passkey-1",
		UserID:       userID,
		CredentialID: []byte("credential-id"),
		Status:       webauthndomain.CredentialStatusActive,
	}
}

func TestPasskeyAsSecondFactorPolicyDefaultsOff(t *testing.T) {
	cases := []struct {
		name   string
		policy map[string]any
		want   bool
	}{
		{name: "absent defaults to false", policy: nil, want: false},
		{name: "bool false", policy: map[string]any{"passkeyAsSecondFactor": false}, want: false},
		{name: "bool true", policy: map[string]any{"passkeyAsSecondFactor": true}, want: true},
		{name: "string true", policy: map[string]any{"passkeyAsSecondFactor": "true"}, want: true},
		{name: "string false", policy: map[string]any{"passkeyAsSecondFactor": "false"}, want: false},
		{name: "passkey disabled overrides switch", policy: map[string]any{
			"passkeyEnabled":        false,
			"passkeyAsSecondFactor": true,
		}, want: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := passkeyAsSecondFactorEnabled(policyTestOrg(testCase.policy)); got != testCase.want {
				t.Fatalf("passkeyAsSecondFactorEnabled() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestMfaMethodsWithPasskeySecondFactor(t *testing.T) {
	user := newTestUser()

	t.Run("switch off leaves stored methods untouched", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := webauthnmock.NewMockRepository(ctrl)
		svc := newTestService(loginTestUnitOfWork{webAuthnRepo: repo})
		org := newTestOrg()

		methods := svc.mfaMethodsWithPasskeySecondFactor(context.Background(),
			loginTestUnitOfWork{webAuthnRepo: repo}, org, user, totpOnlyMethods())
		if len(methods) != 1 || methods[0].MethodType != "totp" {
			t.Fatalf("methods = %+v, want TOTP only", methods)
		}
	})

	t.Run("active credential appends passkey and keeps TOTP", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := webauthnmock.NewMockRepository(ctrl)
		repo.EXPECT().ListByUserID(gomock.Any(), user.ID).
			Return([]*webauthndomain.Credential{activePasskeyCredential(user.ID)}, nil)
		uow := loginTestUnitOfWork{webAuthnRepo: repo}
		svc := newTestService(uow)
		org := newTestOrg(func(o *orgdomain.Organization) {
			o.Security.MfaPolicy = map[string]any{"passkeyAsSecondFactor": true}
		})

		methods := svc.mfaMethodsWithPasskeySecondFactor(context.Background(), uow, org, user, totpOnlyMethods())
		if len(methods) != 2 {
			t.Fatalf("methods = %+v, want TOTP + passkey", methods)
		}
		if methods[0].MethodType != "totp" || methods[1].MethodType != loginMfaMethodPasskey {
			t.Fatalf("methods = %+v, want TOTP then passkey", methods)
		}
	})

	t.Run("revoked credential is not offered", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := webauthnmock.NewMockRepository(ctrl)
		revoked := activePasskeyCredential(user.ID)
		revoked.Status = webauthndomain.CredentialStatusRevoked
		repo.EXPECT().ListByUserID(gomock.Any(), user.ID).
			Return([]*webauthndomain.Credential{revoked}, nil)
		uow := loginTestUnitOfWork{webAuthnRepo: repo}
		svc := newTestService(uow)
		org := newTestOrg(func(o *orgdomain.Organization) {
			o.Security.MfaPolicy = map[string]any{"passkeyAsSecondFactor": true}
		})

		methods := svc.mfaMethodsWithPasskeySecondFactor(context.Background(), uow, org, user, totpOnlyMethods())
		if len(methods) != 1 {
			t.Fatalf("methods = %+v, want TOTP only when the passkey is revoked", methods)
		}
	})

	t.Run("lookup failure degrades to stored methods", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := webauthnmock.NewMockRepository(ctrl)
		repo.EXPECT().ListByUserID(gomock.Any(), user.ID).Return(nil, context.DeadlineExceeded)
		uow := loginTestUnitOfWork{webAuthnRepo: repo}
		svc := newTestService(uow)
		org := newTestOrg(func(o *orgdomain.Organization) {
			o.Security.MfaPolicy = map[string]any{"passkeyAsSecondFactor": true}
		})

		methods := svc.mfaMethodsWithPasskeySecondFactor(context.Background(), uow, org, user, totpOnlyMethods())
		if len(methods) != 1 {
			t.Fatalf("methods = %+v, want TOTP only on lookup failure", methods)
		}
	})
}

func TestPasswordLogin_MfaRequiredOffersPasskeyWhenOrganizationOptsIn(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg(func(o *orgdomain.Organization) {
		o.Security = orgdomain.OrganizationSecurityPolicy{
			MfaPolicy: map[string]any{
				"required":              true,
				"passkeyAsSecondFactor": true,
			},
			OrganizationID: testOrgID,
		}
	})
	user := newTestUser(func(u *userdomain.User) {
		u.MfaMethods = totpOnlyMethods()
	})
	app := newTestApp()

	repo := webauthnmock.NewMockRepository(ctrl)
	studentID := user.ID
	repo.EXPECT().ListByUserID(gomock.Any(), studentID).
		Return([]*webauthndomain.Credential{activePasskeyCredential(studentID)}, nil)

	uow := newTestUow(org, user, app)
	uow.webAuthnRepo = repo
	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}
	if result.Response.Status != LoginStatusMfaRequired {
		t.Fatalf("status = %q, want %q", result.Response.Status, LoginStatusMfaRequired)
	}

	var hasTotp, hasPasskey bool
	for _, method := range result.Response.AvailableMfaMethods {
		switch method.MethodType {
		case "totp":
			hasTotp = true
		case loginMfaMethodPasskey:
			hasPasskey = true
		}
	}
	if !hasTotp {
		t.Fatalf("available methods = %+v, want TOTP fallback", result.Response.AvailableMfaMethods)
	}
	if !hasPasskey {
		t.Fatalf("available methods = %+v, want passkey second factor", result.Response.AvailableMfaMethods)
	}
}
