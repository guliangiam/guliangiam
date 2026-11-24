//go:build integration

package webhooks

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/testutil"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestWebhookRepository_Integration(t *testing.T) {
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

		wh := &domain.Webhook{
			ID:             "01900000-0000-7000-8000-000000000001",
			OrganizationID: orgID,
			Name:           "test-webhook",
			DisplayName:    "Test Webhook",
			URL:            "https://example.com/webhook",
			Method:         "POST",
			Events:         []string{"user.login", "user.logout"},
			IsEnabled:      true,
			Secret:         "wh-secret-123",
			ContentType:    "application/json",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		if err := repo.Create(ctx, wh); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByID(ctx, wh.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.ID != wh.ID {
			t.Errorf("got.ID = %q, want %q", got.ID, wh.ID)
		}
		if got.Name != wh.Name {
			t.Errorf("got.Name = %q, want %q", got.Name, wh.Name)
		}
		if got.URL != wh.URL {
			t.Errorf("got.URL = %q, want %q", got.URL, wh.URL)
		}
		if got.Method != wh.Method {
			t.Errorf("got.Method = %q, want %q", got.Method, wh.Method)
		}
		if !got.IsEnabled {
			t.Errorf("got.IsEnabled = false, want true")
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

		for i, name := range []string{"webhook-a", "webhook-b", "webhook-c"} {
			wh := &domain.Webhook{
				ID:             "01900000-0000-7000-8000-00000000001" + string(rune('0'+i)),
				OrganizationID: orgID,
				Name:           name,
				URL:            "https://example.com/" + name,
				Method:         "POST",
				IsEnabled:      true,
				CreatedAt:      now,
				UpdatedAt:      now,
				Revision:       1,
			}
			if err := repo.Create(ctx, wh); err != nil {
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

		wh := &domain.Webhook{
			ID:             "01900000-0000-7000-8000-000000000002",
			OrganizationID: orgID,
			Name:           "update-webhook",
			DisplayName:    "Before Update",
			URL:            "https://before.example.com",
			Method:         "POST",
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, wh); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		wh.DisplayName = "After Update"
		wh.URL = "https://after.example.com"
		wh.IsEnabled = false
		wh.UpdatedAt = now.Add(time.Minute)
		// 仓储 Update 的乐观锁守卫比较 Revision-1，调用方需先自增。
		wh.Revision++
		if err := repo.Update(ctx, wh); err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		got, err := repo.GetByID(ctx, wh.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.DisplayName != "After Update" {
			t.Errorf("got.DisplayName = %q, want %q", got.DisplayName, "After Update")
		}
		if got.URL != "https://after.example.com" {
			t.Errorf("got.URL = %q, want %q", got.URL, "https://after.example.com")
		}
		if got.IsEnabled {
			t.Errorf("got.IsEnabled = true, want false")
		}
		if got.Revision != 2 {
			t.Errorf("got.Revision = %d, want 2", got.Revision)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		wh := &domain.Webhook{
			ID:             "01900000-0000-7000-8000-000000000003",
			OrganizationID: orgID,
			Name:           "delete-webhook",
			URL:            "https://delete.example.com",
			Method:         "POST",
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, wh); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := repo.Delete(ctx, wh.ID); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err := repo.GetByID(ctx, wh.ID)
		if err == nil {
			t.Fatal("expected error after delete, got nil")
		}
	})

	t.Run("ListByOrganizationAndEvent", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		wh1 := &domain.Webhook{
			ID:             "01900000-0000-7000-8000-000000000010",
			OrganizationID: orgID,
			Name:           "login-webhook",
			URL:            "https://example.com/login",
			Method:         "POST",
			Events:         []string{"user.login"},
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		wh2 := &domain.Webhook{
			ID:             "01900000-0000-7000-8000-000000000011",
			OrganizationID: orgID,
			Name:           "logout-webhook",
			URL:            "https://example.com/logout",
			Method:         "POST",
			Events:         []string{"user.logout"},
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, wh1); err != nil {
			t.Fatalf("Create(wh1) error = %v", err)
		}
		if err := repo.Create(ctx, wh2); err != nil {
			t.Fatalf("Create(wh2) error = %v", err)
		}

		items, err := repo.ListByOrganizationAndEvent(ctx, orgID, "user.login")
		if err != nil {
			t.Fatalf("ListByOrganizationAndEvent() error = %v", err)
		}
		if len(items) < 1 {
			t.Errorf("expected at least 1 webhook for user.login event, got %d", len(items))
		}
	})
}
