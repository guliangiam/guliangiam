package auth

import (
	"context"
	"testing"
	"time"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	webauthnmock "github.com/guliangiam/guliangiam/internal/domain/webauthn/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func webauthnCredentialsTestService(ctrl *gomock.Controller, repo *webauthnmock.MockRepository) *Service {
	victim := &userdomain.User{ID: "user-victim", OrganizationID: "org-1", Username: "victim"}
	other := &userdomain.User{ID: "user-other", OrganizationID: "org-2", Username: "other"}
	return &Service{UOW: loginTestUnitOfWork{
		userRepo: &loginTestUserRepository{
			users: map[string]*userdomain.User{victim.ID: victim, other.ID: other},
			usersByOrg: map[string]map[string]*userdomain.User{
				"org-1": {"victim": victim},
				"org-2": {"other": other},
			},
		},
		webAuthnRepo: repo,
	}}
}

func TestListWebAuthnCredentials_ScopedToSessionUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := webauthnmock.NewMockRepository(ctrl)
	svc := webauthnCredentialsTestService(ctrl, repo)

	repo.EXPECT().ListByUserID(gomock.Any(), "user-victim").Return([]*webauthndomain.Credential{{
		ID: "cred-1", UserID: "user-victim", CredentialID: []byte{1, 2, 3}, Name: "Laptop",
		Status: webauthndomain.CredentialStatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}}, nil)

	credentials, err := svc.ListWebAuthnCredentials(context.Background(), "user-victim", "org-1")
	require.NoError(t, err)
	require.Len(t, credentials, 1)
	assert.Equal(t, "cred-1", credentials[0].ID)
	assert.Equal(t, "active", credentials[0].Status)
}

func TestListWebAuthnCredentials_RequiresSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := webauthnmock.NewMockRepository(ctrl)
	svc := webauthnCredentialsTestService(ctrl, repo)

	_, err := svc.ListWebAuthnCredentials(context.Background(), "", "org-1")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
}

func TestListWebAuthnCredentials_RejectsForeignSessionUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := webauthnmock.NewMockRepository(ctrl)
	svc := webauthnCredentialsTestService(ctrl, repo)

	_, err := svc.ListWebAuthnCredentials(context.Background(), "user-victim", "org-2")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeForbidden, apperrors.FromError(err).Code)
}

func TestRevokeWebAuthnCredential_RevokesOwnCredential(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := webauthnmock.NewMockRepository(ctrl)
	svc := webauthnCredentialsTestService(ctrl, repo)

	repo.EXPECT().ListByUserID(gomock.Any(), "user-victim").Return([]*webauthndomain.Credential{
		{ID: "cred-1", UserID: "user-victim"},
		{ID: "cred-2", UserID: "user-victim"},
	}, nil)
	repo.EXPECT().UpdateStatus(gomock.Any(), "cred-2", webauthndomain.CredentialStatusRevoked).Return(nil)

	require.NoError(t, svc.RevokeWebAuthnCredential(context.Background(), "user-victim", "org-1", "cred-2"))
}

func TestRevokeWebAuthnCredential_UnknownCredentialIsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := webauthnmock.NewMockRepository(ctrl)
	svc := webauthnCredentialsTestService(ctrl, repo)

	repo.EXPECT().ListByUserID(gomock.Any(), "user-victim").Return([]*webauthndomain.Credential{{ID: "cred-1", UserID: "user-victim"}}, nil)

	err := svc.RevokeWebAuthnCredential(context.Background(), "user-victim", "org-1", "cred-other")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}
