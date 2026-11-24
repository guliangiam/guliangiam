package ldaps

import (
	"context"
	"testing"

	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	ldapsmock "github.com/guliangiam/guliangiam/internal/domain/ldaps/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// BE-B-11: without the explicit EnableSsl flag, TestConnection keeps the
// well-known-port inference: 636 (LDAPS) still dials TLS, 389 stays plaintext.
// This pins the port-fallback half of the decision matrix; the explicit-flag
// half lives in operations_tls_test.go.
func TestService_TestConnection_PortFallbackWhenEnableSslUnset(t *testing.T) {
	cases := []struct {
		name    string
		port    int
		wantTLS bool
	}{
		{"ldaps well-known port stays TLS", 636, true},
		{"plaintext port stays plaintext", 389, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
			ldapRepo := ldapsmock.NewMockRepository(ctrl)
			mockClient := ldapsmock.NewMockLDAPTestClient(ctrl)

			svc := Service{Clock: clock.SystemClock{}, UOW: mockUOW, LDAPClient: mockClient}

			ldap := &domain.Ldap{ID: "ldap1", Host: "localhost", Port: tc.port, BindDN: "cn=admin", BindPassword: "pass", EnableSsl: false}
			mockUOW.EXPECT().Ldaps().Return(ldapRepo)
			ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "ldap1", "org1").Return(ldap, nil)
			mockClient.EXPECT().TestConnection("localhost", tc.port, "cn=admin", "pass", tc.wantTLS).Return(nil)

			require.NoError(t, svc.TestConnection(context.Background(), "ldap1", "org1"))
		})
	}
}
