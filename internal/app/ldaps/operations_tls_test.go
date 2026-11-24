package ldaps

import (
	"context"
	"testing"

	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	ldapsmock "github.com/guliangiam/guliangiam/internal/domain/ldaps/mock"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	usersmock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// BE-B-11: EnableSsl is the operator's explicit choice and must win over port
// inference. EnableSsl=true on the plaintext port 389 used to send the bind
// password in cleartext because only 636/6360 triggered TLS.
func TestService_TestConnection_EnableSslOverridesPlaintextPort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	mockClient := ldapsmock.NewMockLDAPTestClient(ctrl)

	svc := Service{Clock: clock.SystemClock{}, UOW: mockUOW, LDAPClient: mockClient}

	ldap := &domain.Ldap{ID: "ldap1", Host: "localhost", Port: 389, BindDN: "cn=admin", BindPassword: "pass", EnableSsl: true}
	mockUOW.EXPECT().Ldaps().Return(ldapRepo)
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "ldap1", "org1").Return(ldap, nil)
	mockClient.EXPECT().TestConnection("localhost", 389, "cn=admin", "pass", true).Return(nil)

	require.NoError(t, svc.TestConnection(context.Background(), "ldap1", "org1"))
}

// The sync/preview path must carry the same explicit choice: buildSyncerConfig
// maps enable_ssl onto SyncerConfig.UseTLS instead of dropping it.
func TestService_BuildSyncerConfig_PassesEnableSsl(t *testing.T) {
	svc := Service{}
	cfg := svc.buildSyncerConfig(&domain.Ldap{
		Host: "localhost", Port: 389, BindDN: "cn=admin", BindPassword: "pass",
		BaseDN: "dc=example,dc=org", EnableSsl: true, OrganizationID: "org1",
	})
	assert.True(t, cfg.UseTLS, "EnableSsl=true must reach SyncerConfig.UseTLS")
	assert.Equal(t, 389, cfg.Port)

	plain := svc.buildSyncerConfig(&domain.Ldap{Host: "localhost", Port: 389})
	assert.False(t, plain.UseTLS, "without EnableSsl the port inference stays in charge")
}

// Preview must hand the syncer a config with UseTLS set when EnableSsl is on,
// even on port 389.
func TestService_PreviewUsers_EnableSslReachesSyncer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	mockSyncer := ldapsmock.NewMockLDAPSyncer(ctrl)

	svc := Service{Clock: clock.SystemClock{}, UOW: mockUOW, LDAPSyncer: mockSyncer}

	ldap := &domain.Ldap{ID: "ldap1", OrganizationID: "org1", Host: "localhost", Port: 389, EnableSsl: true}
	mockUOW.EXPECT().Ldaps().Return(ldapRepo)
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "ldap1", "org1").Return(ldap, nil)
	mockSyncer.EXPECT().PreviewUsersDomain(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, cfg *domain.SyncerConfig) ([]*domain.LDAPPreviewUser, error) {
			assert.True(t, cfg.UseTLS, "preview syncer config must carry UseTLS")
			return []*domain.LDAPPreviewUser{}, nil
		})
	userRepo := usersmock.NewMockRepository(ctrl)
	mockUOW.EXPECT().Users().Return(userRepo)
	userRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*userdomain.User{}, int64(0), nil)

	result, err := svc.PreviewUsers(context.Background(), "ldap1", "org1")
	require.NoError(t, err)
	assert.Equal(t, 0, result.TotalCount)
}
