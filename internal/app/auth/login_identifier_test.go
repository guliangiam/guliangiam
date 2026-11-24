package auth

import (
	"context"
	"testing"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestPasswordLogin_EmailIdentifier_Success(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	// Email is matched case-insensitively so a client may echo any casing.
	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "TESTUSER@Example.COM",
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}
	if result == nil || result.Response.Status != LoginStatusSuccess {
		t.Fatalf("Login() result = %+v, want successful login", result)
	}
	if result.Response.Account == nil || result.Response.Account.Name != testUsername {
		t.Fatalf("Login() account = %+v, want %q", result.Response.Account, testUsername)
	}
}

func TestPasswordLogin_PhoneIdentifier_Success(t *testing.T) {
	org := newTestOrg()
	user := newTestUser(func(u *userdomain.User) {
		u.Profile.Phone = "13800138000"
	})
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "13800138000",
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}
	if result == nil || result.Response.Status != LoginStatusSuccess {
		t.Fatalf("Login() result = %+v, want successful login", result)
	}
	if result.Response.Account == nil || result.Response.Account.Name != testUsername {
		t.Fatalf("Login() account = %+v, want %q", result.Response.Account, testUsername)
	}
}

func TestPasswordLogin_UnknownIdentifier_ReturnsGenericFailure(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "unknown@example.com",
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err == nil {
		t.Fatalf("Login() error = nil, want non-nil error for unknown identifier")
	}
	if result != nil {
		t.Fatalf("Login() result = %+v, want nil for unknown identifier", result)
	}

	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("Login() error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeUnauthorized {
		t.Fatalf("Login() error code = %q, want %q", typed.Code, apperrors.CodeUnauthorized)
	}
}

func TestPasswordLogin_AmbiguousPhoneIdentifier_ReturnsGenericFailure(t *testing.T) {
	org := newTestOrg()
	first := newTestUser(func(u *userdomain.User) {
		u.Profile.Phone = "13900139000"
	})
	second := newTestUser(func(u *userdomain.User) {
		u.ID = "user-2"
		u.Username = "second-user"
		u.Profile.UserID = "user-2"
		u.Profile.Email = "second@example.com"
		u.Profile.Phone = "13900139000"
		u.Credentials.UserID = "user-2"
	})
	app := newTestApp()
	uow := newTestUow(org, first, app)
	uow.userRepo.users[second.ID] = second
	uow.userRepo.usersByOrg[org.ID][second.Username] = second
	svc := newTestService(uow)

	// phone has no unique constraint; two users sharing one number must not be
	// resolved into a logged-in account.
	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "13900139000",
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err == nil {
		t.Fatalf("Login() error = nil, want non-nil error for ambiguous phone")
	}
	if result != nil {
		t.Fatalf("Login() result = %+v, want nil for ambiguous phone", result)
	}

	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("Login() error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeUnauthorized {
		t.Fatalf("Login() error code = %q, want %q", typed.Code, apperrors.CodeUnauthorized)
	}
}

func TestIdentifierMatchesUser(t *testing.T) {
	user := newTestUser(func(u *userdomain.User) {
		u.Profile.Phone = "13800138000"
	})

	cases := []struct {
		name       string
		user       *userdomain.User
		identifier string
		want       bool
	}{
		{name: "empty identifier is not checked", user: user, identifier: "", want: true},
		{name: "blank identifier is not checked", user: user, identifier: "   ", want: true},
		{name: "username exact", user: user, identifier: testUsername, want: true},
		{name: "username differs", user: user, identifier: "other", want: false},
		{name: "email case-insensitive", user: user, identifier: "TESTUSER@example.com", want: true},
		{name: "phone exact", user: user, identifier: "13800138000", want: true},
		{name: "phone differs", user: user, identifier: "13800138001", want: false},
		{name: "nil user", user: nil, identifier: testUsername, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := identifierMatchesUser(tc.user, tc.identifier); got != tc.want {
				t.Fatalf("identifierMatchesUser() = %v, want %v", got, tc.want)
			}
		})
	}
}
