package sessions

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"

	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DBTX 复用 shared.DBTX（抽象 *sql.DB 和 *sql.Tx）。

// Repository 是 MySQL 存储层的 sessions 仓库。
// 与 internal/infra/postgres/sessions.Repository 实现同域接口，
// 由 mysqluow 在 runtime.go 中按 driver 选中。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

var _ domain.Repository = Repository{}

// rowsAffected 从 sql.Result 中安全提取影响行数（database/sql 版返回 (int64, error)）。
func rowsAffected(r sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// ── Read ────────────────────────────────────────────────────────────────

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	return r.queryByID(ctx, queryGetSessionByID, id)
}

// GetByIDForOrganization scopes the lookup to the tenant that owns the session.
// sessions has no organization_id column, so the SQL resolves it through users.
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Session, error) {
	return r.queryByID(ctx, queryGetSessionByIDForOrganization, id, organizationID)
}

func (r Repository) queryByID(ctx context.Context, query string, args ...any) (*domain.Session, error) {
	var row sessionRow
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&row.ID, &row.UserID, &row.Username, &row.OrganizationID,
		&row.OrganizationSlug, &row.ApplicationID, &row.ApplicationSlug,
		&row.SessionState, &row.Sid, &row.DeviceInfo, &row.IPAddress,
		&row.UserAgent, &row.ExpiresAt, &row.RevokedAt, &row.CreatedAt,
		&row.LastSeenAt, &row.UpdatedAt, &row.Revision, &row.AuthenticationLevel,
		&row.AssuranceLevel, &row.IsPrivileged, &row.LifecycleID, &row.TicketID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("session not found")
		}
		return nil, err
	}

	session, err := toSession(row)
	if err != nil {
		return nil, apperrors.NewInternal("decode session")
	}
	return session, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Session, int64, error) {
	pageSize := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	activeArg := nullableBool(q.ActiveOnly)

	rows, err := r.db.QueryContext(ctx, queryListSessions,
		q.OrganizationID, q.OrganizationID, // (? = '' OR col = ?) 两处占位
		q.UserID, q.UserID,
		q.ApplicationID, q.ApplicationID,
		q.State, q.State,
		activeArg, activeArg, activeArg, // (? IS NULL OR (? = 1/0 AND ... 三处
		q.Keyword, q.Keyword, q.Keyword, q.Keyword, q.Keyword, // LIKE CONCAT 五处
		pageSize, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Session, 0)
	for rows.Next() {
		var row sessionRow
		if err := rows.Scan(
			&row.ID, &row.UserID, &row.Username, &row.OrganizationID,
			&row.OrganizationSlug, &row.ApplicationID, &row.ApplicationSlug,
			&row.SessionState, &row.Sid, &row.DeviceInfo, &row.IPAddress,
			&row.UserAgent, &row.ExpiresAt, &row.RevokedAt, &row.CreatedAt,
			&row.LastSeenAt, &row.UpdatedAt, &row.Revision, &row.AuthenticationLevel,
			&row.AssuranceLevel, &row.IsPrivileged, &row.LifecycleID, &row.TicketID,
		); err != nil {
			return nil, 0, err
		}
		s, err := toSession(row)
		if err != nil {
			return nil, 0, apperrors.NewInternal("decode session")
		}
		items = append(items, s)
	}
	if rows.Err() != nil {
		return nil, 0, rows.Err()
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountSessions,
		q.OrganizationID, q.OrganizationID,
		q.UserID, q.UserID,
		q.ApplicationID, q.ApplicationID,
		q.State, q.State,
		activeArg, activeArg, activeArg,
		q.Keyword, q.Keyword, q.Keyword, q.Keyword, q.Keyword,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// nullableBool 把 *bool 转成 sql.NullBool，MySQL driver 能正确绑定。
func nullableBool(b *bool) sql.NullBool {
	if b == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *b, Valid: true}
}

// ── Write ───────────────────────────────────────────────────────────────

func (r Repository) Create(ctx context.Context, session *domain.Session) error {
	deviceInfo, err := json.Marshal(session.DeviceInfo)
	if err != nil {
		return err
	}
	authLevel := session.AuthenticationLevel
	if authLevel == "" {
		authLevel = "password"
	}
	assuranceLevel := normalizeAssuranceLevel(session.AssuranceLevel, authLevel)
	_, err = r.db.ExecContext(ctx, queryInsertSession,
		session.ID, session.UserID, session.ApplicationID,
		session.SessionState, session.Sid, deviceInfo,
		session.IPAddress, session.UserAgent,
		session.ExpiresAt, session.RevokedAt,
		session.CreatedAt, session.LastSeenAt, session.UpdatedAt,
		session.Revision,
		authLevel, assuranceLevel,
	)
	return err
}

func (r Repository) Touch(ctx context.Context, id string, touchedAt time.Time) error {
	affected, err := rowsAffected(
		r.db.ExecContext(ctx, queryTouchSession, touchedAt, touchedAt, id, touchedAt),
	)
	if err != nil {
		return err
	}
	if affected == 0 {
		_, getErr := r.GetByID(ctx, id)
		if getErr != nil {
			return getErr
		}
	}
	return nil
}

func (r Repository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	affected, err := rowsAffected(
		r.db.ExecContext(ctx, queryRevokeSession, revokedAt, revokedAt, id),
	)
	if err != nil {
		return err
	}
	if affected == 0 {
		s, getErr := r.GetByID(ctx, id)
		if getErr != nil {
			return getErr
		}
		if s.RevokedAt != nil {
			return nil // 已被撤销，幂等
		}
	}
	return nil
}

// RevokeForOrganization revokes a session only inside the trusted tenant. A
// missing or foreign row surfaces as not found; an already revoked row stays
// idempotent.
func (r Repository) RevokeForOrganization(ctx context.Context, id, organizationID string, revokedAt time.Time) error {
	affected, err := rowsAffected(
		r.db.ExecContext(ctx, queryRevokeSessionForOrganization, revokedAt, revokedAt, id, organizationID),
	)
	if err != nil {
		return err
	}
	if affected == 0 {
		s, getErr := r.GetByIDForOrganization(ctx, id, organizationID)
		if getErr != nil {
			return getErr
		}
		if s.RevokedAt != nil {
			return nil
		}
	}
	return nil
}

func (r Repository) RevokeByUserExcept(ctx context.Context, userID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	return rowsAffected(
		r.db.ExecContext(ctx, queryRevokeUserOtherSessions, revokedAt, revokedAt, userID, excludeSessionID, revokedAt),
	)
}

func (r Repository) RevokeByUserAndApp(ctx context.Context, userID, applicationID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	return rowsAffected(
		r.db.ExecContext(ctx, queryRevokeUserAppOtherSessions, revokedAt, revokedAt, userID, applicationID, excludeSessionID, revokedAt),
	)
}

func (r Repository) RevokeByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	return rowsAffected(
		r.db.ExecContext(ctx, queryRevokeAllUserSessions, revokedAt, revokedAt, userID, revokedAt),
	)
}

