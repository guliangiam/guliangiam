package users

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

var fixedNow = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)

// anyArgs 返回 n 个 sqlmock.AnyArg()，用于构造 WithArgs 参数占位
func anyArgs(n int) []driver.Value {
	out := make([]driver.Value, n)
	for i := range out {
		out[i] = sqlmock.AnyArg()
	}
	return out
}

// userCols 是用户聚合 SELECT 的完整 56 列顺序（users 主表 + 5 张 user_* 子表）
var userCols = []string{
	"id", "organization_id", "username", "external_id",
	"user_type", "status", "is_admin", "is_forbidden", "is_verified", "is_online",
	"created_ip", "signup_application_id", "register_type", "register_source",
	"tag", "score",
	"signin_wrong_times", "last_signin_wrong_time",
	"preferred_mfa_type", "recovery_code_hashes", "totp_secret",
	"mfa_radius_username", "mfa_radius_provider",
	"mfa_push_receiver", "mfa_push_provider", "mfa_push_enabled",
	"mfa_remember_deadline", "last_signin_at", "last_signin_ip",
	"balance", "balance_credit", "balance_currency",
	"is_deleted", "ip_whitelist",
	"addresses", "managed_accounts", "face_ids", "cart",
	"mfa_items", "mfa_accounts", "application_scopes",
	"custom1", "custom2", "custom3", "custom4", "custom5",
	"custom6", "custom7", "custom8", "custom9", "custom10",
	"created_at", "updated_at", "deleted_at", "revision",
}

func newTestUserRow(id, orgID, username string) []driver.Value {
	emptyJSON, _ := json.Marshal([]any{})
	return []driver.Value{
		id, orgID, username, "",
		"user", "active", false, false, false, false,
		"127.0.0.1", "", "email", "", "", 0,
		0, nil,
		"", emptyJSON, "",
		"corp-radius", "radius-mfa",
		"", "", false,
		nil, nil, "",
		0.0, 0.0, "USD",
		false, "",
		emptyJSON, emptyJSON, emptyJSON, emptyJSON, emptyJSON, emptyJSON, emptyJSON,
		"", "", "", "", "", "", "", "", "", "",
		fixedNow, fixedNow, nil, int64(1),
	}
}

var profileCols = []string{
	"user_id", "display_name", "first_name", "last_name",
	"avatar_url", "email", "email_verified",
	"phone", "phone_verified",
	"country_code", "region", "location",
	"birthday", "gender", "bio",
	"real_name", "id_card_type", "id_card",
	"passport_no", "driver_license_no", "id_card_expiry",
	"issue_country", "issuing_authority",
	"passport_photo_url", "driver_license_front_url", "driver_license_back_url",
	"properties",
}

func newTestProfileRow(userID string) []driver.Value {
	props, _ := json.Marshal(map[string]any{"affiliation": "engineering"})
	return []driver.Value{
		userID, "Alice", "Alice", "Chen",
		"", "alice@example.com", true,
		"", false,
		"", "", "",
		"", "female", "",
		"", "", "",
		"E12345678", "D1234567", "2030-12-31",
		"CN", "MPS Beijing",
		"", "", "",
		props,
	}
}

var credentialsCols = []string{
	"user_id", "password_hash", "password_algo", "password_updated_at",
	"access_key", "access_secret_hash", "access_secret_rotated_at", "need_update_password",
	"access_secret_hash_prev", "access_secret_prev_expires_at",
}

func newTestCredentialsRow(userID string) []driver.Value {
	return []driver.Value{
		userID, "$argon2id$...", "argon2id", nil,
		"", "", nil, false,
		"", nil,
	}
}

var identityCols = []string{
	"id", "user_id", "provider_id", "identity_type",
	"subject", "provider_account", "profile_snapshot", "linked_at",
}

var mfaCols = []string{
	"id", "user_id", "method_type", "enabled", "verified",
	"display_name", "last_used_at",
}

