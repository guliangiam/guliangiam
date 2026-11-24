package sessions

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	return r.queryByID(ctx, queryGetSessionByID, id)
}

// GetByIDForOrganization scopes the lookup to the tenant that owns the session.
// sessions has no organization_id column, so the SQL joins users (the same
// association the RLS predicate uses).
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Session, error) {
	return r.queryByID(ctx, queryGetSessionByIDForOrganization, id, organizationID)
}

func (r Repository) queryByID(ctx context.Context, query string, args ...any) (*domain.Session, error) {
	var row sessionRow
	err := r.db.QueryRow(ctx, query, args...).Scan(&row.ID, &row.UserID, &row.Username, &row.OrganizationID, &row.OrganizationSlug, &row.ApplicationID, &row.ApplicationSlug, &row.SessionState, &row.Sid, &row.DeviceInfo, &row.IPAddress, &row.UserAgent, &row.ExpiresAt, &row.RevokedAt, &row.CreatedAt, &row.LastSeenAt, &row.UpdatedAt, &row.Revision, &row.AuthenticationLevel, &row.AssuranceLevel, &row.IsPrivileged, &row.LifecycleID, &row.TicketID)
	if err != nil {
		if err == pgx.ErrNoRows {
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

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Session, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListSessions, query.OrganizationID, query.UserID, query.ApplicationID, query.State, query.ActiveOnly, query.Keyword, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Perf: pre-allocate with pageSize capacity to avoid slice growth reallocations during row scanning.
	items := make([]*domain.Session, 0, shared.ClampPageSize(query.PageSize))
	for rows.Next() {
		var row sessionRow
		if err := rows.Scan(&row.ID, &row.UserID, &row.Username, &row.OrganizationID, &row.OrganizationSlug, &row.ApplicationID, &row.ApplicationSlug, &row.SessionState, &row.Sid, &row.DeviceInfo, &row.IPAddress, &row.UserAgent, &row.ExpiresAt, &row.RevokedAt, &row.CreatedAt, &row.LastSeenAt, &row.UpdatedAt, &row.Revision, &row.AuthenticationLevel, &row.AssuranceLevel, &row.IsPrivileged, &row.LifecycleID, &row.TicketID); err != nil {
			return nil, 0, err
		}

		session, err := toSession(row)
		if err != nil {
			return nil, 0, apperrors.NewInternal("decode session")
		}
		items = append(items, session)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountSessions, query.OrganizationID, query.UserID, query.ApplicationID, query.State, query.ActiveOnly, query.Keyword).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, session *domain.Session) error {
	deviceInfo, err := shared.MarshalJSON(session.DeviceInfo)
	if err != nil {
		return err
	}

	// authentication_level 缺省为 "password" —— 调用方必须在 session 创建前设置好。
	authLevel := session.AuthenticationLevel
	if authLevel == "" {
		authLevel = "password"
	}
	assuranceLevel := normalizeAssuranceLevel(session.AssuranceLevel, authLevel)

	_, err = r.db.Exec(ctx, queryInsertSession,
		session.ID, session.UserID, session.ApplicationID, session.SessionState, session.Sid,
		deviceInfo, session.IPAddress, session.UserAgent, session.ExpiresAt, session.RevokedAt,
		session.CreatedAt, session.LastSeenAt, session.UpdatedAt, session.Revision,
		authLevel, assuranceLevel,
	)
	return err
}

func (r Repository) Touch(ctx context.Context, id string, touchedAt time.Time) error {
	result, err := r.db.Exec(ctx, queryTouchSession, id, touchedAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		_, getErr := r.GetByID(ctx, id)
		if getErr != nil {
			return getErr
		}
	}

	return nil
}

func (r Repository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	result, err := r.db.Exec(ctx, queryRevokeSession, id, revokedAt)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		session, getErr := r.GetByID(ctx, id)
		if getErr != nil {
			return getErr
		}
		if session.RevokedAt != nil {
			return nil
		}
	}

	return nil
}

// RevokeForOrganization revokes a session only inside the trusted tenant. A
// missing or foreign row surfaces as not found; an already revoked row stays
// idempotent.
func (r Repository) RevokeForOrganization(ctx context.Context, id, organizationID string, revokedAt time.Time) error {
	result, err := r.db.Exec(ctx, queryRevokeSessionForOrganization, id, revokedAt, organizationID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		session, getErr := r.GetByIDForOrganization(ctx, id, organizationID)
		if getErr != nil {
			return getErr
		}
		if session.RevokedAt != nil {
			return nil
		}
	}

	return nil
}

// revokeSessionsQuery is the single place that decides whether a revoke keeps
// one session alive. Callers hand it both statement variants; an empty
// excludeSessionID selects the variant without the uuid predicate and adds no
// bind argument, so "" can never be bound to a uuid column. A non-empty id is
// appended last, keeping placeholder numbering stable for both variants.
func revokeSessionsQuery(excludeSessionID, withExclude, withoutExclude string, args []any) (string, []any) {
	if excludeSessionID == "" {
		return withoutExclude, args
	}
	return withExclude, append(args, excludeSessionID)
}

// RevokeByUserExcept revokes every active session of a user while keeping
// excludeSessionID (when provided) alive. An empty excludeSessionID means
// "no exclusion".
func (r Repository) RevokeByUserExcept(ctx context.Context, userID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	return r.execRevoke(ctx, queryRevokeUserSessionsExcept, queryRevokeUserSessions, excludeSessionID, []any{userID, revokedAt})
}

func (r Repository) RevokeByUserAndApp(ctx context.Context, userID, applicationID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	return r.execRevoke(ctx, queryRevokeUserAppSessionsExcept, queryRevokeUserAppSessions, excludeSessionID, []any{userID, applicationID, revokedAt})
}

// RevokeByUser revokes all active sessions of a user (force logout).
func (r Repository) RevokeByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	return r.execRevoke(ctx, queryRevokeUserSessions, queryRevokeUserSessions, "", []any{userID, revokedAt})
}

