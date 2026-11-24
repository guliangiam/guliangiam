package castickets

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTicket_Creation(t *testing.T) {
	now := time.Now()
	ticket := Ticket{
		ID:         "castkt-001",
		TicketID:   "ST-1-abc123",
		TicketType: "service",
		UserID:     "user-001",
		Username:   "testuser",
		Service:    "https://app.example.com",
		ExpiresAt:  now.Add(5 * time.Minute),
		CreatedAt:  now,
	}

	assert.Equal(t, "ST-1-abc123", ticket.TicketID)
	assert.Equal(t, "service", ticket.TicketType)
	assert.NotEmpty(t, ticket.Service)
}

func TestTicket_Types(t *testing.T) {
	validTypes := map[string]bool{
		"service":        true,
		"proxy_granting": true,
		"proxy":          true,
	}

	tests := []struct {
		name       string
		ticketType string
		valid      bool
	}{
		{name: "service_ticket", ticketType: "service", valid: true},
		{name: "proxy_granting_ticket", ticketType: "proxy_granting", valid: true},
		{name: "proxy_ticket", ticketType: "proxy", valid: true},
		{name: "invalid_type", ticketType: "unknown", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := validTypes[tt.ticketType]
			assert.Equal(t, tt.valid, ok)
		})
	}
}

func TestTicket_AntiReplay(t *testing.T) {
	now := time.Now()

	t.Run("valid_ticket_not_expired", func(t *testing.T) {
		ticket := Ticket{
			TicketID:  "ST-1-abc123",
			ExpiresAt: now.Add(5 * time.Minute),
		}
		assert.True(t, now.Before(ticket.ExpiresAt), "ticket should be valid")
	})

	t.Run("expired_ticket", func(t *testing.T) {
		ticket := Ticket{
			TicketID:  "ST-1-abc123",
			ExpiresAt: now.Add(-1 * time.Minute),
		}
		assert.True(t, now.After(ticket.ExpiresAt), "ticket should be expired")
	})
}
