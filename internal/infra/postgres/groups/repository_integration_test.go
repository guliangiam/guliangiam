//go:build integration

package groups

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/testutil"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestGroupRepository_Integration(t *testing.T) {
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

		group := &domain.Group{
			ID:             "01900000-0000-7000-8000-000000000001",
			OrganizationID: orgID,
			Name:           "engineering",
			DisplayName:    "Engineering Team",
			Type:           "virtual",
			IsEnabled:      true,
			IsTopGroup:     true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		if err := repo.Create(ctx, group); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByID(ctx, group.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.ID != group.ID {
			t.Errorf("got.ID = %q, want %q", got.ID, group.ID)
		}
		if got.Name != group.Name {
			t.Errorf("got.Name = %q, want %q", got.Name, group.Name)
		}
		if got.OrganizationID != group.OrganizationID {
			t.Errorf("got.OrganizationID = %q, want %q", got.OrganizationID, group.OrganizationID)
		}
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		_, err := repo.GetByID(ctx, "01900000-0000-7000-8000-999999999999")
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

		for i, name := range []string{"engineering", "design", "marketing"} {
			group := &domain.Group{
				ID:             "01900000-0000-7000-8000-00000000001" + string(rune('0'+i)),
				OrganizationID: orgID,
				Name:           name,
				DisplayName:    name + " team",
				IsEnabled:      true,
				CreatedAt:      now,
				UpdatedAt:      now,
				Revision:       1,
			}
			if err := repo.Create(ctx, group); err != nil {
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

		group := &domain.Group{
			ID:             "01900000-0000-7000-8000-000000000003",
			OrganizationID: orgID,
			Name:           "updategroup",
			DisplayName:    "Before Update",
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, group); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		group.DisplayName = "After Update"
		group.IsEnabled = false
		group.UpdatedAt = now.Add(time.Minute)
		// 仓储 Update 的乐观锁守卫比较 Revision-1，调用方需先自增。
		group.Revision++
		if err := repo.Update(ctx, group); err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		got, err := repo.GetByID(ctx, group.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.DisplayName != "After Update" {
			t.Errorf("got.DisplayName = %q, want %q", got.DisplayName, "After Update")
		}
		if got.Revision != 2 {
			t.Errorf("got.Revision = %d, want 2", got.Revision)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		group := &domain.Group{
			ID:             "01900000-0000-7000-8000-000000000004",
			OrganizationID: orgID,
			Name:           "deletegroup",
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, group); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := repo.Delete(ctx, group.ID, orgID); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err := repo.GetByID(ctx, group.ID)
		if err == nil {
			t.Fatal("expected error after delete, got nil")
		}
	})
}
