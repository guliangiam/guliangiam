package users

import (
	"context"
	"testing"
	"time"

	appdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type impersonationAppsRepo struct {
	appdomain.Repository
	listFn func(context.Context, appdomain.ListQuery) ([]*appdomain.Application, int64, error)
}

func (r *impersonationAppsRepo) List(ctx context.Context, query appdomain.ListQuery) ([]*appdomain.Application, int64, error) {
	return r.listFn(ctx, query)
}

func TestImpersonateUser_Success(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)

	caller := &userdomain.User{
		ID: "caller-id", OrganizationID: "org-1", Username: "admin", IsAdmin: true,
	}
	target := &userdomain.User{
		ID: "target-id", OrganizationID: "org-1", Username: "target-user",
		SignupApplicationID: "app-signup",
	}

	var createdToken *tokendomain.Token
	userRepo := &mockUserRepo{
		getByUsernameFn: func(_ context.Context, _, _ string) (*userdomain.User, error) {
			return caller, nil
		},
		getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
			if id == "target-id" {
				return target, nil
			}
			return nil, apperrors.NewNotFound("not found")
		},
	}
	tokenRepo := &mockTokenRepo{
		createFn: func(_ context.Context, token *tokendomain.Token) error {
			createdToken = token
			return nil
		},
	}

	uow := testUnitOfWork{users: userRepo, tokens: tokenRepo}
	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UUID:  fakeUUIDGenerator{fixedID: "token-uuid"},
		UOW:   uow,
	}

	result, err := svc.ImpersonateUser(context.Background(), "admin", "org-1", "org-1", "target-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Token == "" {
		t.Error("expected non-empty token value")
	}
	if createdToken == nil {
		t.Fatal("expected token to be created")
	}
	if createdToken.UserID != "target-id" {
		t.Errorf("token UserID = %q, want %q", createdToken.UserID, "target-id")
	}
	if createdToken.TokenPrefix != "imp" {
		t.Errorf("token prefix = %q, want %q", createdToken.TokenPrefix, "imp")
	}
	if createdToken.Revision != 1 {
		t.Errorf("token Revision = %d, want 1", createdToken.Revision)
	}
	if createdToken.ApplicationID != "app-signup" {
		t.Errorf("token ApplicationID = %q, want %q", createdToken.ApplicationID, "app-signup")
	}
}

func TestImpersonateUser_CallerNotAdmin(t *testing.T) {
	caller := &userdomain.User{
		ID: "caller-id", OrganizationID: "org-1", Username: "regular", IsAdmin: false,
	}
	userRepo := &mockUserRepo{
		getByUsernameFn: func(_ context.Context, _, _ string) (*userdomain.User, error) {
			return caller, nil
		},
	}

	uow := testUnitOfWork{users: userRepo}
	svc := Service{UOW: uow}

	_, err := svc.ImpersonateUser(context.Background(), "regular", "org-1", "org-1", "target-id")
	if err == nil {
		t.Error("expected error when caller is not admin")
	}
}

func TestImpersonateUser_CallerNotFound(t *testing.T) {
	userRepo := &mockUserRepo{
		getByUsernameFn: func(_ context.Context, _, _ string) (*userdomain.User, error) {
			return nil, apperrors.NewNotFound("not found")
		},
	}

	uow := testUnitOfWork{users: userRepo}
	svc := Service{UOW: uow}

	_, err := svc.ImpersonateUser(context.Background(), "ghost", "org-1", "org-1", "target-id")
	if err == nil {
		t.Error("expected error when caller not found")
	}
}

func TestImpersonateUser_TargetNotFound(t *testing.T) {
	caller := &userdomain.User{
		ID: "caller-id", OrganizationID: "org-1", Username: "admin", IsAdmin: true,
	}
	userRepo := &mockUserRepo{
		getByUsernameFn: func(_ context.Context, _, _ string) (*userdomain.User, error) {
			return caller, nil
		},
		getByIDFn: func(_ context.Context, _ string) (*userdomain.User, error) {
			return nil, apperrors.NewNotFound("not found")
		},
	}

	uow := testUnitOfWork{users: userRepo}
	svc := Service{UOW: uow}

	_, err := svc.ImpersonateUser(context.Background(), "admin", "org-1", "org-1", "nonexistent")
	if err == nil {
		t.Error("expected error when target user not found")
	}
}

