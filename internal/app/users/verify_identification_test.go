package users

import (
	"context"
	"testing"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type fakeIDVGateway struct {
	called  bool
	request IdentityVerificationRequest
	ticket  IdentityVerificationTicket
	err     error
}

func (g *fakeIDVGateway) InitiateUserVerification(_ context.Context, request IdentityVerificationRequest) (IdentityVerificationTicket, error) {
	g.called = true
	g.request = request
	if g.err != nil {
		return IdentityVerificationTicket{}, g.err
	}
	return g.ticket, nil
}

func TestVerifyUserIdentification_StartsProviderSession(t *testing.T) {
	user := &userdomain.User{
		ID: testUserID,
		Profile: userdomain.UserProfile{
			UserID:      testUserID,
			RealName:    "Zhang San",
			IDCardType:  "ID card",
			IDCard:      "110101199001011234",
			DisplayName: "Test",
		},
		IsVerified: false,
		Revision:   1,
	}

	var updated *userdomain.User
	userRepo := &mockUserRepo{
		getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
			cp := *user
			return &cp, nil
		},
		updateFn: func(_ context.Context, u *userdomain.User) error {
			updated = u
			return nil
		},
	}

	gateway := &fakeIDVGateway{ticket: IdentityVerificationTicket{
		VerificationID: "vid-1",
		Provider:       "jumio",
		RedirectURL:    "https://example.test/verify",
	}}
	svc := Service{
		UOW:                  testUnitOfWork{users: userRepo},
		IdentityVerification: gateway,
	}

	ticket, err := svc.VerifyUserIdentification(context.Background(), "org-1", testUserID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gateway.called {
		t.Fatal("expected the provider gateway to be called")
	}
	if gateway.request.FullName != "Zhang San" || gateway.request.IDNumber != "110101199001011234" {
		t.Errorf("claimed identity not forwarded: %+v", gateway.request)
	}
	if ticket.VerificationID != "vid-1" || ticket.RedirectURL == "" {
		t.Errorf("unexpected ticket: %+v", ticket)
	}
	// Self-attestation must not flip the verified flag; only the provider
	// result may do that.
	if updated != nil {
		t.Error("user must not be updated before the provider approves")
	}
}

func TestVerifyUserIdentification_ForwardsClientMeta(t *testing.T) {
	user := &userdomain.User{
		ID: testUserID,
		Profile: userdomain.UserProfile{
			UserID: testUserID, RealName: "Zhang San", IDCardType: "ID card", IDCard: "110101199001011234",
		},
		Revision: 1,
	}
	userRepo := &mockUserRepo{
		getByIDFn: func(_ context.Context, _ string) (*userdomain.User, error) {
			cp := *user
			return &cp, nil
		},
	}
	gateway := &fakeIDVGateway{ticket: IdentityVerificationTicket{VerificationID: "vid-1", Provider: "aliyun_idv"}}
	svc := Service{UOW: testUnitOfWork{users: userRepo}, IdentityVerification: gateway}

	_, err := svc.VerifyUserIdentification(context.Background(), "org-1", testUserID, IdentityVerificationMeta{
		MetaInfo: `{"deviceType":"web"}`, Country: "CHN", ReturnURL: "https://iam.test/return",
		Action: "real_name_verify_cn",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gateway.request.MetaInfo != `{"deviceType":"web"}` {
		t.Errorf("metaInfo not forwarded: %+v", gateway.request)
	}
	if gateway.request.Country != "CHN" || gateway.request.ReturnURL != "https://iam.test/return" {
		t.Errorf("country/returnUrl not forwarded: %+v", gateway.request)
	}
	if gateway.request.Action != "real_name_verify_cn" {
		t.Errorf("action not forwarded: %+v", gateway.request)
	}
}

func TestVerifyUserIdentification_AlreadyVerified(t *testing.T) {
	user := &userdomain.User{
		ID:         testUserID,
		IsVerified: true,
		Profile:    userdomain.UserProfile{UserID: testUserID},
		Revision:   1,
	}
	userRepo := &mockUserRepo{
		getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
			return user, nil
		},
		updateFn: func(_ context.Context, _ *userdomain.User) error {
			t.Error("Update should not be called for already-verified user")
			return nil
		},
	}
	gateway := &fakeIDVGateway{}
	svc := Service{UOW: testUnitOfWork{users: userRepo}, IdentityVerification: gateway}

	ticket, err := svc.VerifyUserIdentification(context.Background(), "org-1", testUserID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ticket.AlreadyVerified {
		t.Error("expected AlreadyVerified ticket")
	}
	if gateway.called {
		t.Error("provider must not be called for an already-verified user")
	}
}

func TestVerifyUserIdentification_MissingPrerequisites(t *testing.T) {
	tests := []struct {
		name    string
		profile userdomain.UserProfile
	}{
		{name: "missing real name", profile: userdomain.UserProfile{IDCardType: "ID card", IDCard: "123"}},
		{name: "missing id card type", profile: userdomain.UserProfile{RealName: "Zhang San", IDCard: "123"}},
		{name: "missing id card", profile: userdomain.UserProfile{RealName: "Zhang San", IDCardType: "ID card"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &userdomain.User{ID: testUserID, Profile: tt.profile, Revision: 1}
			userRepo := &mockUserRepo{
				getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
					return user, nil
				},
			}
			gateway := &fakeIDVGateway{}
			svc := Service{UOW: testUnitOfWork{users: userRepo}, IdentityVerification: gateway}

			if _, err := svc.VerifyUserIdentification(context.Background(), "org-1", testUserID); err == nil {
				t.Error("expected error for missing prerequisites")
			}
			if gateway.called {
				t.Error("provider must not be called without the claimed identity")
			}
		})
	}
}

func TestVerifyUserIdentification_UserNotFound(t *testing.T) {
	userRepo := &mockUserRepo{
		getByIDFn: func(_ context.Context, _ string) (*userdomain.User, error) {
			return nil, apperrors.NewNotFound("user not found")
		},
	}
	svc := Service{UOW: testUnitOfWork{users: userRepo}, IdentityVerification: &fakeIDVGateway{}}

	if _, err := svc.VerifyUserIdentification(context.Background(), "org-1", "nonexistent"); err == nil {
		t.Error("expected error for nonexistent user")
	}
}

func TestVerifyUserIdentification_ProviderNotConfigured(t *testing.T) {
	svc := Service{UOW: testUnitOfWork{users: &mockUserRepo{}}}

	_, err := svc.VerifyUserIdentification(context.Background(), "org-1", testUserID)
	if err == nil {
		t.Fatal("expected an error when no provider is wired")
	}
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeUnavailable {
		t.Errorf("expected Unavailable, got %v", err)
	}
}
