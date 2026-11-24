package sessions

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/stretchr/testify/assert"
)

// newMockDB 构造 sql.DB + sqlmock，每个测试独立。
func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// sessionCols 是 GetByID/List 查询的完整列顺序（与 sql.go SELECT 严格一致）。
var sessionCols = []string{
	"id", "user_id", "username", "organization_id",
	"organization_slug", "application_id", "application_slug",
	"session_state", "sid", "device_info", "ip", "user_agent",
	"expires_at", "revoked_at", "created_at", "last_seen_at",
	"updated_at", "revision", "authentication_level",
	"assurance_level", "is_privileged", "lifecycle_id", "ticket_id",
}

// 固定时间戳，避免测试依赖 now()
var (
	fixedNow     = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedExpires = fixedNow.Add(7 * 24 * time.Hour)
)

// newTestSessionRow 构造一组 sqlmock 行数据，返回 []driver.Value 供 AddRow 展开。
// 必须严格匹配 sql.go SELECT 列顺序（全链路契约同步）。
func newTestSessionRow(id, userID, username, appSlug string) []driver.Value {
	deviceInfo, _ := json.Marshal(map[string]any{"browser": "Chrome"})
	return []driver.Value{
		// 1-3: id, user_id, username
		id, userID, username,
		// 4-5: organization_id, organization_slug (从 users.organization_id JOIN + organizations.slug)
		"org-1", "my-org",
		// 6-7: application_id, application_slug (从 sessions.application_id + applications.slug JOIN)
		"app-1", appSlug,
		// 8-9: session_state, sid
		"active", "sid-value",
		// 10-12: device_info, ip, user_agent
		deviceInfo, "127.0.0.1", "Mozilla/5.0",
		// 13-20: expires_at, revoked_at, created_at, last_seen_at, updated_at, revision, authentication_level, assurance_level
		fixedExpires, nil, fixedNow, fixedNow, fixedNow, int64(1), "password", "aal1",
		false, "", "",
	}
}

// ─── GetByID ──────────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT s.id")).
		WithArgs("sess-1").
		WillReturnRows(sqlmock.NewRows(sessionCols).AddRow(newTestSessionRow("sess-1", "user-1", "alice", "my-app")...))

	r := NewRepository(db)
	s, err := r.GetByID(context.Background(), "sess-1")

	assert.NoError(t, err)
	assert.Equal(t, "sess-1", s.ID)
	assert.Equal(t, "user-1", s.UserID)
	assert.Equal(t, "alice", s.Username)
	assert.Equal(t, "my-org", s.OrganizationSlug)
	assert.Equal(t, "my-app", s.ApplicationSlug)
	assert.Equal(t, domain.StateActive, s.SessionState)
	assert.Equal(t, "password", s.AuthenticationLevel)
	assert.Equal(t, domain.AssuranceLevelAAL1, s.AssuranceLevel)
	assert.Equal(t, "sid-value", s.Sid)
	assert.Equal(t, "127.0.0.1", s.IPAddress)
	assert.Equal(t, "Mozilla/5.0", s.UserAgent)
	assert.Equal(t, fixedExpires, s.ExpiresAt)
	assert.Equal(t, int64(1), s.Revision)
	assert.NotNil(t, s.DeviceInfo)
	assert.Equal(t, "Chrome", s.DeviceInfo["browser"])
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT s.id")).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows(sessionCols))

	r := NewRepository(db)
	_, err := r.GetByID(context.Background(), "missing")

	assert.Error(t, err)
	var nf *apperrors.Error
	assert.True(t, errors.As(err, &nf))
	assert.Equal(t, apperrors.CodeNotFound, nf.Code)
}

