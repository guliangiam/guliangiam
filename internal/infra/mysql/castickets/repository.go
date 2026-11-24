package castickets

import (
	"context"
	"database/sql"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/castickets"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 castickets 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// ticketRow 是 cas_tickets 表扫描目标。
type ticketRow struct {
	ID              string
	TicketID        string
	TicketType      string
	UserID          string
	Username        string
	Service         string
	ExpiresAt       sql.NullTime
	CreatedAt       sql.NullTime
	PGTIOU          sql.NullString
	PGTIOUExpiresAt sql.NullTime
	PGTIOUUsed      bool
}

// GetByTicketID 按 ticket_id 查 CAS Ticket。
func (r Repository) GetByTicketID(ctx context.Context, ticketID string) (*domain.Ticket, error) {
	var row ticketRow
	err := r.db.QueryRowContext(ctx, queryGetByTicketID, ticketID).Scan(
		&row.ID, &row.TicketID, &row.TicketType, &row.UserID, &row.Username,
		&row.Service, &row.ExpiresAt, &row.CreatedAt,
		&row.PGTIOU, &row.PGTIOUExpiresAt, &row.PGTIOUUsed,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("cas ticket not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

// GetByPGTIOU 按 pgt_iou 查 CAS Ticket。
func (r Repository) GetByPGTIOU(ctx context.Context, pgtIOU string) (*domain.Ticket, error) {
	var row ticketRow
	err := r.db.QueryRowContext(ctx, queryGetByPGTIOU, pgtIOU).Scan(
		&row.ID, &row.TicketID, &row.TicketType, &row.UserID, &row.Username,
		&row.Service, &row.ExpiresAt, &row.CreatedAt,
		&row.PGTIOU, &row.PGTIOUExpiresAt, &row.PGTIOUUsed,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("cas pgtIou not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

// MarkPGTIOUUsed 原子性地标记 pgt_iou 为已消费。
// PG 版是 UPDATE ... RETURNING ticket_id → MySQL 需要两步（UPDATE 判定 affected rows + SELECT ticket_id）。
func (r Repository) MarkPGTIOUUsed(ctx context.Context, pgtIOU string) (string, error) {
	// 先 UPDATE，WHERE 要求 NOT used AND 未过期
	result, err := r.db.ExecContext(ctx, queryMarkPGTIOUUsed, pgtIOU)
	if err != nil {
		return "", err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	if affected == 0 {
		return "", apperrors.NewNotFound("cas pgtIou invalid, already used, or expired")
	}

	// 再 SELECT ticket_id
	var ticketID string
	err = r.db.QueryRowContext(ctx, querySelectPGTTicketID, pgtIOU).Scan(&ticketID)
	if err != nil {
		return "", err
	}
	return ticketID, nil
}

// Create 插入 CAS Ticket。nullable 字段传 nil → MySQL NULL。
func (r Repository) Create(ctx context.Context, ticket *domain.Ticket) error {
	_, err := r.db.ExecContext(ctx, queryCreate,
		ticket.ID, ticket.TicketID, ticket.TicketType, ticket.UserID, ticket.Username,
		ticket.Service, ticket.ExpiresAt, ticket.CreatedAt,
		nullIfEmpty(ticket.PGTIOU), nullTimeIfZero(ticket.PGTIOUExpiresAt), ticket.PGTIOUUsed,
	)
	return err
}

// DeleteByTicketID 先 SELECT 拿到 ticket 再 DELETE（MySQL 不支持 DELETE RETURNING）。
func (r Repository) DeleteByTicketID(ctx context.Context, ticketID string) (*domain.Ticket, error) {
	var row ticketRow
	err := r.db.QueryRowContext(ctx, queryGetByTicketID, ticketID).Scan(
		&row.ID, &row.TicketID, &row.TicketType, &row.UserID, &row.Username,
		&row.Service, &row.ExpiresAt, &row.CreatedAt,
		&row.PGTIOU, &row.PGTIOUExpiresAt, &row.PGTIOUUsed,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("cas ticket not found")
		}
		return nil, err
	}
	if _, err := r.db.ExecContext(ctx, queryDeleteByTicketID, ticketID); err != nil {
		return nil, err
	}
	return toDomain(row), nil
}

// DeleteExpired 删除所有已过期的 CAS Ticket。
func (r Repository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, queryDeleteExpired)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// toDomain 把 MySQL row 转成 domain.Ticket。
func toDomain(row ticketRow) *domain.Ticket {
	t := &domain.Ticket{
		ID:         row.ID,
		TicketID:   row.TicketID,
		TicketType: row.TicketType,
		UserID:     row.UserID,
		Username:   row.Username,
		Service:    row.Service,
		PGTIOUUsed: row.PGTIOUUsed,
	}
	if row.ExpiresAt.Valid {
		t.ExpiresAt = row.ExpiresAt.Time
	}
	if row.CreatedAt.Valid {
		t.CreatedAt = row.CreatedAt.Time
	}
	if row.PGTIOU.Valid {
		t.PGTIOU = row.PGTIOU.String
	}
	if row.PGTIOUExpiresAt.Valid {
		t.PGTIOUExpiresAt = row.PGTIOUExpiresAt.Time
	}
	return t
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullTimeIfZero(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// 编译期接口断言
var _ domain.Repository = Repository{}
