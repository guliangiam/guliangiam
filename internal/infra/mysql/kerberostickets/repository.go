package kerberostickets

import (
	"context"
	"database/sql"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 kerberos_tickets 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// ticketRow 是 SELECT 的扫描目标。
type ticketRow struct {
	ID        string
	TicketID  string
	UserID    string
	Username  string
	Service   string
	Realm     string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// GetByTicketID 按 Kerberos ticket_id 查找票据。
func (r Repository) GetByTicketID(ctx context.Context, ticketID string) (*domain.Ticket, error) {
	var row ticketRow
	err := r.db.QueryRowContext(ctx, queryGetByTicketID, ticketID).Scan(
		&row.ID, &row.TicketID, &row.UserID, &row.Username,
		&row.Service, &row.Realm, &row.ExpiresAt, &row.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("kerberos ticket not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

// Create 插入 Kerberos ticket。
func (r Repository) Create(ctx context.Context, ticket *domain.Ticket) error {
	_, err := r.db.ExecContext(ctx, queryCreate,
		ticket.ID, ticket.TicketID, ticket.UserID, ticket.Username,
		ticket.Service, ticket.Realm, ticket.ExpiresAt, ticket.CreatedAt,
	)
	return err
}

// DeleteByTicketID 按 ticket_id 删除单条记录。
func (r Repository) DeleteByTicketID(ctx context.Context, ticketID string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteByTicketID, ticketID)
	return err
}

// DeleteExpired 清理所有已过期票据，返回受影响行数。
func (r Repository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, queryDeleteExpired)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// toDomain 把 DB 扫描行转成 domain.Ticket。
func toDomain(row ticketRow) *domain.Ticket {
	return &domain.Ticket{
		ID:        row.ID,
		TicketID:  row.TicketID,
		UserID:    row.UserID,
		Username:  row.Username,
		Service:   row.Service,
		Realm:     row.Realm,
		ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt,
	}
}

// 编译期接口断言
var _ domain.Repository = Repository{}
