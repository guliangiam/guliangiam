package enforcers

import (
	"context"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestService_ByIDMethodsRequireOrganization pins the app-layer fail-fast: a
// missing trusted tenant is rejected before the UoW or Casbin is touched (a
// zero-value Service has no UOW at all).
func TestService_ByIDMethodsRequireOrganization(t *testing.T) {
	svc := Service{}

	tests := []struct {
		name string
		call func() error
	}{
		{"get", func() error { _, err := svc.GetEnforcer(context.Background(), "", "enf-1"); return err }},
		{"patch", func() error {
			return svc.PatchEnforcer(context.Background(), "  ", PatchEnforcerCommand{ID: "enf-1"})
		}},
		{"delete", func() error { return svc.DeleteEnforcer(context.Background(), "", "enf-1") }},
		{"remove-policy", func() error {
			return svc.RemovePolicy(context.Background(), "", "enf-1", []string{"p"})
		}},
		{"get-policies", func() error {
			_, err := svc.GetPolicies(context.Background(), " ", "enf-1")
			return err
		}},
		{"get-filtered-policies", func() error {
			_, err := svc.GetFilteredPolicies(context.Background(), "", "enf-1", 0, "alice")
			return err
		}},
		{"update-policy", func() error {
			return svc.UpdatePolicy(context.Background(), "\t", "enf-1", []string{"old"}, []string{"new"})
		}},
		{"subjects", func() error {
			_, err := svc.GetAllSubjects(context.Background(), "", "enf-1")
			return err
		}},
		{"objects", func() error {
			_, err := svc.GetAllObjects(context.Background(), "", "enf-1")
			return err
		}},
		{"actions", func() error {
			_, err := svc.GetAllActions(context.Background(), "", "enf-1")
			return err
		}},
		{"roles", func() error {
			_, err := svc.GetAllRoles(context.Background(), "", "enf-1")
			return err
		}},
		{"export-policies", func() error {
			_, err := svc.ExportPolicies(context.Background(), "  ", "enf-1")
			return err
		}},
		{"enforce", func() error {
			_, err := svc.Enforce(context.Background(), "", EnforceCommand{ID: "enf-1"})
			return err
		}},
		{"batch-enforce", func() error {
			_, err := svc.BatchEnforce(context.Background(), " ", BatchEnforceCommand{ID: "enf-1"})
			return err
		}},
		{"add-policy", func() error {
			return svc.AddPolicy(context.Background(), "\t", "enf-1", []string{"p"})
		}},
		{"reload", func() error {
			return svc.ReloadPolicy(context.Background(), "", "enf-1")
		}},
		{"import-policies", func() error {
			return svc.ImportPolicies(context.Background(), "  ", "enf-1", [][]string{{"p"}})
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
