package invitations

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 invitations 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// invitationRow 是 invitations 表扫描目标。
// RoleIDs / GroupIDs PG text[] → MySQL TEXT (JSON 数组字符串)
type invitationRow struct {
	ID             string
	OrganizationID string
	Email          string
	Code           string
	RoleIDs        string // JSON 数组 → []string
	GroupIDs       string // JSON 数组 → []string
	Status         string
	ExpiresAt      time.Time
	AcceptedAt     *time.Time
	InvitedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// listArgs 封装 List 的参数构造。
// 占位符：org_id×2(2), keyword(1), status×2(2), limit, offset(2) = 7
//
//	 CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END  → 2
//	 AND (? = '' OR email LIKE CONCAT('%', ?, '%'))          → 2
//	 AND (? = '' OR status = ?)                              → 2
//	 LIMIT ?, OFFSET ?                                       → 2
//	总计 = 8
func listArgs(q domain.ListQuery) []any {
	return []any{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword,
		q.Status, q.Status,
		mysqlshared.ClampPageSize(q.PageSize), mysqlshared.PageOffset(q.Page, q.PageSize),
	}
}

// countArgs 跟 listArgs 完全一样，只是去掉最后 limit, offset
func countArgs(q domain.ListQuery) []any {
	return []any{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword,
		q.Status, q.Status,
	}
}

// scanInvitationRow 扫描一行到 invitationRow。
func scanInvitationRow(scanner interface{ Scan(...any) error }) (*invitationRow, error) {
	var row invitationRow
	err := scanner.Scan(
		&row.ID, &row.OrganizationID, &row.Email, &row.Code,
		&row.RoleIDs, &row.GroupIDs, &row.Status,
		&row.ExpiresAt, &row.AcceptedAt, &row.InvitedBy,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("invitation not found")
		}
		return nil, err
	}
	return &row, nil
}

// GetByID 按 id 查 Invitation。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Invitation, error) {
	row, err := scanInvitationRow(r.db.QueryRowContext(ctx, queryGetInvitationByID, id))
	if err != nil {
		return nil, err
	}
	return toInvitation(*row), nil
}

// GetByIDForOrganization 按 id + organization_id 查 Invitation（防止跨租户读取）。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Invitation, error) {
	row, err := scanInvitationRow(r.db.QueryRowContext(ctx, queryGetInvitationByIDForOrganization, id, organizationID))
	if err != nil {
		return nil, err
	}
	return toInvitation(*row), nil
}

// GetByCode 按 code 查 Invitation。
func (r Repository) GetByCode(ctx context.Context, code string) (*domain.Invitation, error) {
	row, err := scanInvitationRow(r.db.QueryRowContext(ctx, queryGetInvitationByCode, code))
	if err != nil {
		return nil, err
	}
	return toInvitation(*row), nil
}

// GetByEmail 按 organization_id + email 查最新 pending invitation。
func (r Repository) GetByEmail(ctx context.Context, organizationID, email string) (*domain.Invitation, error) {
	row, err := scanInvitationRow(r.db.QueryRowContext(ctx, queryGetInvitationByEmail, organizationID, email))
	if err != nil {
		return nil, err
	}
	return toInvitation(*row), nil
}

// List 分页查询 Invitations。
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Invitation, int64, error) {
	rows, err := r.db.QueryContext(ctx, queryListInvitations, listArgs(q)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Invitation, 0)
	for rows.Next() {
		var row invitationRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Email, &row.Code,
			&row.RoleIDs, &row.GroupIDs, &row.Status,
			&row.ExpiresAt, &row.AcceptedAt, &row.InvitedBy,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, toInvitation(row))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountInvitations, countArgs(q)...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Create 插入 Invitation。
func (r Repository) Create(ctx context.Context, invitation *domain.Invitation) error {
	roleIDsJSON, err := json.Marshal(invitation.RoleIDs)
	if err != nil {
		return apperrors.NewInternal("encode role_ids")
	}
	groupIDsJSON, err := json.Marshal(invitation.GroupIDs)
	if err != nil {
		return apperrors.NewInternal("encode group_ids")
	}

	_, err = r.db.ExecContext(ctx, queryInsertInvitation,
		invitation.ID, invitation.OrganizationID, invitation.Email, invitation.Code,
		string(roleIDsJSON), string(groupIDsJSON), invitation.Status,
		invitation.ExpiresAt, invitation.AcceptedAt, invitation.InvitedBy,
		invitation.CreatedAt, invitation.UpdatedAt, invitation.Revision,
	)
	return err
}

// Update 乐观锁更新 Invitation。
// PG RETURNING revision → MySQL UPDATE + SELECT 两步。
func (r Repository) Update(ctx context.Context, invitation *domain.Invitation) error {
	roleIDsJSON, err := json.Marshal(invitation.RoleIDs)
	if err != nil {
		return apperrors.NewInternal("encode role_ids")
	}
	groupIDsJSON, err := json.Marshal(invitation.GroupIDs)
	if err != nil {
		return apperrors.NewInternal("encode group_ids")
	}

	result, err := r.db.ExecContext(ctx, queryUpdateInvitation,
		invitation.Email, invitation.Code, string(roleIDsJSON), string(groupIDsJSON),
		invitation.Status, invitation.ExpiresAt, invitation.AcceptedAt,
		invitation.InvitedBy, invitation.UpdatedAt,
		invitation.ID, invitation.Revision,
		invitation.OrganizationID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("invitation revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevisionAfterUpdate, invitation.ID).Scan(&newRevision); err != nil {
		return err
	}
	invitation.Revision = newRevision
	return nil
}

// Delete 按 id 删除 Invitation。
func (r Repository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, queryDeleteInvitation, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("invitation not found")
	}
	return nil
}

// toInvitation 把 MySQL row 转成 domain.Invitation。
func toInvitation(row invitationRow) *domain.Invitation {
	inv := &domain.Invitation{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Email:          row.Email,
		Code:           row.Code,
		Status:         row.Status,
		ExpiresAt:      row.ExpiresAt,
		AcceptedAt:     row.AcceptedAt,
		InvitedBy:      row.InvitedBy,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
	// role_ids JSON → []string
	if row.RoleIDs != "" {
		_ = json.Unmarshal([]byte(row.RoleIDs), &inv.RoleIDs)
	}
	if inv.RoleIDs == nil {
		inv.RoleIDs = []string{}
	}
	// group_ids JSON → []string
	if row.GroupIDs != "" {
		_ = json.Unmarshal([]byte(row.GroupIDs), &inv.GroupIDs)
	}
	if inv.GroupIDs == nil {
		inv.GroupIDs = []string{}
	}
	return inv
}

// 编译期接口断言
var _ domain.Repository = Repository{}

// ListExpiring 扫描即将过期且仍未接受的邀请。仅供维护作业在系统作用域调用：
// 不做组织过滤，跨租户返回候选行由调用方按 organization_id 分组投递。
func (r Repository) ListExpiring(ctx context.Context, now, deadline time.Time, limit int) ([]*domain.Invitation, error) {
	rows, err := r.db.QueryContext(ctx, queryListExpiringInvitations, now, deadline, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*domain.Invitation, 0)
	for rows.Next() {
		var row invitationRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Email, &row.Code,
			&row.RoleIDs, &row.GroupIDs, &row.Status,
			&row.ExpiresAt, &row.AcceptedAt, &row.InvitedBy,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, err
		}
		items = append(items, toInvitation(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

var _ domain.ExpiringScanner = Repository{}
