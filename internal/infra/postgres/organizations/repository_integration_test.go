//go:build integration

package organizations

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/testutil"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestOrganizationRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	tc := testutil.SetupPostgres(ctx, t)
	defer tc.Cleanup(ctx)

	pool := tc.ConnectPool(ctx, t)
	repo := NewRepository(pool)

	t.Run("CreateAndGetByID", func(t *testing.T) {
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		org := &domain.Organization{
			ID:          "01900000-0000-7000-8000-000000000001",
			Slug:        "test-org",
			DisplayName: "Test Organization",
			WebsiteURL:  "https://test.example.com",
			Status:      "active",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Revision:    1,
		}

		if err := repo.Create(ctx, org); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByID(ctx, org.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.ID != org.ID {
			t.Errorf("got.ID = %q, want %q", got.ID, org.ID)
		}
		if got.Slug != org.Slug {
			t.Errorf("got.Slug = %q, want %q", got.Slug, org.Slug)
		}
		if got.DisplayName != org.DisplayName {
			t.Errorf("got.DisplayName = %q, want %q", got.DisplayName, org.DisplayName)
		}
		if got.Status != org.Status {
			t.Errorf("got.Status = %q, want %q", got.Status, org.Status)
		}
	})

	t.Run("GetBySlug", func(t *testing.T) {
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		org := &domain.Organization{
			ID:          "01900000-0000-7000-8000-000000000002",
			Slug:        "slug-test",
			DisplayName: "Slug Test Org",
			Status:      "active",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Revision:    1,
		}

		if err := repo.Create(ctx, org); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetBySlug(ctx, "slug-test")
		if err != nil {
			t.Fatalf("GetBySlug() error = %v", err)
		}
		if got.ID != org.ID {
			t.Errorf("got.ID = %q, want %q", got.ID, org.ID)
		}
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		if err := testutil.TruncateAll(ctx, pool); err != nil {
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
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		for i, slug := range []string{"org-a", "org-b", "org-c"} {
			org := &domain.Organization{
				ID:          "01900000-0000-7000-8000-00000000001" + string(rune('0'+i)),
				Slug:        slug,
				DisplayName: "Org " + slug,
				Status:      "active",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				Revision:    1,
			}
			if err := repo.Create(ctx, org); err != nil {
				t.Fatalf("Create(%d) error = %v", i, err)
			}
		}

		items, total, err := repo.List(ctx, domain.ListQuery{Page: 1, PageSize: 10})
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
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		org := &domain.Organization{
			ID:          "01900000-0000-7000-8000-000000000003",
			Slug:        "update-test",
			DisplayName: "Before Update",
			Status:      "active",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Revision:    1,
		}
		if err := repo.Create(ctx, org); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		org.DisplayName = "After Update"
		org.UpdatedAt = time.Now()
		// 仓储 Update 的乐观锁守卫比较 Revision-1，调用方需先自增。
		org.Revision++
		if err := repo.Update(ctx, org); err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		got, err := repo.GetByID(ctx, org.ID)
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

	t.Run("UpdateSettings", func(t *testing.T) {
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		org := &domain.Organization{
			ID:          "01900000-0000-7000-8000-000000000004",
			Slug:        "settings-test",
			DisplayName: "Settings Test",
			Status:      "active",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Revision:    1,
		}
		if err := repo.Create(ctx, org); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		settings := &domain.OrganizationSettings{
			OrganizationID:     org.ID,
			MfaRememberInHours: 24,
			AccountItems:       []domain.AccountItem{{Name: "email", Visible: true}},
		}
		if err := repo.UpdateSettings(ctx, settings); err != nil {
			t.Fatalf("UpdateSettings() error = %v", err)
		}

		got, err := repo.GetByID(ctx, org.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.Settings.MfaRememberInHours != 24 {
			t.Errorf("got.Settings.MfaRememberInHours = %d, want 24", got.Settings.MfaRememberInHours)
		}
	})

	t.Run("UpdateTheme", func(t *testing.T) {
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		org := &domain.Organization{
			ID:          "01900000-0000-7000-8000-000000000005",
			Slug:        "theme-test",
			DisplayName: "Theme Test",
			Status:      "active",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Revision:    1,
		}
		if err := repo.Create(ctx, org); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		theme := &domain.OrganizationThemeSettings{
			OrganizationID: org.ID,
			ThemeName:      "dark",
		}
		if err := repo.UpdateTheme(ctx, theme); err != nil {
			t.Fatalf("UpdateTheme() error = %v", err)
		}

		got, err := repo.GetByID(ctx, org.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.Theme.ThemeName != "dark" {
			t.Errorf("got.Theme.ThemeName = %q, want %q", got.Theme.ThemeName, "dark")
		}
	})

	t.Run("UpdateSecurityPolicy", func(t *testing.T) {
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		org := &domain.Organization{
			ID:          "01900000-0000-7000-8000-000000000006",
			Slug:        "security-test",
			DisplayName: "Security Test",
			Status:      "active",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Revision:    1,
		}
		if err := repo.Create(ctx, org); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		policy := &domain.OrganizationSecurityPolicy{
			OrganizationID: org.ID,
			PasswordPolicy: map[string]any{"minLength": 12, "requireUppercase": true},
		}
		if err := repo.UpdateSecurityPolicy(ctx, policy); err != nil {
			t.Fatalf("UpdateSecurityPolicy() error = %v", err)
		}

		got, err := repo.GetByID(ctx, org.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		// Security policy should have been loaded
		if got.Security.OrganizationID != org.ID {
			t.Errorf("got.Security.OrganizationID = %q, want %q", got.Security.OrganizationID, org.ID)
		}
	})
}
