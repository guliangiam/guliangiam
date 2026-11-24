package castickets

import "time"

type ticketRow struct {
	ID              string
	TicketID        string
	TicketType      string
	UserID          string
	Username        string
	Service         string
	ExpiresAt       time.Time
	CreatedAt       time.Time
	PGTIOU          *string
	PGTIOUExpiresAt *time.Time
	PGTIOUUsed      bool
}
