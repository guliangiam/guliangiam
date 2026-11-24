package castickets

import "context"

// Repository defines the interface for CAS ticket persistence.
type Repository interface {
	GetByTicketID(ctx context.Context, ticketID string) (*Ticket, error)
	GetByPGTIOU(ctx context.Context, pgtIOU string) (*Ticket, error)
	MarkPGTIOUUsed(ctx context.Context, pgtIOU string) (string, error)
	Create(ctx context.Context, ticket *Ticket) error
	DeleteByTicketID(ctx context.Context, ticketID string) (*Ticket, error)
	DeleteExpired(ctx context.Context) (int64, error)
}
