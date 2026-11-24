package auth

import (
	"context"
	"testing"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func registrationSessionTestService() *Service {
	org := &orgdomain.Organization{ID: "org-1", Slug: "acme"}
	victim := &userdomain.User{ID: "user-victim", OrganizationID: "org-1", Username: "victim"}
	attacker := &userdomain.User{ID: "user-attacker", OrganizationID: "org-1", Username: "attacker"}
	return &Service{
		WebAuthnAllowedOrigins: []string{"https://console.example.com"},
		UOW: loginTestUnitOfWork{
			orgRepo: &loginTestOrgRepository{orgs: map[string]*orgdomain.Organization{"acme": org}},
			userRepo: &loginTestUserRepository{
				users: map[string]*userdomain.User{victim.ID: victim, attacker.ID: attacker},
				usersByOrg: map[string]map[string]*userdomain.User{
					"org-1": {"victim": victim, "attacker": attacker},
				},
			},
		},
	}
}

// OPEN-API-56 ①：自服务注册的身份必须来自可信会话；会话用户 A 不能在
// 请求体里指定受害者 username 完成 begin（否则可把 A 的 Passkey 登记到受害者）。
func TestWebAuthnRegistrationBegin_RejectsSessionBodyIdentityMismatch(t *testing.T) {
	svc := registrationSessionTestService()

	_, err := svc.WebAuthnRegistrationBegin(context.Background(), WebAuthnRegistrationBeginCommand{
		OrganizationSlug:      "acme",
		Username:              "victim",
		SessionUserID:         "user-attacker",
		SessionOrganizationID: "org-1",
	})
	if err == nil {
		t.Fatal("expected forbidden for victim username with attacker session")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeForbidden {
		t.Fatalf("code = %s, want forbidden", code)
	}
}

func TestWebAuthnRegistrationBegin_RejectsForeignSessionOrganization(t *testing.T) {
	svc := registrationSessionTestService()

	_, err := svc.WebAuthnRegistrationBegin(context.Background(), WebAuthnRegistrationBeginCommand{
		OrganizationSlug:      "acme",
		Username:              "attacker",
		SessionUserID:         "user-attacker",
		SessionOrganizationID: "org-other",
	})
	if err == nil {
		t.Fatal("expected forbidden when session organization differs")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeForbidden {
		t.Fatalf("code = %s, want forbidden", code)
	}
}

// 会话用户本人注册：用户名一致时 helper 解析到会话用户。
func TestRegistrationUserForCommand_UsesSessionUser(t *testing.T) {
	svc := registrationSessionTestService()
	organization, err := svc.UOW.Organizations().GetBySlug(context.Background(), "acme")
	if err != nil {
		t.Fatalf("load org: %v", err)
	}

	user, err := svc.registrationUserForCommand(context.Background(), organization, "user-attacker", "org-1", "attacker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "user-attacker" {
		t.Fatalf("resolved user = %s, want session user", user.ID)
	}

	if _, err := svc.registrationUserForCommand(context.Background(), organization, "user-attacker", "org-1", "victim"); err == nil {
		t.Fatal("expected forbidden for mismatched body username")
	}
}

// 无会话（登录流程）保持请求体身份解析；挑战绑定仍由 finish 校验。
func TestRegistrationUserForCommand_LoginFlowKeepsBodyIdentity(t *testing.T) {
	svc := registrationSessionTestService()
	organization, err := svc.UOW.Organizations().GetBySlug(context.Background(), "acme")
	if err != nil {
		t.Fatalf("load org: %v", err)
	}
	user, err := svc.registrationUserForCommand(context.Background(), organization, "", "", "victim")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "user-victim" {
		t.Fatalf("resolved user = %s, want victim", user.ID)
	}
}
