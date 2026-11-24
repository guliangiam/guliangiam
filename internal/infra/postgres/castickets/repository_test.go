package castickets

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/castickets"
	"github.com/stretchr/testify/assert"
)

func TestToDomain(t *testing.T) {
	now := time.Now()
	row := ticketRow{
		ID:         "019abc",
		TicketID:   "ST-12345-cas",
		TicketType: "service",
		UserID:     "019user",
		Username:   "alice",
		Service:    "https://app.example.com",
		ExpiresAt:  now.Add(10 * time.Second),
		CreatedAt:  now,
	}

	result := toDomain(row)

	assert.Equal(t, row.ID, result.ID)
	assert.Equal(t, row.TicketID, result.TicketID)
	assert.Equal(t, row.TicketType, result.TicketType)
	assert.Equal(t, row.UserID, result.UserID)
	assert.Equal(t, row.Username, result.Username)
	assert.Equal(t, row.Service, result.Service)
	assert.Equal(t, row.ExpiresAt, result.ExpiresAt)
	assert.Equal(t, row.CreatedAt, result.CreatedAt)
}

func TestToDomain_ProxyGrantingTicket(t *testing.T) {
	now := time.Now()
	row := ticketRow{
		ID:         "019pgt",
		TicketID:   "PGT-98765-cas",
		TicketType: "proxy_granting",
		UserID:     "019user",
		Username:   "bob",
		Service:    "https://backend.example.com/callback",
		ExpiresAt:  now.Add(1 * time.Hour),
		CreatedAt:  now,
	}

	result := toDomain(row)

	assert.Equal(t, "proxy_granting", result.TicketType)
	assert.Equal(t, "PGT-98765-cas", result.TicketID)
}

func TestNewRepository(t *testing.T) {
	repo := NewRepository(nil)
	assert.NotNil(t, repo)
}

func TestRepository_ImplementsInterface(t *testing.T) {
	var _ domain.Repository = Repository{}
}

func TestSQLConstants_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, queryGetByTicketID)
	assert.NotEmpty(t, queryCreate)
	assert.NotEmpty(t, queryDeleteByTicketID)
	assert.NotEmpty(t, queryDeleteExpired)
}

func TestSQLConstants_DeleteByTicketID_Returning(t *testing.T) {
	// DeleteByTicketID uses DELETE ... RETURNING for atomic delete-and-return
	assert.Contains(t, queryDeleteByTicketID, "DELETE")
	assert.Contains(t, queryDeleteByTicketID, "RETURNING")
}

func TestSQLConstants_TableName(t *testing.T) {
	assert.Contains(t, queryGetByTicketID, "cas_tickets")
	assert.Contains(t, queryCreate, "cas_tickets")
	assert.Contains(t, queryDeleteByTicketID, "cas_tickets")
	assert.Contains(t, queryDeleteExpired, "cas_tickets")
}
