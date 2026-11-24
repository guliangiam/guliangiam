// integration_test.go — users 仓储在真实 MySQL 8.4 + 生成 DDL 上的运行期用例。
// 仅当显式设置 MYSQL_DSN 时执行；默认 CI（无 MySQL）跳过。
package users

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	platformdb "github.com/guliangiam/guliangiam/internal/platform/db"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
)

func openIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN not set; skipping MySQL users integration test")
	}
	normalized, err := platformdb.NormalizeMySQLDSN(dsn)
	require.NoError(t, err)
	db, err := sql.Open("mysql", normalized)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, db.PingContext(ctx), "MYSQL_DSN must point at a migrated MySQL 8.4+ database")
	return db
}

func insertIntegrationOrg(t *testing.T, db *sql.DB) string {
	t.Helper()
	orgID := uuid.NewString()
	_, err := db.Exec(
		"INSERT INTO `organizations` (id, slug, display_name, status) VALUES (?, ?, ?, ?)",
		orgID, "it-"+orgID[:8], "Integration Org", "active",
	)
	require.NoError(t, err)
	return orgID
}

func cleanupIntegrationOrg(t *testing.T, db *sql.DB, orgID string) {
	t.Helper()
	// user_* 子表由 FK ON DELETE CASCADE 清理；必须物理删除 users 行后 organizations 才能删。
	_, err := db.Exec("DELETE FROM `users` WHERE organization_id = ?", orgID)
	require.NoError(t, err)
	_, err = db.Exec("DELETE FROM `organizations` WHERE id = ?", orgID)
	require.NoError(t, err)
}

func TestIntegration_UsersAggregateRoundTrip(t *testing.T) {
	db := openIntegrationDB(t)
	ctx := context.Background()

	orgID := insertIntegrationOrg(t, db)
	t.Cleanup(func() { cleanupIntegrationOrg(t, db, orgID) })

	now := time.Now().UTC().Truncate(time.Second)
	userID := uuid.NewString()
	lastWrong := now.Add(-time.Hour)
	lastSignin := now.Add(-time.Minute)
	remember := now.Add(24 * time.Hour)

	u := &domain.User{
		ID:                  userID,
		OrganizationID:      orgID,
		Username:            "it-" + userID[:8],
		UserType:            "user",
		Status:              "active",
		IsVerified:          true,
		CreatedIp:           "127.0.0.1",
		RegisterType:        "password",
		RegisterSource:      "self_signup",
		Tag:                 "integration",
		Score:               7,
		CreatedAt:           now,
		UpdatedAt:           now,
		Revision:            1,
		SigninWrongTimes:    2,
		LastSigninWrongTime: &lastWrong,
		LastSigninAt:        &lastSignin,
		LastSigninIP:        "10.0.0.1",
		PreferredMfaType:    "totp",
		RecoveryCodeHashes:  []string{"code-1", "code-2"},
		TotpSecret:          "JBSWY3DPEHPK3PXP", // #nosec G101 -- deterministic test fixture, not a real credential
		MfaRadiusUsername:   "radius-user",
		MfaRadiusProvider:   "radius-provider",
		MfaPushEnabled:      true,
		MfaPushReceiver:     "push-receiver",
		MfaPushProvider:     "push-provider",
		MfaRememberDeadline: &remember,
		Balance:             12.5,
		BalanceCredit:       3.25,
		BalanceCurrency:     "USD",
		Custom1:             "c1",
		Custom2:             "c2",
		IpWhitelist:         "127.0.0.0/8",
		Profile: domain.UserProfile{
			UserID:        userID,
			DisplayName:   "Integration User",
			Email:         "it@example.com",
			EmailVerified: true,
			Properties:    map[string]any{"affiliation": "engineering"},
		},
		Credentials: domain.UserCredentials{
			UserID:       userID,
			PasswordHash: "$argon2id$integration", // #nosec G101 -- deterministic test fixture, not a real credential
			PasswordAlgo: "argon2id",
		},
	}

	repo := NewRepository(db)
	require.NoError(t, repo.Create(ctx, u))

	got, err := repo.GetByID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, userID, got.ID)
	require.Equal(t, "integration", got.Tag)
	require.Equal(t, 2, got.SigninWrongTimes)
	require.NotNil(t, got.LastSigninWrongTime)
	require.Equal(t, "10.0.0.1", got.LastSigninIP)
	require.Equal(t, "totp", got.PreferredMfaType)
	require.Equal(t, 2, len(got.RecoveryCodeHashes))
	require.Equal(t, "JBSWY3DPEHPK3PXP", got.TotpSecret)
	require.Equal(t, "radius-user", got.MfaRadiusUsername)
	require.Equal(t, "radius-provider", got.MfaRadiusProvider)
	require.True(t, got.MfaPushEnabled)
	require.NotNil(t, got.MfaRememberDeadline)
	require.Equal(t, 12.5, got.Balance)
	require.Equal(t, 3.25, got.BalanceCredit)
	require.Equal(t, "USD", got.BalanceCurrency)
	require.Equal(t, "c1", got.Custom1)
	require.Equal(t, "Integration User", got.Profile.DisplayName)
	require.Equal(t, "engineering", got.Profile.Affiliation)
	require.Equal(t, "$argon2id$integration", got.Credentials.PasswordHash)

	list, total, err := repo.List(ctx, domain.ListQuery{OrganizationID: orgID, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	require.Equal(t, userID, list[0].ID)
	require.Equal(t, 2, list[0].SigninWrongTimes)

	// 领域层语义：调用 Update 前 Revision++（旧 revision 用于 WHERE 守卫）。
	got.Balance = 20
	got.Custom2 = "c2-updated"
	got.Revision++
	require.NoError(t, repo.Update(ctx, got))

	after, err := repo.GetByID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, int64(2), after.Revision)
	require.Equal(t, 20.0, after.Balance)
	require.Equal(t, "c2-updated", after.Custom2)

	// 过期 revision 必须返回 conflict，且子表不得被改写。
	stale := *after
	stale.Balance = 99
	stale.Revision = 1
	err = repo.Update(ctx, &stale)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected app error, got %v", err)
	require.Equal(t, apperrors.CodeConflict, appErr.Code)
	still, err := repo.GetByID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, 20.0, still.Balance)

	require.NoError(t, Preflight(ctx, db))
	require.NoError(t, PreflightWrite(ctx, db, userID))

	require.NoError(t, repo.Delete(ctx, userID, orgID))
	_, err = repo.GetByID(ctx, userID)
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}
