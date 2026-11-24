package castickets

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/castickets"
)

func toDomain(row ticketRow) *domain.Ticket {
	t := &domain.Ticket{
		ID:         row.ID,
		TicketID:   row.TicketID,
		TicketType: row.TicketType,
		UserID:     row.UserID,
		Username:   row.Username,
		Service:    row.Service,
		ExpiresAt:  row.ExpiresAt,
		CreatedAt:  row.CreatedAt,
		PGTIOUUsed: row.PGTIOUUsed,
	}
	if row.PGTIOU != nil {
		t.PGTIOU = *row.PGTIOU
	}
	if row.PGTIOUExpiresAt != nil {
		t.PGTIOUExpiresAt = *row.PGTIOUExpiresAt
	}
	return t
}
