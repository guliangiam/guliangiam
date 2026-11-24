package castickets

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/castickets"
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

func (r Repository) GetByTicketID(ctx context.Context, ticketID string) (*domain.Ticket, error) {
	var row ticketRow
	err := r.db.QueryRow(ctx, queryGetByTicketID, ticketID).Scan(
		&row.ID, &row.TicketID, &row.TicketType, &row.UserID, &row.Username,
		&row.Service, &row.ExpiresAt, &row.CreatedAt,
		&row.PGTIOU, &row.PGTIOUExpiresAt, &row.PGTIOUUsed,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("cas ticket not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r Repository) GetByPGTIOU(ctx context.Context, pgtIOU string) (*domain.Ticket, error) {
	var row ticketRow
	err := r.db.QueryRow(ctx, queryGetByPGTIOU, pgtIOU).Scan(
		&row.ID, &row.TicketID, &row.TicketType, &row.UserID, &row.Username,
		&row.Service, &row.ExpiresAt, &row.CreatedAt,
		&row.PGTIOU, &row.PGTIOUExpiresAt, &row.PGTIOUUsed,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("cas pgtIou not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

// MarkPGTIOUUsed atomically marks the pgtIou as consumed (one-time-use).
// Returns the bound PGT ticket_id on success. Returns NotFound when:
//   - pgtIou unknown
//   - pgtIou already used
//   - pgtIou expired
func (r Repository) MarkPGTIOUUsed(ctx context.Context, pgtIOU string) (string, error) {
	var pgtTicketID string
	err := r.db.QueryRow(ctx, queryMarkPGTIOUUsed, pgtIOU).Scan(&pgtTicketID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", apperrors.NewNotFound("cas pgtIou invalid, already used, or expired")
		}
		return "", err
	}
	return pgtTicketID, nil
}

func (r Repository) Create(ctx context.Context, ticket *domain.Ticket) error {
	_, err := r.db.Exec(ctx, queryCreate,
		ticket.ID, ticket.TicketID, ticket.TicketType, ticket.UserID, ticket.Username,
		ticket.Service, ticket.ExpiresAt, ticket.CreatedAt,
		nullIfEmpty(ticket.PGTIOU), nullTimeIfZero(ticket.PGTIOUExpiresAt), ticket.PGTIOUUsed,
	)
	return err
}

func (r Repository) DeleteByTicketID(ctx context.Context, ticketID string) (*domain.Ticket, error) {
	var row ticketRow
	err := r.db.QueryRow(ctx, queryDeleteByTicketID, ticketID).Scan(
		&row.ID, &row.TicketID, &row.TicketType, &row.UserID, &row.Username,
		&row.Service, &row.ExpiresAt, &row.CreatedAt,
		&row.PGTIOU, &row.PGTIOUExpiresAt, &row.PGTIOUUsed,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("cas ticket not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r Repository) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, queryDeleteExpired)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
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

var _ domain.Repository = Repository{}
