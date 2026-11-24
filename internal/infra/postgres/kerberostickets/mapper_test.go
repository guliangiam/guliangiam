package kerberostickets

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToDomain(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		row := ticketRow{
			ID:        "krb-001",
			TicketID:  "TGT-KRB-789",
			UserID:    "user-001",
			Username:  "alice",
			Service:   "HTTP/app.example.com",
			Realm:     "EXAMPLE.COM",
			ExpiresAt: now.Add(24 * time.Hour),
			CreatedAt: now,
		}

		result := toDomain(row)

		assert.Equal(t, "krb-001", result.ID)
		assert.Equal(t, "TGT-KRB-789", result.TicketID)
		assert.Equal(t, "user-001", result.UserID)
		assert.Equal(t, "alice", result.Username)
		assert.Equal(t, "HTTP/app.example.com", result.Service)
		assert.Equal(t, "EXAMPLE.COM", result.Realm)
	})
}
