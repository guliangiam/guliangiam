//go:build integration

package permissions

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/testutil"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestPermissionRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	tc := testutil.SetupPostgres(ctx, t)
	defer tc.Cleanup(ctx)

	pool := tc.ConnectPool(ctx, t)
	repo := NewRepository(pool)

	now := time.Now()
	orgID := "01900000-0000-7000-8000-000000000200"

	t.Run("CreateAndGetByID", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		perm := &domain.Permission{
			ID:             "01900000-0000-7000-8000-000000000001",
			OrganizationID: orgID,
			Name:           "users:list",
			DisplayName:    "List Users",
			Description:    "Permission to list users",
			Actions:        []string{"list"},
			ResourceType:   "users",
			Effect:         "allow",
			IsEnabled:      true,
			State:          "approved",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		if err := repo.Create(ctx, perm); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByIDForOrganization(ctx, perm.ID, orgID)
		if err != nil {
			t.Fatalf("GetByIDForOrganization() error = %v", err)
		}
		if got.ID != perm.ID {
			t.Errorf("got.ID = %q, want %q", got.ID, perm.ID)
		}
		if got.Name != perm.Name {
			t.Errorf("got.Name = %q, want %q", got.Name, perm.Name)
		}
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		_, err := repo.GetByIDForOrganization(ctx, "01900000-0000-7000-8000-999999999999", orgID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		appErr, ok := err.(*apperrors.Error)
		if !ok || appErr.Code != apperrors.CodeNotFound {
			t.Errorf("error = %v, want not_found", err)
		}
	})

	t.Run("List", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		for i, name := range []string{"users:list", "users:create", "users:delete"} {
			perm := &domain.Permission{
				ID:             "01900000-0000-7000-8000-00000000001" + string(rune('0'+i)),
				OrganizationID: orgID,
				Name:           name,
				DisplayName:    name,
				IsEnabled:      true,
				State:          "approved",
				CreatedAt:      now,
				UpdatedAt:      now,
				Revision:       1,
			}
			if err := repo.Create(ctx, perm); err != nil {
				t.Fatalf("Create(%d) error = %v", i, err)
			}
		}

		items, total, err := repo.List(ctx, domain.ListQuery{
			Page:           1,
			PageSize:       10,
			OrganizationID: orgID,
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(items) != 3 {
			t.Errorf("len(items) = %d, want 3", len(items))
		}
	})

	t.Run("Update", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		perm := &domain.Permission{
			ID:             "01900000-0000-7000-8000-000000000003",
			OrganizationID: orgID,
			Name:           "update-perm",
			DisplayName:    "Before",
			IsEnabled:      true,
			State:          "approved",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, perm); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		perm.DisplayName = "After"
		perm.IsEnabled = false
		perm.UpdatedAt = now.Add(time.Minute)
		// 仓储 Update 的乐观锁守卫比较 Revision-1，调用方需先自增。
		perm.Revision++
		if err := repo.Update(ctx, perm); err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		got, err := repo.GetByIDForOrganization(ctx, perm.ID, orgID)
		if err != nil {
			t.Fatalf("GetByIDForOrganization() error = %v", err)
		}
		if got.DisplayName != "After" {
			t.Errorf("got.DisplayName = %q, want %q", got.DisplayName, "After")
		}
		if got.Revision != 2 {
			t.Errorf("got.Revision = %d, want 2", got.Revision)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		perm := &domain.Permission{
			ID:             "01900000-0000-7000-8000-000000000004",
			OrganizationID: orgID,
			Name:           "delete-perm",
			IsEnabled:      true,
			State:          "approved",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, perm); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := repo.Delete(ctx, perm.ID, orgID); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err := repo.GetByIDForOrganization(ctx, perm.ID, orgID)
		if err == nil {
			t.Fatal("expected error after delete, got nil")
		}
	})
}
