package roles

import (
	"context"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestByIDOperations_RequireOrganizationID pins the app-layer fail-fast: a
// missing trusted tenant is rejected before the UoW or repository is touched
// (a zero-value Service has no UOW at all).
func TestByIDOperations_RequireOrganizationID(t *testing.T) {
	svc := Service{}

	tests := []struct {
		name string
		call func() error
	}{
		{"get", func() error {
			_, err := svc.GetRole(context.Background(), "", "role-1")
			return err
		}},
		{"patch", func() error {
			return svc.PatchRole(context.Background(), "  ", PatchRoleCommand{ID: "role-1"})
		}},
		{"delete", func() error {
			return svc.DeleteRole(context.Background(), "", "role-1")
		}},
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
