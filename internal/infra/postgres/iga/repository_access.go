package iga

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type rowScanner interface{ Scan(dest ...any) error }

func scanLifecycle(row rowScanner) (*domain.AccessLifecycle, error) {
	var l domain.AccessLifecycle
	var status string
	err := row.Scan(&l.ID, &l.OrganizationID, &l.UserID, &l.RoleID, &l.TicketID,
		&l.EnforcerID, &l.Resource, &l.Action, &l.PolicyID, &l.GrantedAt,
		&l.StartsAt, &l.ExpiresAt, &l.RevokedAt, &status, &l.RetryCount,
		&l.LastError, &l.CreatedAt, &l.Revision)
	if err != nil {
		return nil, err
	}
	l.Status = domain.LifecycleStatus(status)
	return &l, nil
}

func scanAccessRequest(row rowScanner) (*domain.AccessRequest, error) {
	var r domain.AccessRequest
	var status string
	err := row.Scan(&r.ID, &r.OrganizationID, &r.RequesterID, &r.RoleID,
		&r.ApplicationID, &r.EnforcerID, &r.Resource, &r.Action, &r.Reason,
		&status, &r.StartsAt, &r.ExpiresAt, &r.ManagerApproverID,
		&r.SAPApproverID, &r.LifecycleID, &r.CreatedAt, &r.UpdatedAt, &r.Revision)
	if err != nil {
		return nil, err
	}
	r.Status = domain.RequestStatus(status)
	return &r, nil
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (r Repository) ListAccessRequests(ctx context.Context, q domain.ListAccessRequestsQuery) ([]*domain.AccessRequest, int64, error) {
	args := []any{q.OrganizationID}
	extra := ""
	if q.RequesterID != "" {
		extra, args = shared.AppendParamCondition(extra, "requester_id", q.RequesterID, args)
	}
	if q.Status != nil && *q.Status != "" {
		args = append(args, string(*q.Status))
		if len(args) == 2 {
			extra += " AND COALESCE(workflow_state, status) = $2"
		} else {
			extra += " AND COALESCE(workflow_state, status) = $3"
		}
	}
	var total int64
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM guliangiam.tickets WHERE organization_id = $1 AND type = 'iga_access_request' AND deleted_at IS NULL"+extra, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := shared.ClampPageSize(q.PageSize)
	query := qListAccessRequestsBase + extra + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, limit, shared.PageOffset(q.Page, q.PageSize))
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []*domain.AccessRequest{}
	for rows.Next() {
		item, err := scanAccessRequest(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r Repository) GetAccessRequest(ctx context.Context, orgID, id string, forUpdate bool) (*domain.AccessRequest, error) {
	query := qGetAccessRequest
	if forUpdate {
		query = qGetAccessRequestForUpdate
	}
	item, err := scanAccessRequest(r.db.QueryRow(ctx, query, orgID, id))
	if err == pgx.ErrNoRows {
		return nil, apperrors.NewNotFound("access request not found")
	}
	return item, err
}

func (r Repository) CreateAccessRequest(ctx context.Context, item *domain.AccessRequest) error {
	name := "iga-" + item.ID
	_, err := r.db.Exec(ctx, qInsertAccessRequest, item.ID, item.OrganizationID, name,
		"IGA access request", item.RequesterID, item.Reason,
		item.RequesterID, item.RoleID, item.ApplicationID, item.EnforcerID,
		item.Resource, item.Action, item.StartsAt, item.ExpiresAt, item.Status,
		item.CreatedAt, item.UpdatedAt, item.Revision)
	return err
}

func (r Repository) UpdateAccessRequest(ctx context.Context, item *domain.AccessRequest, expected domain.RequestStatus) (bool, error) {
	result, err := r.db.Exec(ctx, qUpdateAccessRequest, item.Status,
		nullableUUID(item.ManagerApproverID), nullableUUID(item.SAPApproverID),
		nullableUUID(item.LifecycleID), item.UpdatedAt, item.OrganizationID,
		item.ID, expected, item.Revision)
	if err != nil {
		return false, err
	}
	if result.RowsAffected() == 0 {
		return false, nil
	}
	item.Revision++
	return true, nil
}

func (r Repository) AddTicketMessage(ctx context.Context, m *domain.TicketMessage) error {
	_, err := r.db.Exec(ctx, qInsertTicketMessage, m.ID, m.TicketID, nullableUUID(m.AuthorID), m.Action, m.Content, m.CreatedAt)
	return err
}

func (r Repository) ListTicketMessages(ctx context.Context, orgID, ticketID string) ([]*domain.TicketMessage, error) {
	rows, err := r.db.Query(ctx, qListTicketMessages, orgID, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*domain.TicketMessage{}
	for rows.Next() {
		var m domain.TicketMessage
		if err := rows.Scan(&m.ID, &m.TicketID, &m.AuthorID, &m.Action, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, &m)
	}
	return items, rows.Err()
}

func (r Repository) ValidateAccessTargets(ctx context.Context, orgID, userID, roleID, applicationID, enforcerID string) (domain.AccessTargetValidity, error) {
	var v domain.AccessTargetValidity
	err := r.db.QueryRow(ctx, qValidateAccessTargets, orgID, userID, roleID, applicationID, enforcerID).Scan(
		&v.UserValid, &v.RoleValid, &v.ApplicationValid, &v.EnforcerValid, &v.Subject, &v.OrganizationSlug)
	return v, err
}

func (r Repository) ListEffectiveRoleIDs(ctx context.Context, orgID, userID string, at time.Time) ([]string, error) {
	rows, err := r.db.Query(ctx, qListEffectiveRoleIDs, orgID, userID, at)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r Repository) ResolveDefaultEnforcer(ctx context.Context, orgID string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, qResolveDefaultEnforcer, orgID).Scan(&id)
	if err == pgx.ErrNoRows {
		return "", apperrors.NewNotFound("enabled enforcer not found")
	}
	return id, err
}

func (r Repository) CreateJITBinding(ctx context.Context, b *domain.JITBinding) error {
	_, err := r.db.Exec(ctx, qInsertJITBinding, b.UserID, b.RoleID, b.StartsAt, b.ExpiresAt, b.LifecycleID)
	return err
}

func (r Repository) CreateJITPolicy(ctx context.Context, p *domain.JITPolicy) error {
	metadata, err := json.Marshal(map[string]any{
		"source": "iga_jit", "lifecycleId": p.LifecycleID,
		"startsAt": p.StartsAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, qInsertJITPolicy, p.ID, p.EnforcerID, p.Subject,
		p.OrganizationSlug, p.Resource, p.Action,
		p.ExpiresAt.UTC().Format(time.RFC3339Nano), string(metadata))
	return err
}

func (r Repository) RevokeJITArtifacts(ctx context.Context, lifecycleID, policyID string) error {
	if _, err := r.db.Exec(ctx, qDeleteJITBinding, lifecycleID); err != nil {
		return err
	}
	if policyID != "" {
		if _, err := r.db.Exec(ctx, qDeleteJITPolicy, policyID); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) RevokePrivilegedSessions(ctx context.Context, lifecycleID string, at time.Time) (int64, error) {
	result, err := r.db.Exec(ctx, qRevokePrivilegedSessions, lifecycleID, at)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r Repository) GetLifecycleForUpdate(ctx context.Context, orgID, id string) (*domain.AccessLifecycle, error) {
	l, err := scanLifecycle(r.db.QueryRow(ctx, qGetLifecycleForUpdate, id, orgID))
	if err == pgx.ErrNoRows {
		return nil, apperrors.NewNotFound("lifecycle grant not found")
	}
	return l, err
}

func (r Repository) ListExpiringLifecycle(ctx context.Context, orgID string, before time.Time, limit int) ([]*domain.AccessLifecycle, error) {
	if limit <= 0 {
		limit = 100
	}
	query := qListExpiringLifecycle
	args := []any{before, limit}
	if orgID != "" {
		query = qListExpiringLifecycleByOrg
		args = []any{orgID, before, limit}
	}
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*domain.AccessLifecycle{}
	for rows.Next() {
		l, err := scanLifecycle(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	return items, rows.Err()
}

func (r Repository) CompleteLifecycle(ctx context.Context, orgID, id string, status domain.LifecycleStatus, at time.Time) (bool, error) {
	result, err := r.db.Exec(ctx, qCompleteLifecycle, status, at, orgID, id)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}

func (r Repository) RecordLifecycleFailure(ctx context.Context, orgID, id, message string) error {
	_, err := r.db.Exec(ctx, qRecordLifecycleFailure, message, orgID, id)
	return err
}