// RevokeByOrganization 撤销某组织下所有活跃会话。
// MySQL sessions 表没有 organization_id 列（PG 版也没有，用 JOIN users），
// 所以用子查询 `user_id IN (SELECT id FROM users WHERE organization_id = ?)`。
func (r Repository) RevokeByOrganization(ctx context.Context, organizationID string, revokedAt time.Time) (int64, error) {
	return rowsAffected(
		r.db.ExecContext(ctx, queryRevokeAllOrgSessions, revokedAt, revokedAt, organizationID, revokedAt),
	)
}

func (r Repository) Delete(ctx context.Context, id string) error {
	affected, err := rowsAffected(r.db.ExecContext(ctx, queryDeleteSession, id))
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("session not found")
	}
	return nil
}

// DeleteForOrganization deletes a session only inside the trusted tenant; a
// foreign or missing row surfaces as not found.
func (r Repository) DeleteForOrganization(ctx context.Context, id, organizationID string) error {
	affected, err := rowsAffected(r.db.ExecContext(ctx, queryDeleteSessionForOrganization, id, organizationID))
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("session not found")
	}
	return nil
}

func (r Repository) Update(ctx context.Context, session *domain.Session) error {
	affected, err := rowsAffected(r.db.ExecContext(ctx, queryUpdateSession,
		session.SessionState, session.ExpiresAt, session.UpdatedAt,
		session.IsPrivileged, session.LifecycleID, session.TicketID,
		normalizeAssuranceLevel(session.AssuranceLevel, session.AuthenticationLevel),
		session.ID, session.Revision,
	))
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("session revision conflict or already revoked")
	}
	return nil
}

// normalizeAssuranceLevel resolves the value persisted to assurance_level.
// An explicit aal1/aal2 is kept; an unset value is derived from the
// authentication level, and an empty authentication level falls back to aal1.
func normalizeAssuranceLevel(assuranceLevel, authenticationLevel string) string {
	if assuranceLevel != "" {
		return assuranceLevel
	}
	return domain.AssuranceLevelForAuthenticationLevel(authenticationLevel)
}

// CleanupExpired 永久删除过期会话，返回删除行数。
func (r Repository) CleanupExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	return rowsAffected(r.db.ExecContext(ctx, queryCleanupExpired, cutoff))
}
