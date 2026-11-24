package kerberostickets

import "time"

// Ticket represents a Kerberos service ticket stored in the database.
type Ticket struct {
	ID        string // UUIDv7 primary key
	TicketID  string // Kerberos ticket identifier (KRBT-xxx)
	UserID    string
	Username  string
	Service   string
	Realm     string
	ExpiresAt time.Time
	CreatedAt time.Time
}
