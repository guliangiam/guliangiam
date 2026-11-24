package tickets

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/tickets"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 tickets 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// ticketRow 是 tickets 表扫描目标。
type ticketRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Type           string
	Status         string
	Priority       string
	Assignee       string
	Creator        string
	Description    string
	Attachment     string
	ResolvedAt     sql.NullTime
	CreatedAt      sql.NullTime
	UpdatedAt      sql.NullTime
	Revision       int64
}

// GetByID 按 id + organization_id 查 Ticket；跨租户按不存在处理。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Ticket, error) {
	var row ticketRow
	err := r.db.QueryRowContext(ctx, queryGetByID, id, organizationID).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
		&row.Type, &row.Status, &row.Priority, &row.Assignee, &row.Creator,
		&row.Description, &row.Attachment, &row.ResolvedAt,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("ticket not found")
		}
		return nil, err
	}
	return toTicket(row), nil
}

// List 分页列出 Ticket。
// PG ListQuery 额外参数: Assignee, Creator, Priority, Type
// countArgs: org×2 + keyword×3 + status×2 + assignee×2 + creator×2 + priority×2 + type×2 = 15
// listArgs: countArgs + 2 = 17
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Ticket, int64, error) {
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	limit := mysqlshared.ClampPageSize(q.PageSize)

	statusVal := ""
	if q.Status != nil {
		statusVal = *q.Status
	}

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		statusVal, statusVal,
		q.Assignee, q.Assignee,
		q.Creator, q.Creator,
		q.Priority, q.Priority,
		q.Type, q.Type,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountTickets, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)
	rows, err := r.db.QueryContext(ctx, queryListTickets, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Ticket, 0)
	for rows.Next() {
		var row ticketRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
			&row.Type, &row.Status, &row.Priority, &row.Assignee, &row.Creator,
			&row.Description, &row.Attachment, &row.ResolvedAt,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, toTicket(row))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}
	return items, total, nil
}

// Create 插入 Ticket。
func (r Repository) Create(ctx context.Context, t *domain.Ticket) error {
	_, err := r.db.ExecContext(ctx, queryInsertTicket,
		t.ID, t.OrganizationID, t.Name, t.DisplayName,
		t.Type, t.Status, t.Priority, t.Assignee, t.Creator,
		t.Description, t.Attachment, t.ResolvedAt,
		t.CreatedAt, t.UpdatedAt, t.Revision,
	)
	return err
}

// Update 更新 Ticket；WHERE 携带实体 organization_id 作为租户谓词。
func (r Repository) Update(ctx context.Context, t *domain.Ticket) error {
	res, err := r.db.ExecContext(ctx, queryUpdateTicket,
		t.DisplayName, t.Type, t.Status, t.Priority, t.Assignee,
		t.Description, t.Attachment, t.ResolvedAt, t.UpdatedAt, t.Revision,
		t.ID, t.OrganizationID,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("ticket not found")
	}
	return nil
}

// Delete 按 id + organization_id 删除 Ticket；跨租户按不存在处理。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteTicket, id, organizationID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("ticket not found")
	}
	return nil
}

// toTicket 把 MySQL ticketRow 转成 domain.Ticket。
func toTicket(row ticketRow) *domain.Ticket {
	t := &domain.Ticket{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		DisplayName:    row.DisplayName,
		Type:           row.Type,
		Status:         row.Status,
		Priority:       row.Priority,
		Assignee:       row.Assignee,
		Creator:        row.Creator,
		Description:    row.Description,
		Attachment:     row.Attachment,
		Revision:       row.Revision,
	}
	if row.ResolvedAt.Valid {
		t.ResolvedAt = &row.ResolvedAt.Time
	}
	if row.CreatedAt.Valid {
		t.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		t.UpdatedAt = row.UpdatedAt.Time
	}
	return t
}

// 编译期接口断言
var _ domain.Repository = Repository{}
