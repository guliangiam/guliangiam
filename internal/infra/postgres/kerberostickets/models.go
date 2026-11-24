package kerberostickets

import "time"

type ticketRow struct {
	ID        string
	TicketID  string
	UserID    string
	Username  string
	Service   string
	Realm     string
	ExpiresAt time.Time
	CreatedAt time.Time
}
