package castickets

import "time"

// Ticket represents a CAS service ticket stored in the database.
type Ticket struct {
	ID              string // UUIDv7 primary key
	TicketID        string // CAS ticket identifier (ST-xxx, PGT-xxx, PT-xxx)
	TicketType      string // "service", "proxy_granting", "proxy"
	UserID          string
	Username        string
	Service         string
	ExpiresAt       time.Time
	CreatedAt       time.Time
	PGTIOU          string
	PGTIOUExpiresAt time.Time
	PGTIOUUsed      bool
}