func (r Repository) execRevoke(ctx context.Context, withExclude, withoutExclude string, excludeSessionID string, args []any) (int64, error) {
	query, queryArgs := revokeSessionsQuery(excludeSessionID, withExclude, withoutExclude, args)
	result, err := r.db.Exec(ctx, query, queryArgs...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}

// RevokeByOrganization revokes all active sessions in an organization.
func (r Repository) RevokeByOrganization(ctx context.Context, organizationID string, revokedAt time.Time) (int64, error) {
	result, err := r.db.Exec(ctx, queryRevokeAllOrgSessions, organizationID, revokedAt)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r Repository) Delete(ctx context.Context, id string) error {
	result, err := r.db.Exec(ctx, queryDeleteSession, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.NewNotFound("session not found")
	}

	return nil
}

// DeleteForOrganization deletes a session only inside the trusted tenant; a
// foreign or missing row surfaces as not found.
func (r Repository) DeleteForOrganization(ctx context.Context, id, organizationID string) error {
	result, err := r.db.Exec(ctx, queryDeleteSessionForOrganization, id, organizationID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.NewNotFound("session not found")
	}

	return nil
}

func (r Repository) Update(ctx context.Context, session *domain.Session) error {
	result, err := r.db.Exec(ctx, queryUpdateSession, session.ID, session.SessionState, session.ExpiresAt, session.UpdatedAt, session.Revision, session.IsPrivileged, session.LifecycleID, session.TicketID, normalizeAssuranceLevel(session.AssuranceLevel, session.AuthenticationLevel))
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewConflict("session revision conflict or already revoked")
	}
	return nil
}

var _ domain.Repository = Repository{}

// CleanupExpired permanently deletes sessions that expired before the given
// cutoff time. Returns the number of deleted sessions.
func (r Repository) CleanupExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.db.Exec(ctx,
		`DELETE FROM guliangiam.sessions WHERE expires_at < $1`,
		cutoff,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
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

// derefString flattens the nullable sid column. A session created before the
// column existed (or by a path that has not been given a sid yet) reads back as
// NULL, and the domain treats "no sid" as an empty string so the ID token simply
// omits the claim rather than carrying a literal "<nil>".
func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// maxInt returns value when positive, otherwise fallback. Kept package-local
// (and renamed from max) so it does not shadow the predeclared max builtin.
func maxInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
