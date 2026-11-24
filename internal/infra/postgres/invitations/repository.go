package invitations

import (
	"context"
	"strings"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
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

func scanInvitation(scanner pgx.Row) (*domain.Invitation, error) {
	var row invitationRow
	var invitedBy *string
	err := scanner.Scan(
		&row.ID, &row.OrganizationID, &row.Email, &row.Code,
		&row.RoleIDs, &row.GroupIDs, &row.Status,
		&row.ExpiresAt, &row.AcceptedAt, &invitedBy,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	row.InvitedBy = stringOrEmpty(invitedBy)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("invitation not found")
		}
		return nil, err
	}
	invitation := toInvitation(row)
	return &invitation, nil
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Invitation, error) {
	return scanInvitation(r.db.QueryRow(ctx, queryGetInvitationByID, id))
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Invitation, error) {
	return scanInvitation(r.db.QueryRow(ctx, queryGetInvitationByIDForOrganization, id, organizationID))
}

func (r Repository) GetByCode(ctx context.Context, code string) (*domain.Invitation, error) {
	return scanInvitation(r.db.QueryRow(ctx, queryGetInvitationByCode, code))
}

func (r Repository) GetByEmail(ctx context.Context, organizationID, email string) (*domain.Invitation, error) {
	return scanInvitation(r.db.QueryRow(ctx, queryGetInvitationByEmail, organizationID, email))
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Invitation, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListInvitations, query.OrganizationID, query.Keyword, query.Status, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Invitation, 0)
	for rows.Next() {
		var row invitationRow
		var invitedBy *string
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Email, &row.Code,
			&row.RoleIDs, &row.GroupIDs, &row.Status,
			&row.ExpiresAt, &row.AcceptedAt, &invitedBy,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		row.InvitedBy = stringOrEmpty(invitedBy)
		invitation := toInvitation(row)
		items = append(items, &invitation)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountInvitations, query.OrganizationID, query.Keyword, query.Status).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, invitation *domain.Invitation) error {
	_, err := r.db.Exec(ctx, queryInsertInvitation,
		invitation.ID, invitation.OrganizationID, invitation.Email, invitation.Code,
		stringSlice(invitation.RoleIDs), stringSlice(invitation.GroupIDs), invitation.Status,
		invitation.ExpiresAt, invitation.AcceptedAt, nullableUUID(invitation.InvitedBy),
		invitation.CreatedAt, invitation.UpdatedAt, invitation.Revision,
	)
	return err
}

func (r Repository) Update(ctx context.Context, invitation *domain.Invitation) error {
	var newRevision int64
	err := r.db.QueryRow(ctx, queryUpdateInvitation,
		invitation.Email, invitation.Code, stringSlice(invitation.RoleIDs), stringSlice(invitation.GroupIDs),
		invitation.Status, invitation.ExpiresAt, invitation.AcceptedAt,
		// App 层已 Revision++，这里要传自增前的值做乐观锁守卫。
		nullableUUID(invitation.InvitedBy), invitation.UpdatedAt, invitation.Revision-1, invitation.ID,
		invitation.OrganizationID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("invitation revision conflict")
		}
		return err
	}
	invitation.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id string) error {
	result, err := r.db.Exec(ctx, queryDeleteInvitation, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewNotFound("invitation not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

// ListExpiring 扫描即将过期且仍未接受的邀请。仅供维护作业在系统作用域调用：
// 不做组织过滤，跨租户返回候选行由调用方按 organization_id 分组投递。
func (r Repository) ListExpiring(ctx context.Context, now, deadline time.Time, limit int) ([]*domain.Invitation, error) {
	rows, err := r.db.Query(ctx, queryListExpiringInvitations, now, deadline, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*domain.Invitation, 0)
	for rows.Next() {
		var row invitationRow
		var invitedBy *string
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Email, &row.Code,
			&row.RoleIDs, &row.GroupIDs, &row.Status,
			&row.ExpiresAt, &row.AcceptedAt, &invitedBy,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, err
		}
		row.InvitedBy = stringOrEmpty(invitedBy)
		invitation := toInvitation(row)
		items = append(items, &invitation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

var _ domain.ExpiringScanner = Repository{}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// nullableUUID 把空字符串转成 NULL：invited_by 是 uuid 列，直接写 "" 会触发
// SQLSTATE 22P02（invalid input syntax for type uuid）。
func nullableUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

// stringSlice 保证 NOT NULL 的 UUID[] 列收到空数组而不是 NULL。
func stringSlice(value []string) []string {
	if value == nil {
		return []string{}
	}
	return value
}
