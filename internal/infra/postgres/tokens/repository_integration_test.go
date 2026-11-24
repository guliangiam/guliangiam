//go:build integration

package tokens

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/testutil"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestTokenRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	tc := testutil.SetupPostgres(ctx, t)
	defer tc.Cleanup(ctx)

	pool := tc.ConnectPool(ctx, t)
	repo := NewRepository(pool)

	now := time.Now()
	expiresAt := now.Add(time.Hour)

	t.Run("CreateAndGetByID", func(t *testing.T) {
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		seedTokenPrerequisites(ctx, t, pool)

		token := &domain.Token{
			ID:               "01900000-0000-7000-8000-000000000001",
			UserID:           "01900000-0000-7000-8000-000000000100",
			Username:         "testuser",
			OrganizationID:   "01900000-0000-7000-8000-000000000200",
			OrganizationSlug: "test-org",
			ApplicationID:    "01900000-0000-7000-8000-000000000300",
			ApplicationSlug:  "test-app",
			TokenPrefix:      "atk_",
			TokenHash:        "sha256hash1234567890abcdef",
			Scopes:           []string{"openid", "profile"},
			Audience:         []string{"app-client-id"},
			ExpiresAt:        expiresAt,
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		}

		if err := repo.Create(ctx, token); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByID(ctx, token.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.ID != token.ID {
			t.Errorf("got.ID = %q, want %q", got.ID, token.ID)
		}
		if got.TokenHash != token.TokenHash {
			t.Errorf("got.TokenHash = %q, want %q", got.TokenHash, token.TokenHash)
		}
		if got.TokenPrefix != token.TokenPrefix {
			t.Errorf("got.TokenPrefix = %q, want %q", got.TokenPrefix, token.TokenPrefix)
		}
	})

	t.Run("GetByTokenHash", func(t *testing.T) {
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		seedTokenPrerequisites(ctx, t, pool)

		token := &domain.Token{
			ID:               "01900000-0000-7000-8000-000000000002",
			UserID:           "01900000-0000-7000-8000-000000000100",
			Username:         "testuser",
			OrganizationID:   "01900000-0000-7000-8000-000000000200",
			OrganizationSlug: "test-org",
			ApplicationID:    "01900000-0000-7000-8000-000000000300",
			ApplicationSlug:  "test-app",
			TokenPrefix:      "atk_",
			TokenHash:        "uniquehash4567890abcdef",
			ExpiresAt:        expiresAt,
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if err := repo.Create(ctx, token); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByTokenHash(ctx, "uniquehash4567890abcdef")
		if err != nil {
			t.Fatalf("GetByTokenHash() error = %v", err)
		}
		if got.ID != token.ID {
			t.Errorf("got.ID = %q, want %q", got.ID, token.ID)
		}
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		seedTokenPrerequisites(ctx, t, pool)

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
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		seedTokenPrerequisites(ctx, t, pool)

		token := &domain.Token{
			ID:               "01900000-0000-7000-8000-000000000003",
			UserID:           "01900000-0000-7000-8000-000000000100",
			Username:         "testuser",
			OrganizationID:   "01900000-0000-7000-8000-000000000200",
			OrganizationSlug: "test-org",
			ApplicationID:    "01900000-0000-7000-8000-000000000300",
			ApplicationSlug:  "test-app",
			TokenPrefix:      "atk_",
			TokenHash:        "revoketesthash123",
			ExpiresAt:        expiresAt,
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if err := repo.Create(ctx, token); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		revokedAt := now.Add(time.Minute)
		if err := repo.Revoke(ctx, token.ID, revokedAt); err != nil {
			t.Fatalf("Revoke() error = %v", err)
		}

		got, err := repo.GetByID(ctx, token.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.RevokedAt == nil {
			t.Error("expected RevokedAt to be set, got nil")
		}
	})

	t.Run("CreateRefreshAndGetRefreshByTokenHash", func(t *testing.T) {
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		seedTokenPrerequisites(ctx, t, pool)
		// refresh_tokens.access_token_id has an FK to access_tokens, so the pair
		// must exist before the refresh row is written.
		seedAccessToken(ctx, t, repo, "01900000-0000-7000-8000-000000000003", now, expiresAt)

		refreshToken := &domain.RefreshToken{
			ID:               "01900000-0000-7000-8000-000000000010",
			AccessTokenID:    "01900000-0000-7000-8000-000000000003",
			UserID:           "01900000-0000-7000-8000-000000000100",
			Username:         "testuser",
			OrganizationID:   "01900000-0000-7000-8000-000000000200",
			OrganizationSlug: "test-org",
			ApplicationID:    "01900000-0000-7000-8000-000000000300",
			ApplicationSlug:  "test-app",
			RefreshTokenHash: "refreshhash12345678",
			// refresh_tokens.family_id is a self-FK; a new family's root row is
			// its own first token, so the family id must equal this row's id.
			FamilyID:  "01900000-0000-7000-8000-000000000010",
			Scopes:    []string{"openid"},
			Audience:  []string{"app-client-id"},
			ExpiresAt: now.Add(7 * 24 * time.Hour),
			CreatedAt: now,
			UpdatedAt: now,
			Revision:  1,
		}

		if err := repo.CreateRefresh(ctx, refreshToken); err != nil {
			t.Fatalf("CreateRefresh() error = %v", err)
		}

		got, err := repo.GetRefreshByTokenHash(ctx, "refreshhash12345678")
		if err != nil {
			t.Fatalf("GetRefreshByTokenHash() error = %v", err)
		}
		if got.ID != refreshToken.ID {
			t.Errorf("got.ID = %q, want %q", got.ID, refreshToken.ID)
		}
		if got.FamilyID != refreshToken.FamilyID {
			t.Errorf("got.FamilyID = %q, want %q", got.FamilyID, refreshToken.FamilyID)
		}
	})

	t.Run("RevokeByFamilyID", func(t *testing.T) {
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		seedTokenPrerequisites(ctx, t, pool)
		seedAccessToken(ctx, t, repo, "01900000-0000-7000-8000-000000000003", now, expiresAt)

		// The first token inserted (id ...020) is the family root and points at
		// itself; the following two reference it. This keeps the self-FK
		// satisfiable without an extra row that would skew the revoked count.
		familyID := "01900000-0000-7000-8000-000000000020"
		for i := 0; i < 3; i++ {
			rt := &domain.RefreshToken{
				ID:               "01900000-0000-7000-8000-00000000002" + string(rune('0'+i)),
				AccessTokenID:    "01900000-0000-7000-8000-000000000003",
				UserID:           "01900000-0000-7000-8000-000000000100",
				Username:         "testuser",
				OrganizationID:   "01900000-0000-7000-8000-000000000200",
				OrganizationSlug: "test-org",
				ApplicationID:    "01900000-0000-7000-8000-000000000300",
				ApplicationSlug:  "test-app",
				RefreshTokenHash: "familyrevoketest" + string(rune('0'+i)),
				FamilyID:         familyID,
				ExpiresAt:        now.Add(7 * 24 * time.Hour),
				CreatedAt:        now,
				UpdatedAt:        now,
				Revision:         1,
			}
			if err := repo.CreateRefresh(ctx, rt); err != nil {
				t.Fatalf("CreateRefresh(%d) error = %v", i, err)
			}
		}

		revokedAt := now
		affected, err := repo.RevokeByFamilyID(ctx, familyID, revokedAt)
		if err != nil {
			t.Fatalf("RevokeByFamilyID() error = %v", err)
		}
		if affected != 3 {
			t.Errorf("affected = %d, want 3", affected)
		}
	})

	t.Run("List", func(t *testing.T) {
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		seedTokenPrerequisites(ctx, t, pool)

		for i := 0; i < 3; i++ {
			token := &domain.Token{
				ID:               "01900000-0000-7000-8000-00000000003" + string(rune('0'+i)),
				UserID:           "01900000-0000-7000-8000-000000000100",
				Username:         "testuser",
				OrganizationID:   "01900000-0000-7000-8000-000000000200",
				OrganizationSlug: "test-org",
				ApplicationID:    "01900000-0000-7000-8000-000000000300",
				ApplicationSlug:  "test-app",
				TokenPrefix:      "atk_",
				TokenHash:        "listtesthash" + string(rune('0'+i)),
				ExpiresAt:        expiresAt,
				CreatedAt:        now,
				UpdatedAt:        now,
				Revision:         1,
			}
			if err := repo.Create(ctx, token); err != nil {
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
		if err := testutil.TruncateAll(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		seedTokenPrerequisites(ctx, t, pool)

		token := &domain.Token{
			ID:               "01900000-0000-7000-8000-000000000004",
			UserID:           "01900000-0000-7000-8000-000000000100",
			Username:         "testuser",
			OrganizationID:   "01900000-0000-7000-8000-000000000200",
			OrganizationSlug: "test-org",
			ApplicationID:    "01900000-0000-7000-8000-000000000300",
			ApplicationSlug:  "test-app",
			TokenPrefix:      "atk_",
			TokenHash:        "deletetesthash123",
			ExpiresAt:        expiresAt,
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if err := repo.Create(ctx, token); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := repo.Delete(ctx, token.ID); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err := repo.GetByID(ctx, token.ID)
		if err == nil {
			t.Fatal("expected error after delete, got nil")
		}
	})
}

// seedTokenPrerequisites inserts the organization / user / application rows that
// access_tokens and refresh_tokens reference, so the subtests exercise the SQL
// that matters instead of failing on a foreign key. The ids match the literals
// the subtests use.
func seedTokenPrerequisites(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	const (
		orgID = "01900000-0000-7000-8000-000000000200"
		usrID = "01900000-0000-7000-8000-000000000100"
		appID = "01900000-0000-7000-8000-000000000300"
	)

	if _, err := pool.Exec(ctx,
		`INSERT INTO guliangiam.organizations (id, slug, display_name)
		 VALUES ($1::uuid, $2, $3) ON CONFLICT (id) DO NOTHING`,
		orgID, "test-org", "Test Org"); err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO guliangiam.users (id, organization_id, username)
		 VALUES ($1::uuid, $2::uuid, $3) ON CONFLICT (id) DO NOTHING`,
		usrID, orgID, "testuser"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO guliangiam.applications (id, organization_id, slug, display_name)
		 VALUES ($1::uuid, $2::uuid, $3, $4) ON CONFLICT (id) DO NOTHING`,
		appID, orgID, "test-app", "Test App"); err != nil {
		t.Fatalf("seed application: %v", err)
	}
}

// seedAccessToken inserts the access token a refresh token points at, so the
// refresh_tokens.access_token_id foreign key is satisfiable.
func seedAccessToken(ctx context.Context, t *testing.T, repo Repository, id string, now, expiresAt time.Time) {
	t.Helper()

	token := &domain.Token{
		ID:               id,
		UserID:           "01900000-0000-7000-8000-000000000100",
		Username:         "testuser",
		OrganizationID:   "01900000-0000-7000-8000-000000000200",
		OrganizationSlug: "test-org",
		ApplicationID:    "01900000-0000-7000-8000-000000000300",
		ApplicationSlug:  "test-app",
		TokenPrefix:      "atk_",
		TokenHash:        "paired-" + id,
		Scopes:           []string{"openid"},
		Audience:         []string{"app-client-id"},
		ExpiresAt:        expiresAt,
		CreatedAt:        now,
		UpdatedAt:        now,
		Revision:         1,
	}
	if err := repo.Create(ctx, token); err != nil {
		t.Fatalf("seed access token: %v", err)
	}
}