// ─── GetByID ──────────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.organization_id, u.username")).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(userCols).AddRow(newTestUserRow("user-1", "org-1", "alice")...))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id, COALESCE(display_name, '')")).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(profileCols).AddRow(newTestProfileRow("user-1")...))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id, password_hash")).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(credentialsCols).AddRow(newTestCredentialsRow("user-1")...))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, provider_id")).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(identityCols))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, method_type")).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(mfaCols))

	r := NewRepository(db)
	u, err := r.GetByID(context.Background(), "user-1")

	assert.NoError(t, err)
	assert.Equal(t, "user-1", u.ID)
	assert.Equal(t, "alice", u.Username)
	assert.Equal(t, "org-1", u.OrganizationID)
	assert.Equal(t, "Alice", u.Profile.DisplayName)
	assert.Equal(t, "engineering", u.Profile.Affiliation)
	assert.Equal(t, "E12345678", u.Profile.PassportNo)
	assert.Equal(t, "D1234567", u.Profile.DriverLicenseNo)
	assert.Equal(t, "2030-12-31", u.Profile.IDCardExpiry)
	assert.Equal(t, "CN", u.Profile.IssueCountry)
	assert.Equal(t, "MPS Beijing", u.Profile.IssuingAuthority)
	assert.Equal(t, "$argon2id$...", u.Credentials.PasswordHash)
	assert.Equal(t, int64(1), u.Revision)
	assert.Equal(t, 0, u.SigninWrongTimes)
	assert.Equal(t, "corp-radius", u.MfaRadiusUsername)
	assert.Equal(t, "radius-mfa", u.MfaRadiusProvider)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.organization_id, u.username")).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows(userCols))

	r := NewRepository(db)
	_, err := r.GetByID(context.Background(), "missing")

	assert.Error(t, err)
	var nf *apperrors.Error
	assert.True(t, errors.As(err, &nf))
	assert.Equal(t, apperrors.CodeNotFound, nf.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── GetByUsername ─────────────────────────────────────────────────────

func TestRepository_GetByUsername_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("WHERE (? = '' OR u.organization_id = ?) AND u.username = ?")).
		WithArgs("org-1", "org-1", "alice").
		WillReturnRows(sqlmock.NewRows(userCols).AddRow(newTestUserRow("user-1", "org-1", "alice")...))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id, COALESCE(display_name, '')")).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(profileCols).AddRow(newTestProfileRow("user-1")...))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id, password_hash")).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(credentialsCols).AddRow(newTestCredentialsRow("user-1")...))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, provider_id")).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(identityCols))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id, method_type")).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(mfaCols))

	r := NewRepository(db)
	u, err := r.GetByUsername(context.Background(), "org-1", "alice")
	assert.NoError(t, err)
	assert.Equal(t, "user-1", u.ID)
	assert.Equal(t, "alice", u.Username)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── List ────────────────────────────────────────────────────────────

