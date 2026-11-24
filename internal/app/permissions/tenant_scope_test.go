package permissions

import (
	"context"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	permissiondomain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

// scopedPermissionUOW forwards the transaction to the outer wrapper so the
// scoped permission repo stays visible inside Patch/Delete transactions.
type scopedPermissionUOW struct {
	enforceUOWWithPerm
}

func (u scopedPermissionUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

// TestService_ByIDMethodsRequireOrganization pins the app-layer fail-fast: a
// missing trusted tenant is rejected before the UoW or Casbin is touched (a
// zero-value Service has no UOW at all).
func TestService_ByIDMethodsRequireOrganization(t *testing.T) {
	svc := Service{}

	tests := []struct {
		name string
		call func() error
	}{
		{"get", func() error { _, err := svc.GetPermission(context.Background(), "", "perm-1"); return err }},
		{"patch", func() error {
			return svc.PatchPermission(context.Background(), "  ", PatchPermissionCommand{ID: "perm-1"})
		}},
		{"delete", func() error { return svc.DeletePermission(context.Background(), "", "perm-1") }},
		{"get-policies", func() error {
			_, err := svc.GetPolicies(context.Background(), " ", "perm-1")
			return err
		}},
		{"update-policies", func() error {
			return svc.UpdatePolicies(context.Background(), "\t", "perm-1", nil)
		}},
		{"objects", func() error {
			_, err := svc.GetAllObjects(context.Background(), "", "perm-1")
			return err
		}},
		{"actions", func() error {
			_, err := svc.GetAllActions(context.Background(), "", "perm-1")
			return err
		}},
		{"roles", func() error {
			_, err := svc.GetAllRoles(context.Background(), "", "perm-1")
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			appErr := apperrors.FromError(err)
			assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code, "empty organization must fail fast")
		})
	}
}

// TestService_ByIDMethodsRejectForeignTenant pins that the app layer only
// resolves the permission through the scoped repository lookup.
func TestService_ByIDMethodsRejectForeignTenant(t *testing.T) {
	repo := &mockPermissionRepo{
		permissions: []*permissiondomain.Permission{
			{ID: "perm-1", OrganizationID: "org-1", Resources: []string{"enf-1"}},
		},
	}
	svc := Service{
		CasbinManager: &mockCasbinManager{},
		UOW:           scopedPermissionUOW{enforceUOWWithPerm{permRepo: repo}},
	}

	tests := []struct {
		name string
		call func() error
	}{
		{"get", func() error { _, err := svc.GetPermission(context.Background(), "org-2", "perm-1"); return err }},
		{"patch", func() error {
			return svc.PatchPermission(context.Background(), "org-2", PatchPermissionCommand{ID: "perm-1"})
		}},
		{"delete", func() error { return svc.DeletePermission(context.Background(), "org-2", "perm-1") }},
		{"get-policies", func() error {
			_, err := svc.GetPolicies(context.Background(), "org-2", "perm-1")
			return err
		}},
		{"update-policies", func() error {
			return svc.UpdatePolicies(context.Background(), "org-2", "perm-1", nil)
		}},
		{"objects", func() error {
			_, err := svc.GetAllObjects(context.Background(), "org-2", "perm-1")
			return err
		}},
		{"actions", func() error {
			_, err := svc.GetAllActions(context.Background(), "org-2", "perm-1")
			return err
		}},
		{"roles", func() error {
			_, err := svc.GetAllRoles(context.Background(), "org-2", "perm-1")
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr := apperrors.FromError(tt.call())
			assert.Equal(t, apperrors.CodeNotFound, appErr.Code, "foreign tenant must look missing")
		})
	}

	assert.Len(t, repo.permissions, 1, "foreign permission row must remain untouched")
}
