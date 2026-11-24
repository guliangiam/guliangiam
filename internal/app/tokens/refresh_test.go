package tokens

import (
	"context"
	"errors"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// RefreshAccessToken tests
// ---------------------------------------------------------------------------

func TestService_RefreshAccessToken_EmptyToken(t *testing.T) {
	svc := Service{Clock: fakeClock{}, UUID: &fakeUUIDGenerator{fixedID: "test"}, UOW: testUnitOfWork{}}

	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{RefreshToken: ""})
	if err == nil {
		t.Fatal("RefreshAccessToken() expected error for empty token, got nil")
	}

	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeInvalidArgument)
	}
}

func TestService_RefreshAccessToken_WhitespaceToken(t *testing.T) {
	svc := Service{Clock: fakeClock{}, UUID: &fakeUUIDGenerator{fixedID: "test"}, UOW: testUnitOfWork{}}

	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{RefreshToken: "   "})
	if err == nil {
		t.Fatal("RefreshAccessToken() expected error for whitespace token, got nil")
	}
}

func TestService_RefreshAccessToken_InvalidRefreshToken(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{}
	orgRepo := &mockOrgRepo{}
	userRepo := &mockUserRepo{}
	appRepo := &mockAppRepo{}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "test"},
		UOW:   testUnitOfWork{tokenRepo: repo, orgRepo: orgRepo, userRepo: userRepo, appRepo: appRepo},
	}

	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		RefreshToken: "gl_prt_nonexistent",
	})
	if err == nil {
		t.Fatal("RefreshAccessToken() expected error for nonexistent refresh token, got nil")
	}
}

// ---------------------------------------------------------------------------
// Additional validateCreateTokenCommand tests
// ---------------------------------------------------------------------------

func TestService_ValidateCreateTokenCommand_NegativeExpiry(t *testing.T) {
	err := validateCreateTokenCommand(CreateTokenCommand{
		OrganizationID:   "org-1",
		UserID:           "user-1",
		ApplicationID:    "app-1",
		Scopes:           []string{"read"},
		ExpiresInSeconds: -1,
	})
	if err == nil {
		t.Fatal("expected error for negative ExpiresInSeconds, got nil")
	}
}

func TestService_ValidateCreateTokenCommand_NilScopes(t *testing.T) {
	err := validateCreateTokenCommand(CreateTokenCommand{
		OrganizationID:   "org-1",
		UserID:           "user-1",
		ApplicationID:    "app-1",
		Scopes:           nil,
		ExpiresInSeconds: 3600,
	})
	if err == nil {
		t.Fatal("expected error for nil scopes, got nil")
	}
}

// ---------------------------------------------------------------------------
// NewService test
// ---------------------------------------------------------------------------

func TestNewService(t *testing.T) {
	svc := NewService(fakeClock{}, &fakeUUIDGenerator{fixedID: "test"}, testUnitOfWork{})
	if svc.Clock == nil {
		t.Error("Clock should not be nil")
	}
	if svc.UUID == nil {
		t.Error("UUID should not be nil")
	}
	if svc.UOW == nil {
		t.Error("UOW should not be nil")
	}
}
