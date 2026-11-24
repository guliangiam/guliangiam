package kerberostickets

import "context"

// Repository defines the interface for Kerberos ticket persistence.
type Repository interface {
	GetByTicketID(ctx context.Context, ticketID string) (*Ticket, error)
	Create(ctx context.Context, ticket *Ticket) error
	DeleteByTicketID(ctx context.Context, ticketID string) error
	DeleteExpired(ctx context.Context) (int64, error)
}
