package ldaps

import (
	"context"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BE-B-17: host is required and port must be within 1..65535 on create.
// 缺 host 是 400（缺字段）；端口越界是 422（语义范围校验，手册 4.14.1）。
func TestCreateLdap_HostPortValidation(t *testing.T) {
	cases := []struct {
		name     string
		host     string
		port     int
		wantCode apperrors.Code
	}{
		{"empty host", "", 389, apperrors.CodeInvalidArgument},
		{"blank host", "   ", 389, apperrors.CodeInvalidArgument},
		{"zero port", "ldap.example.com", 0, apperrors.CodeUnprocessableEntity},
		{"negative port", "ldap.example.com", -1, apperrors.CodeUnprocessableEntity},
		{"port too large", "ldap.example.com", 65536, apperrors.CodeUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Validation runs before UUID/Clock/UOW, so a zero-value Service
			// proves invalid input never reaches persistence.
			svc := Service{}
			err := svc.CreateLdap(context.Background(), CreateLdapCommand{
				OrganizationID: "org-1", Name: "test", Host: tc.host, Port: tc.port,
			})
			require.Error(t, err)
			assert.Equal(t, tc.wantCode, apperrors.FromError(err).Code)
		})
	}
}

// BE-B-17: patch validates host/port pointers before opening a transaction.
func TestPatchLdap_HostPortValidationBeforeTransaction(t *testing.T) {
	svc := Service{} // no UOW: any transaction attempt would panic
	blank := "  "
	err := svc.PatchLdap(context.Background(), "org-1", PatchLdapCommand{ID: "l1", Host: &blank})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)

	for _, port := range []int{0, -1, 65536} {
		p := port
		err := svc.PatchLdap(context.Background(), "org-1", PatchLdapCommand{ID: "l1", Port: &p})
		require.Error(t, err)
		assert.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
	}
}

func TestValidateLdapPort_Boundaries(t *testing.T) {
	assert.NoError(t, validateLdapPort(1))
	assert.NoError(t, validateLdapPort(65535))
	assert.Error(t, validateLdapPort(0))
	assert.Error(t, validateLdapPort(65536))
}