// ─── Create ──────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO")).
		WithArgs(
			"sess-1", "user-1", "app-1",
			domain.StateActive, "sid-xyz",
			sqlmock.AnyArg(), // device_info（JSON 编码，不严格匹配）
			"10.0.0.1", "Safari/18",
			fixedExpires, nil,
			fixedNow, fixedNow, fixedNow,
			int64(1),
			"password", // authentication_level 缺省
			"aal1",     // assurance_level 由认证等级归一
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	s := &domain.Session{
		ID:            "sess-1",
		UserID:        "user-1",
		ApplicationID: "app-1",
		SessionState:  domain.StateActive,
		Sid:           "sid-xyz",
		DeviceInfo:    map[string]any{"browser": "Safari"},
		IPAddress:     "10.0.0.1",
		UserAgent:     "Safari/18",
		ExpiresAt:     fixedExpires,
		CreatedAt:     fixedNow,
		LastSeenAt:    fixedNow,
		UpdatedAt:     fixedNow,
		Revision:      1,
	}

	err := r.Create(context.Background(), s)
	assert.NoError(t, err)
}

// ─── Revoke ──────────────────────────────────────────────────────────

func TestRepository_Revoke_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE")).
		WithArgs(fixedNow, fixedNow, "sess-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Revoke(context.Background(), "sess-1", fixedNow)
	assert.NoError(t, err)
}

func TestRepository_Revoke_AlreadyRevoked(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// UPDATE 影响 0 行 → GetByID 返回已撤销会话
	mock.ExpectExec(regexp.QuoteMeta("UPDATE")).
		WithArgs(fixedNow, fixedNow, "sess-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	revokedAt := fixedNow.Add(-time.Hour)
	row := newTestSessionRow("sess-1", "user-1", "alice", "my-app")
	row[13] = revokedAt // revoked_at 位置
	mock.ExpectQuery(regexp.QuoteMeta("SELECT s.id")).
		WithArgs("sess-1").
		WillReturnRows(sqlmock.NewRows(sessionCols).AddRow(row...))

	r := NewRepository(db)
	err := r.Revoke(context.Background(), "sess-1", fixedNow)
	assert.NoError(t, err) // 幂等：已撤销的会话 RevokedAt != nil → 返回 nil
}

// ─── RevokeByUserExcept ───────────────────────────────────────────────

func TestRepository_RevokeByUserExcept(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE")).
		WithArgs(fixedNow, fixedNow, "user-1", "sess-keep", fixedNow).
		WillReturnResult(sqlmock.NewResult(2, 2))

	r := NewRepository(db)
	n, err := r.RevokeByUserExcept(context.Background(), "user-1", "sess-keep", fixedNow)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), n)
}

// ─── RevokeByUser ────────────────────────────────────────────────────

func TestRepository_RevokeByUser(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE")).
		WithArgs(fixedNow, fixedNow, "user-1", fixedNow).
		WillReturnResult(sqlmock.NewResult(3, 3))

	r := NewRepository(db)
	n, err := r.RevokeByUser(context.Background(), "user-1", fixedNow)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)
}

// ─── Delete ─────────────────────────────────────────────────────────

func TestRepository_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM")).
		WithArgs("sess-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), "sess-1")
	assert.NoError(t, err)
}

func TestRepository_Delete_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM")).
		WithArgs("ghost").
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), "ghost")
	assert.Error(t, err)
	var nf *apperrors.Error
	assert.True(t, errors.As(err, &nf))
	assert.Equal(t, apperrors.CodeNotFound, nf.Code)
}

// ─── Update (乐观锁) ────────────────────────────────────────────────

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE")).
		WithArgs(
			domain.StateActive, fixedExpires.Add(time.Hour), fixedNow,
			false, "", "", "aal1", "sess-1", int64(5),
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	s := &domain.Session{
		ID:           "sess-1",
		SessionState: domain.StateActive,
		ExpiresAt:    fixedExpires.Add(time.Hour),
		UpdatedAt:    fixedNow,
		Revision:     5,
	}
	err := r.Update(context.Background(), s)
	assert.Error(t, err)
	var conflict *apperrors.Error
	assert.True(t, errors.As(err, &conflict))
	assert.Equal(t, apperrors.CodeConflict, conflict.Code)
}

// ─── Touch ──────────────────────────────────────────────────────────

