package kerberostickets

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
)

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
