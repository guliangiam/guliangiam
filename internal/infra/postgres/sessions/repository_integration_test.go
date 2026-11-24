//go:build integration

package sessions

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/testutil"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestSessionRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	tc := testutil.SetupPostgres(ctx, t)
	defer tc.Cleanup(ctx)

	pool := tc.ConnectPool(ctx, t)
	repo := NewRepository(pool)

	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	t.Run("CreateAndGetByID", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedSessionPrerequisites(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		session := &domain.Session{
			ID:               "01900000-0000-7000-8000-000000000001",
			UserID:           "01900000-0000-7000-8000-000000000100",
			Username:         "testuser",
			OrganizationID:   "01900000-0000-7000-8000-000000000200",
			OrganizationSlug: "test-org",
			ApplicationID:    "01900000-0000-7000-8000-000000000300",
			ApplicationSlug:  "test-app",
			SessionState:     "active",
			IPAddress:        "127.0.0.1",
			UserAgent:        "test-agent",
			ExpiresAt:        expiresAt,
			CreatedAt:        now,
			LastSeenAt:       now,
			UpdatedAt:        now,
			Revision:         1,
		}

		if err := repo.Create(ctx, session); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByID(ctx, session.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.ID != session.ID {
			t.Errorf("got.ID = %q, want %q", got.ID, session.ID)
		}
		if got.UserID != session.UserID {
			t.Errorf("got.UserID = %q, want %q", got.UserID, session.UserID)
		}
		if got.SessionState != session.SessionState {
			t.Errorf("got.SessionState = %q, want %q", got.SessionState, session.SessionState)
		}
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedSessionPrerequisites(ctx, pool); err != nil {
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

	t.Run("Revoke", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedSessionPrerequisites(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		session := &domain.Session{
			ID:               "01900000-0000-7000-8000-000000000002",
			UserID:           "01900000-0000-7000-8000-000000000100",
			Username:         "testuser",
			OrganizationID:   "01900000-0000-7000-8000-000000000200",
			OrganizationSlug: "test-org",
			ApplicationID:    "01900000-0000-7000-8000-000000000300",
			ApplicationSlug:  "test-app",
			SessionState:     "active",
			ExpiresAt:        expiresAt,
			CreatedAt:        now,
			LastSeenAt:       now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if err := repo.Create(ctx, session); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		revokedAt := now.Add(time.Minute)
		if err := repo.Revoke(ctx, session.ID, revokedAt); err != nil {
			t.Fatalf("Revoke() error = %v", err)
		}

		got, err := repo.GetByID(ctx, session.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.RevokedAt == nil {
			t.Error("expected RevokedAt to be set, got nil")
		}
	})

	t.Run("Touch", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedSessionPrerequisites(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		session := &domain.Session{
			ID:               "01900000-0000-7000-8000-000000000003",
			UserID:           "01900000-0000-7000-8000-000000000100",
			Username:         "testuser",
			OrganizationID:   "01900000-0000-7000-8000-000000000200",
			OrganizationSlug: "test-org",
			ApplicationID:    "01900000-0000-7000-8000-000000000300",
			ApplicationSlug:  "test-app",
			SessionState:     "active",
			ExpiresAt:        expiresAt,
			CreatedAt:        now,
			LastSeenAt:       now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if err := repo.Create(ctx, session); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		touchedAt := now.Add(5 * time.Minute)
		if err := repo.Touch(ctx, session.ID, touchedAt); err != nil {
			t.Fatalf("Touch() error = %v", err)
		}
	})

	t.Run("List", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedSessionPrerequisites(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		for i := 0; i < 3; i++ {
			session := &domain.Session{
				ID:               "01900000-0000-7000-8000-00000000001" + string(rune('0'+i)),
				UserID:           "01900000-0000-7000-8000-000000000100",
				Username:         "testuser",
				OrganizationID:   "01900000-0000-7000-8000-000000000200",
				OrganizationSlug: "test-org",
				ApplicationID:    "01900000-0000-7000-8000-000000000300",
				ApplicationSlug:  "test-app",
				SessionState:     "active",
				ExpiresAt:        expiresAt,
				CreatedAt:        now,
				LastSeenAt:       now,
				UpdatedAt:        now,
				Revision:         1,
			}
			if err := repo.Create(ctx, session); err != nil {
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

	t.Run("Delete", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedSessionPrerequisites(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		session := &domain.Session{
			ID:               "01900000-0000-7000-8000-000000000004",
			UserID:           "01900000-0000-7000-8000-000000000100",
			Username:         "testuser",
			OrganizationID:   "01900000-0000-7000-8000-000000000200",
			OrganizationSlug: "test-org",
			ApplicationID:    "01900000-0000-7000-8000-000000000300",
			ApplicationSlug:  "test-app",
			SessionState:     "active",
			ExpiresAt:        expiresAt,
			CreatedAt:        now,
			LastSeenAt:       now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if err := repo.Create(ctx, session); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := repo.Delete(ctx, session.ID); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err := repo.GetByID(ctx, session.ID)
		if err == nil {
			t.Fatal("expected error after delete, got nil")
		}
	})

	t.Run("Update", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedSessionPrerequisites(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		session := &domain.Session{
			ID:               "01900000-0000-7000-8000-000000000005",
			UserID:           "01900000-0000-7000-8000-000000000100",
			Username:         "testuser",
			OrganizationID:   "01900000-0000-7000-8000-000000000200",
			OrganizationSlug: "test-org",
			ApplicationID:    "01900000-0000-7000-8000-000000000300",
			ApplicationSlug:  "test-app",
			SessionState:     "active",
			ExpiresAt:        expiresAt,
			CreatedAt:        now,
			LastSeenAt:       now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if err := repo.Create(ctx, session); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		session.SessionState = "expired"
		session.UpdatedAt = now.Add(time.Minute)
		if err := repo.Update(ctx, session); err != nil {
			t.Fatalf("Update() error = %v", err)
		}
	})
}
