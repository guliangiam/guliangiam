package kerberostickets

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
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
		&row.ID, &row.TicketID, &row.UserID, &row.Username,
		&row.Service, &row.Realm, &row.ExpiresAt, &row.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("kerberos ticket not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r Repository) Create(ctx context.Context, ticket *domain.Ticket) error {
	_, err := r.db.Exec(ctx, queryCreate,
		ticket.ID, ticket.TicketID, ticket.UserID, ticket.Username,
		ticket.Service, ticket.Realm, ticket.ExpiresAt, ticket.CreatedAt,
	)
	return err
}

func (r Repository) DeleteByTicketID(ctx context.Context, ticketID string) error {
	_, err := r.db.Exec(ctx, queryDeleteByTicketID, ticketID)
	return err
}

func (r Repository) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, queryDeleteExpired)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

var _ domain.Repository = Repository{}