// List 的参数顺序曾与 SQL 占位符数量不一致（keyword 多绑一个参数），
// 该用例锁定 13 个列表参数与 11 个计数参数。
func TestRepository_List_ParameterOrder(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.organization_id, u.username")).
		WithArgs("org-1", "org-1", "", "", "", "", "", "", "", nil, nil, 10, 0).
		WillReturnRows(sqlmock.NewRows(userCols))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs("org-1", "org-1", "", "", "", "", "", "", "", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		Page: 1, PageSize: 10, OrganizationID: "org-1",
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, items)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRepository_List_OffsetOverridesPage 锁定 SCIM startIndex 语义：显式
// Offset > 0 时覆盖页推导偏移（C07，与 postgres 镜像同构）。
func TestRepository_List_OffsetOverridesPage(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.organization_id, u.username")).
		WithArgs("org-1", "org-1", "", "", "", "", "", "", "", nil, nil, 10, 42).
		WillReturnRows(sqlmock.NewRows(userCols))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs("org-1", "org-1", "", "", "", "", "", "", "", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	r := NewRepository(db)
	_, _, err := r.List(context.Background(), domain.ListQuery{
		Page: 3, PageSize: 10, Offset: 42, OrganizationID: "org-1",
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRepository_List_LeanSkipsAggregateLoads 锁定 lean 投影：只加载
// profile，跳过凭据/身份/MFA 批量查询（C11①，与 postgres 镜像同构）。
// 任何被跳过的查询若仍被发起，sqlmock 会以 unexpected query 失败。
func TestRepository_List_LeanSkipsAggregateLoads(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT u.id, u.organization_id, u.username")).
		WithArgs("org-1", "org-1", "", "", "", "", "", "", "", nil, nil, 10, 0).
		WillReturnRows(sqlmock.NewRows(userCols).AddRow(newTestUserRow("user-1", "org-1", "alice")...))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id, COALESCE(display_name, '')")).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(profileCols).AddRow(newTestProfileRow("user-1")...))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs("org-1", "org-1", "", "", "", "", "", "", "", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		Page: 1, PageSize: 10, OrganizationID: "org-1", Lean: true,
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── Create ──────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `users`")).
		WithArgs(
			"user-1", "org-1", "alice", nil, "user", "active",
			false, false, false, false,
			nil, nil, nil, nil, "", 0, false, "",
			fixedNow, fixedNow, nil, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_signin_tracking`")).
		WithArgs(anyArgs(5)...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_mfa_config`")).
		WithArgs(anyArgs(10)...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_financials`")).
		WithArgs(anyArgs(4)...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_collections`")).
		WithArgs(anyArgs(8)...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_custom_attributes`")).
		WithArgs(anyArgs(11)...).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_profiles`")).
		WithArgs(
			"user-1", "Alice", "Alice", "Chen", nil,
			"alice@example.com", true, nil, false,
			nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil,
			[]byte("{}"),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_credentials`")).
		WithArgs(anyArgs(10)...).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	r := NewRepository(db)
	u := &domain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Status:         "active",
		UserType:       "user",
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       1,
		Profile: domain.UserProfile{
			UserID:        "user-1",
			DisplayName:   "Alice",
			FirstName:     "Alice",
			LastName:      "Chen",
			Email:         "alice@example.com",
			EmailVerified: true,
			Properties:    map[string]any{},
		},
		Credentials: domain.UserCredentials{
			UserID:       "user-1",
			PasswordHash: "hash",
			PasswordAlgo: "argon2id",
		},
	}

	err := r.Create(context.Background(), u)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRepository_Create_ProfileFailureRollsBack pins the MySQL half of the SCIM
// POST failure path: the users/child rows are inserted first, and a failing
// user_profiles insert must roll back the whole transaction.
func TestRepository_Create_ProfileFailureRollsBack(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `users`")).
		WithArgs(anyArgs(22)...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	for _, table := range []string{"user_signin_tracking", "user_mfa_config", "user_financials", "user_collections", "user_custom_attributes"} {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `" + table + "`")).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_profiles`")).
		WillReturnError(errors.New("user_profiles insert failed"))
	mock.ExpectRollback()

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Status:         "active",
		UserType:       "user",
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       1,
		Profile:        domain.UserProfile{UserID: "user-1", DisplayName: "Alice"},
	})
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── Update ──────────────────────────────────────────────────────────

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `users`")).
		WithArgs(anyArgs(19)...).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	r := NewRepository(db)
	u := &domain.User{ID: "user-1", Revision: 3, UpdatedAt: fixedNow}
	err := r.Update(context.Background(), u)

	assert.Error(t, err)
	var conflict *apperrors.Error
	assert.True(t, errors.As(err, &conflict), "expected conflict error, got %v", err)
	assert.Equal(t, apperrors.CodeConflict, conflict.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `users`")).
		WithArgs(anyArgs(19)...).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_signin_tracking`")).
		WithArgs(anyArgs(5)...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_mfa_config`")).
		WithArgs(anyArgs(10)...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_financials`")).
		WithArgs(anyArgs(4)...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_collections`")).
		WithArgs(anyArgs(8)...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_custom_attributes`")).
		WithArgs(anyArgs(11)...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	r := NewRepository(db)
	u := &domain.User{ID: "user-1", Revision: 3, UpdatedAt: fixedNow}
	err := r.Update(context.Background(), u)

	assert.NoError(t, err)
	assert.Equal(t, int64(3), u.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── Delete ──────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE `users` SET deleted_at")).
		WithArgs("user-1", "org-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), "user-1", "org-1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── Count ──────────────────────────────────────────────────────────

func TestRepository_Count_WithOrg(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `users` WHERE organization_id")).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(42)))

	r := NewRepository(db)
	n, err := r.Count(context.Background(), "org-1")
	assert.NoError(t, err)
	assert.Equal(t, int64(42), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Count_NoOrg(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `users` WHERE deleted_at IS NULL")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(100)))

	r := NewRepository(db)
	n, err := r.Count(context.Background(), "")
	assert.NoError(t, err)
	assert.Equal(t, int64(100), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── HasDeletedWithUsername ──────────────────────────────────────────

func TestRepository_HasDeletedWithUsername_True(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS")).
		WithArgs("org-1", "alice").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	r := NewRepository(db)
	exists, err := r.HasDeletedWithUsername(context.Background(), "org-1", "alice")
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── UpdateProfile ───────────────────────────────────────────────────

func TestRepository_UpdateProfile(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_profiles`")).
		WithArgs(
			"user-1", "Alice Renamed", nil, nil, nil,
			nil, true, nil, false,
			nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil,
			[]byte("{}"),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.UpdateProfile(context.Background(), &domain.UserProfile{
		UserID:        "user-1",
		DisplayName:   "Alice Renamed",
		EmailVerified: true,
		Properties:    map[string]any{},
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── ReplaceIdentities ────────────────────────────────────────────────

func TestRepository_ReplaceIdentities(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `user_identities`")).
		WithArgs("user-1").
		WillReturnResult(sqlmock.NewResult(2, 2))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_identities`")).
		WithArgs(anyArgs(8)...).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	r := NewRepository(db)
	idents := []domain.UserIdentity{
		{
			ID:              "id-1",
			UserID:          "user-1",
			ProviderID:      "github",
			IdentityType:    "oauth",
			Subject:         "github|123",
			ProfileSnapshot: map[string]any{},
			LinkedAt:        fixedNow,
		},
	}
	err := r.ReplaceIdentities(context.Background(), "user-1", idents)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── ListRoleBindingsByUserIDs ────────────────────────────────────────

func TestRepository_ListRoleBindingsByUserIDs(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id, role_id FROM")).
		WithArgs("user-1", "user-2").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "role_id"}).
			AddRow("user-1", "role-admin").
			AddRow("user-1", "role-user").
			AddRow("user-2", "role-user"),
		)

	r := NewRepository(db)
	result, err := r.ListRoleBindingsByUserIDs(context.Background(), []string{"user-1", "user-2"})
	assert.NoError(t, err)
	assert.Len(t, result["user-1"], 2)
	assert.Len(t, result["user-2"], 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── 编译期 domain.Repository 接口断言 ──────────────────────────────────

var _ domain.Repository = Repository{}
