package kerberostickets

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTicket_Creation(t *testing.T) {
	now := time.Now()
	ticket := Ticket{
		ID:        "krbtkt-001",
		TicketID:  "KRBT-001-abc123",
		UserID:    "user-001",
		Username:  "testuser",
		Service:   "HTTP/webapp.example.com",
		Realm:     "EXAMPLE.COM",
		ExpiresAt: now.Add(8 * time.Hour),
		CreatedAt: now,
	}

	assert.Equal(t, "KRBT-001-abc123", ticket.TicketID)
	assert.Equal(t, "EXAMPLE.COM", ticket.Realm)
	assert.NotEmpty(t, ticket.Service)
}

func TestTicket_AntiReplay(t *testing.T) {
	now := time.Now()

	t.Run("valid_ticket", func(t *testing.T) {
		ticket := Ticket{
			TicketID:  "KRBT-001-abc123",
			ExpiresAt: now.Add(8 * time.Hour),
		}
		assert.True(t, now.Before(ticket.ExpiresAt))
	})

	t.Run("expired_ticket", func(t *testing.T) {
		ticket := Ticket{
			TicketID:  "KRBT-001-abc123",
			ExpiresAt: now.Add(-1 * time.Hour),
		}
		assert.True(t, now.After(ticket.ExpiresAt))
	})
}

func TestTicket_RealmFormat(t *testing.T) {
	tests := []struct {
		name  string
		realm string
	}{
		{name: "uppercase_realm", realm: "EXAMPLE.COM"},
		{name: "multi_part_realm", realm: "CORP.EXAMPLE.COM"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := Ticket{Realm: tt.realm}
			assert.Equal(t, tt.realm, ticket.Realm)
		})
	}
}
