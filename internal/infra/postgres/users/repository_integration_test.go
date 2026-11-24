//go:build integration

package users

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/testutil"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestUserRepository_Integration(t *testing.T) {
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

		user := &domain.User{
			ID:              "01900000-0000-7000-8000-000000000001",
			OrganizationID:  orgID,
			Username:        "testuser",
			UserType:        "normal",
			Status:          "active",
			IsAdmin:         false,
			IsForbidden:     false,
			IsOnline:        false,
			Balance:         0,
			BalanceCurrency: "USD",
			CreatedAt:       now,
			UpdatedAt:       now,
			Revision:        1,
		}

		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.ID != user.ID {
			t.Errorf("got.ID = %q, want %q", got.ID, user.ID)
		}
		if got.Username != user.Username {
			t.Errorf("got.Username = %q, want %q", got.Username, user.Username)
		}
		if got.OrganizationID != user.OrganizationID {
			t.Errorf("got.OrganizationID = %q, want %q", got.OrganizationID, user.OrganizationID)
		}
		if got.Status != user.Status {
			t.Errorf("got.Status = %q, want %q", got.Status, user.Status)
		}
	})

	t.Run("GetByUsername", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		user := &domain.User{
			ID:             "01900000-0000-7000-8000-000000000002",
			OrganizationID: orgID,
			Username:       "uniqueuser",
			UserType:       "normal",
			Status:         "active",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByUsername(ctx, orgID, "uniqueuser")
		if err != nil {
			t.Fatalf("GetByUsername() error = %v", err)
		}
		if got.ID != user.ID {
			t.Errorf("got.ID = %q, want %q", got.ID, user.ID)
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

		for i, username := range []string{"user-a", "user-b", "user-c"} {
			user := &domain.User{
				ID:             "01900000-0000-7000-8000-00000000001" + string(rune('0'+i)),
				OrganizationID: orgID,
				Username:       username,
				UserType:       "normal",
				Status:         "active",
				CreatedAt:      now,
				UpdatedAt:      now,
				Revision:       1,
			}
			if err := repo.Create(ctx, user); err != nil {
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

		user := &domain.User{
			ID:             "01900000-0000-7000-8000-000000000003",
			OrganizationID: orgID,
			Username:       "updateuser",
			UserType:       "normal",
			Status:         "active",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		user.Status = "disabled"
		user.IsAdmin = true
		user.UpdatedAt = now.Add(time.Minute)
		if err := repo.Update(ctx, user); err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		got, err := repo.GetByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.Status != "disabled" {
			t.Errorf("got.Status = %q, want %q", got.Status, "disabled")
		}
		if !got.IsAdmin {
			t.Error("got.IsAdmin = false, want true")
		}
		if got.Revision != 2 {
			t.Errorf("got.Revision = %d, want 2", got.Revision)
		}
	})

	t.Run("Update_RevisionConflict", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		user := &domain.User{
			ID:             "01900000-0000-7000-8000-000000000003",
			OrganizationID: orgID,
			Username:       "conflictuser",
			UserType:       "normal",
			Status:         "active",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		// Simulate stale revision (revision was already incremented by another update)
		user.Revision = 1 // same as current, but the update SQL checks revision-1
		user.Status = "disabled"
		if err := repo.Update(ctx, user); err != nil {
			t.Fatalf("first Update() error = %v", err)
		}

		// Now try again with stale revision — should get conflict
		user.Revision = 1 // stale
		user.Status = "banned"
		err := repo.Update(ctx, user)
		if err == nil {
			t.Fatal("expected conflict error, got nil")
		}
		appErr, ok := err.(*apperrors.Error)
		if !ok || appErr.Code != apperrors.CodeConflict {
			t.Errorf("error = %v, want conflict", err)
		}
	})

	t.Run("UpdateProfile", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		user := &domain.User{
			ID:             "01900000-0000-7000-8000-000000000004",
			OrganizationID: orgID,
			Username:       "profileuser",
			UserType:       "normal",
			Status:         "active",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		profile := &domain.UserProfile{
			UserID:      user.ID,
			DisplayName: "Test User",
			Email:       "test@example.com",
			Phone:       "+8613800138000",
		}
		if err := repo.UpdateProfile(ctx, profile); err != nil {
			t.Fatalf("UpdateProfile() error = %v", err)
		}

		got, err := repo.GetByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.Profile.DisplayName != "Test User" {
			t.Errorf("got.Profile.DisplayName = %q, want %q", got.Profile.DisplayName, "Test User")
		}
		if got.Profile.Email != "test@example.com" {
			t.Errorf("got.Profile.Email = %q, want %q", got.Profile.Email, "test@example.com")
		}
	})

	t.Run("UpdateCredentials", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		user := &domain.User{
			ID:             "01900000-0000-7000-8000-000000000005",
			OrganizationID: orgID,
			Username:       "creduser",
			UserType:       "normal",
			Status:         "active",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		cred := &domain.UserCredentials{
			UserID:            user.ID,
			HasPassword:       true,
			PasswordHash:      "$argon2id$v=19$m=65536,t=3,p=4$hash",
			PasswordAlgo:      "argon2id",
			PasswordUpdatedAt: &now,
		}
		if err := repo.UpdateCredentials(ctx, cred); err != nil {
			t.Fatalf("UpdateCredentials() error = %v", err)
		}

		got, err := repo.GetByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if !got.Credentials.HasPassword {
			t.Error("got.Credentials.HasPassword = false, want true")
		}
		if got.Credentials.PasswordAlgo != "argon2id" {
			t.Errorf("got.Credentials.PasswordAlgo = %q, want %q", got.Credentials.PasswordAlgo, "argon2id")
		}
	})

	t.Run("Delete_SoftDelete", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		user := &domain.User{
			ID:             "01900000-0000-7000-8000-000000000006",
			OrganizationID: orgID,
			Username:       "deleteuser",
			UserType:       "normal",
			Status:         "active",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := repo.Delete(ctx, user.ID, orgID); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		// Soft-deleted user should not be found by GetByID
		_, err := repo.GetByID(ctx, user.ID)
		if err == nil {
			t.Fatal("expected error after soft delete, got nil")
		}
		appErr, ok := err.(*apperrors.Error)
		if !ok || appErr.Code != apperrors.CodeNotFound {
			t.Errorf("error = %v, want not_found", err)
		}
	})

	t.Run("ReplaceIdentities", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		user := &domain.User{
			ID:             "01900000-0000-7000-8000-000000000007",
			OrganizationID: orgID,
			Username:       "identityuser",
			UserType:       "normal",
			Status:         "active",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		identities := []domain.UserIdentity{
			{
				ID:              "01900000-0000-7000-8000-000000000070",
				UserID:          user.ID,
				ProviderID:      "01900000-0000-7000-8000-000000000071",
				IdentityType:    "oauth",
				Subject:         "github-12345",
				ProviderAccount: "testuser",
				LinkedAt:        now,
			},
		}
		if err := repo.ReplaceIdentities(ctx, user.ID, identities); err != nil {
			t.Fatalf("ReplaceIdentities() error = %v", err)
		}

		got, err := repo.GetByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if len(got.Identities) != 1 {
			t.Fatalf("len(got.Identities) = %d, want 1", len(got.Identities))
		}
		if got.Identities[0].Subject != "github-12345" {
			t.Errorf("got.Identities[0].Subject = %q, want %q", got.Identities[0].Subject, "github-12345")
		}
	})

	t.Run("ReplaceMfaMethods", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		user := &domain.User{
			ID:             "01900000-0000-7000-8000-000000000008",
			OrganizationID: orgID,
			Username:       "mfauser",
			UserType:       "normal",
			Status:         "active",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		methods := []domain.UserMfaMethod{
			{
				ID:          "01900000-0000-7000-8000-000000000080",
				UserID:      user.ID,
				MethodType:  "totp",
				Enabled:     true,
				Verified:    true,
				DisplayName: "Authenticator App",
			},
		}
		if err := repo.ReplaceMfaMethods(ctx, user.ID, methods); err != nil {
			t.Fatalf("ReplaceMfaMethods() error = %v", err)
		}

		got, err := repo.GetByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if len(got.MfaMethods) != 1 {
			t.Fatalf("len(got.MfaMethods) = %d, want 1", len(got.MfaMethods))
		}
		if got.MfaMethods[0].MethodType != "totp" {
			t.Errorf("got.MfaMethods[0].MethodType = %q, want %q", got.MfaMethods[0].MethodType, "totp")
		}
	})

	t.Run("Count", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		for i := 0; i < 5; i++ {
			user := &domain.User{
				ID:             "01900000-0000-7000-8000-00000000009" + string(rune('0'+i)),
				OrganizationID: orgID,
				Username:       "countuser" + string(rune('0'+i)),
				UserType:       "normal",
				Status:         "active",
				CreatedAt:      now,
				UpdatedAt:      now,
				Revision:       1,
			}
			if err := repo.Create(ctx, user); err != nil {
				t.Fatalf("Create(%d) error = %v", i, err)
			}
		}

		count, err := repo.Count(ctx, orgID)
		if err != nil {
			t.Fatalf("Count() error = %v", err)
		}
		if count != 5 {
			t.Errorf("count = %d, want 5", count)
		}
	})
}