func TestRepository_Touch_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE")).
		WithArgs(fixedNow, fixedNow, "sess-1", fixedNow).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Touch(context.Background(), "sess-1", fixedNow)
	assert.NoError(t, err)
}

// ─── CleanupExpired ─────────────────────────────────────────────────

func TestRepository_CleanupExpired(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM")).
		WithArgs(fixedNow).
		WillReturnResult(sqlmock.NewResult(5, 5))

	r := NewRepository(db)
	n, err := r.CleanupExpired(context.Background(), fixedNow)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), n)
}

// ─── RevokeByUserAndApp ─────────────────────────────────────────────

func TestRepository_RevokeByUserAndApp(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE")).
		WithArgs(fixedNow, fixedNow, "user-1", "app-1", "keep-sess", fixedNow).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	n, err := r.RevokeByUserAndApp(context.Background(), "user-1", "app-1", "keep-sess", fixedNow)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

// ─── RevokeByOrganization ──────────────────────────────────────────

func TestRepository_RevokeByOrganization(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE")).
		WithArgs(fixedNow, fixedNow, "org-1", fixedNow).
		WillReturnResult(sqlmock.NewResult(10, 10))

	r := NewRepository(db)
	n, err := r.RevokeByOrganization(context.Background(), "org-1", fixedNow)
	assert.NoError(t, err)
	assert.Equal(t, int64(10), n)
}

// ─── List 基本查询 ───────────────────────────────────────────────────

func TestRepository_List_Basic(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// List 行
	mock.ExpectQuery(regexp.QuoteMeta("SELECT s.id")).
		WithArgs(
			"org-1", "org-1",
			"", "",
			"", "",
			"", "",
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			"", "", "", "", "",
			20, 0,
		).
		WillReturnRows(sqlmock.NewRows(sessionCols).
			AddRow(newTestSessionRow("sess-1", "user-1", "alice", "my-app")...).
			AddRow(newTestSessionRow("sess-2", "user-2", "bob", "other-app")...))

	// Count 行
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs(
			"org-1", "org-1", "", "", "", "", "", "",
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			"", "", "", "", "",
		).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	r := NewRepository(db)
	q := domain.ListQuery{
		OrganizationID: "org-1",
		PageSize:       20,
		Page:           1,
	}
	items, total, err := r.List(context.Background(), q)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 2)
	assert.Equal(t, "sess-1", items[0].ID)
	assert.Equal(t, "sess-2", items[1].ID)
}

func TestRepository_List_HostilePaginationKeepsOffsetNonNegative(t *testing.T) {
	cases := []struct {
		name           string
		page, pageSize int
		wantLimit      int
		wantOffset     int
	}{
		{
			name: "hostile page is capped",
			page: math.MaxInt, pageSize: 20,
			wantLimit: 20, wantOffset: (platformpagination.MaxPage - 1) * 20,
		},
		{
			name: "hostile page size is capped",
			page: 2, pageSize: math.MaxInt,
			wantLimit: 500, wantOffset: 500,
		},
		{
			name: "normal page",
			page: 3, pageSize: 20,
			wantLimit: 20, wantOffset: 40,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newMockDB(t)
			defer db.Close()

			mock.ExpectQuery(regexp.QuoteMeta("SELECT s.id")).
				WithArgs(
					"org-1", "org-1",
					"", "",
					"", "",
					"", "",
					sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
					"", "", "", "", "",
					tc.wantLimit, tc.wantOffset,
				).
				WillReturnRows(sqlmock.NewRows(sessionCols))

			mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
				WithArgs(
					"org-1", "org-1", "", "", "", "", "", "",
					sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
					"", "", "", "", "",
				).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			items, total, err := NewRepository(db).List(context.Background(), domain.ListQuery{
				OrganizationID: "org-1", Page: tc.page, PageSize: tc.pageSize,
			})
			assert.NoError(t, err)
			assert.Empty(t, items)
			assert.Equal(t, int64(0), total)
			assert.GreaterOrEqual(t, tc.wantOffset, 0)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── 编译期 domain.Repository 接口断言 ──────────────────────────────────

var _ domain.Repository = Repository{}