func TestImpersonateUser_TokenCreateFailure(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	caller := &userdomain.User{
		ID: "caller-id", OrganizationID: "org-1", Username: "admin", IsAdmin: true,
	}
	target := &userdomain.User{
		ID: "target-id", OrganizationID: "org-1", Username: "target-user",
		SignupApplicationID: "app-signup",
	}

	userRepo := &mockUserRepo{
		getByUsernameFn: func(_ context.Context, _, _ string) (*userdomain.User, error) {
			return caller, nil
		},
		getByIDFn: func(_ context.Context, id string) (*userdomain.User, error) {
			return target, nil
		},
	}
	tokenRepo := &mockTokenRepo{
		createFn: func(_ context.Context, _ *tokendomain.Token) error {
			return apperrors.NewInternal("db error")
		},
	}

	uow := testUnitOfWork{users: userRepo, tokens: tokenRepo}
	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UUID:  fakeUUIDGenerator{fixedID: "token-uuid"},
		UOW:   uow,
	}

	_, err := svc.ImpersonateUser(context.Background(), "admin", "org-1", "org-1", "target-id")
	if err == nil {
		t.Error("expected error when token creation fails")
	}
}

func impersonationNoSignupAppFixture(apps *impersonationAppsRepo, createdToken **tokendomain.Token) Service {
	caller := &userdomain.User{
		ID: "caller-id", OrganizationID: "org-1", Username: "admin", IsAdmin: true,
	}
	target := &userdomain.User{
		ID: "target-id", OrganizationID: "org-1", Username: "target-user",
	}
	userRepo := &mockUserRepo{
		getByUsernameFn: func(_ context.Context, _, _ string) (*userdomain.User, error) {
			return caller, nil
		},
		getByIDFn: func(_ context.Context, _ string) (*userdomain.User, error) {
			return target, nil
		},
	}
	tokenRepo := &mockTokenRepo{
		createFn: func(_ context.Context, token *tokendomain.Token) error {
			*createdToken = token
			return nil
		},
	}
	return Service{
		Clock: fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUID:  fakeUUIDGenerator{fixedID: "token-uuid"},
		UOW:   testUnitOfWork{users: userRepo, tokens: tokenRepo, apps: apps},
	}
}

func TestImpersonateUser_FallbackToOrganizationApplication(t *testing.T) {
	var createdToken *tokendomain.Token
	var gotQuery appdomain.ListQuery
	apps := &impersonationAppsRepo{
		listFn: func(_ context.Context, query appdomain.ListQuery) ([]*appdomain.Application, int64, error) {
			gotQuery = query
			return []*appdomain.Application{{ID: "app-org-1", OrganizationID: "org-1"}}, 1, nil
		},
	}
	svc := impersonationNoSignupAppFixture(apps, &createdToken)

	result, err := svc.ImpersonateUser(context.Background(), "admin", "org-1", "org-1", "target-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Token == "" {
		t.Error("expected non-empty token value")
	}
	if createdToken == nil || createdToken.ApplicationID != "app-org-1" {
		t.Fatalf("expected token bound to org application, got %+v", createdToken)
	}
	if gotQuery.OrganizationID != "org-1" {
		t.Errorf("fallback List OrganizationID = %q, want %q", gotQuery.OrganizationID, "org-1")
	}
}

func TestImpersonateUser_NoApplicationRejected(t *testing.T) {
	var createdToken *tokendomain.Token
	apps := &impersonationAppsRepo{
		listFn: func(_ context.Context, _ appdomain.ListQuery) ([]*appdomain.Application, int64, error) {
			return nil, 0, nil
		},
	}
	svc := impersonationNoSignupAppFixture(apps, &createdToken)

	_, err := svc.ImpersonateUser(context.Background(), "admin", "org-1", "org-1", "target-id")
	if err == nil {
		t.Fatal("expected rejection when organization has no applications")
	}
	if apperrors.FromError(err).Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", apperrors.FromError(err).Code, apperrors.CodeInvalidArgument)
	}
	if createdToken != nil {
		t.Error("no token should be created when application binding is missing")
	}
}
