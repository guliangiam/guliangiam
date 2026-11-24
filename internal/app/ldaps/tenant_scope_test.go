package ldaps

import (
	"context"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestService_ByIDMethodsRequireOrganization pins the app-layer fail-fast: a
// missing trusted tenant is rejected before the UoW or any external client is
// touched (a zero-value Service has no UOW at all).
func TestService_ByIDMethodsRequireOrganization(t *testing.T) {
	svc := Service{}

	tests := []struct {
		name string
		call func() error
	}{
		{"get", func() error { _, err := svc.GetLdap(context.Background(), "ldap-1", ""); return err }},
		{"patch", func() error {
			return svc.PatchLdap(context.Background(), "  ", PatchLdapCommand{ID: "ldap-1"})
		}},
		{"delete", func() error { return svc.DeleteLdap(context.Background(), "", "ldap-1") }},
		{"sync", func() error {
			_, err := svc.SyncLdap(context.Background(), "ldap-1", nil, "")
			return err
		}},
		{"preview-users", func() error {
			_, err := svc.PreviewUsers(context.Background(), "ldap-1", "")
			return err
		}},
		{"test-connection", func() error { return svc.TestConnection(context.Background(), "ldap-1", "") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			require.Error(t, err)
			appErr := apperrors.FromError(err)
			assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
		})
	}
}
