package ldapsync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	usersmock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
)

// GM-PWD-1a: an LDAP-synced user authenticates against the directory and has
// no local password hash. The created aggregate must not carry a credential
// marker that a local verifier could mistake for a hash.
func TestCreateUserFromLDAP_DoesNotMarkLocalPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := usersmock.NewMockRepository(ctrl)

	var created *users.User
	userRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, user *users.User) error {
			created = user
			return nil
		})

	svc := NewService(nil, nil, userRepo)
	config := &ldaps.Ldap{ID: "ldap-1", OrganizationID: "org-1"}
	result := &ldaps.LdapSyncResult{}

	svc.createUserFromLDAP(context.Background(), config, &ldapEntry{
		Username:    "ldap-user",
		DisplayName: "LDAP User",
		Email:       "ldap-user@example.com",
	}, result)

	require.Equal(t, 1, result.UsersCreated)
	require.NotNil(t, created)
	assert.Equal(t, "ldap", created.RegisterType)
	assert.False(t, created.Credentials.HasPassword)
	assert.Empty(t, created.Credentials.PasswordHash)
	assert.Empty(t, created.Credentials.PasswordAlgo, "a provider marker must not be stored as a hash algorithm")
	assert.Empty(t, created.Credentials.UserID, "without UserID the repository skips the credential row")

	algo, ok := created.Credentials.PasswordVerificationAlgorithm()
	assert.False(t, ok, "an LDAP-only user must not be verifiable as a local password")
	assert.Empty(t, algo)
}
