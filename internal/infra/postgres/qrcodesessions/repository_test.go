package qrcodesessions

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
	"github.com/stretchr/testify/assert"
)

func TestToDomain(t *testing.T) {
	now := time.Now()
	row := sessionRow{
		ID:             "019abc",
		CodeID:         "code-123",
		Code:           "https://iam.example.com/qr/code-123",
		OrganizationID: "019org",
		ApplicationID:  "019app",
		ExpiresAt:      now.Add(5 * time.Minute),
		Status:         "pending",
		UserID:         "",
		Username:       "",
		IPAddress:      "",
		UserAgent:      "",
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	result := toDomain(row)

	assert.Equal(t, row.ID, result.ID)
	assert.Equal(t, row.CodeID, result.CodeID)
	assert.Equal(t, row.Code, result.Code)
	assert.Equal(t, row.OrganizationID, result.OrganizationID)
	assert.Equal(t, row.ApplicationID, result.ApplicationID)
	assert.Equal(t, row.ExpiresAt, result.ExpiresAt)
	assert.Equal(t, row.Status, result.Status)
	assert.Equal(t, row.UserID, result.UserID)
	assert.Equal(t, row.Username, result.Username)
	assert.Equal(t, row.IPAddress, result.IPAddress)
	assert.Equal(t, row.UserAgent, result.UserAgent)
	assert.Equal(t, row.CreatedAt, result.CreatedAt)
	assert.Equal(t, row.UpdatedAt, result.UpdatedAt)
	assert.Equal(t, row.Revision, result.Revision)
}

func TestToDomain_ScannedState(t *testing.T) {
	now := time.Now()
	row := sessionRow{
		ID:             "019abc",
		CodeID:         "code-456",
		Code:           "https://iam.example.com/qr/code-456",
		OrganizationID: "019org",
		ApplicationID:  "019app",
		ExpiresAt:      now.Add(5 * time.Minute),
		Status:         "scanned",
		UserID:         "019user",
		Username:       "alice",
		IPAddress:      "192.168.1.1",
		UserAgent:      "Mozilla/5.0",
		CreatedAt:      now,
		UpdatedAt:      now.Add(1 * time.Minute),
		Revision:       2,
	}

	result := toDomain(row)

	assert.Equal(t, "scanned", result.Status)
	assert.Equal(t, "019user", result.UserID)
	assert.Equal(t, "alice", result.Username)
	assert.Equal(t, "192.168.1.1", result.IPAddress)
	assert.Equal(t, "Mozilla/5.0", result.UserAgent)
	assert.Equal(t, int64(2), result.Revision)
}

func TestNewRepository(t *testing.T) {
	repo := NewRepository(nil)
	assert.NotNil(t, repo)
}

func TestRepository_ImplementsInterface(t *testing.T) {
	var _ domain.Repository = Repository{}
}

func TestSQLConstants_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, queryGetByCodeID)
	assert.NotEmpty(t, queryGetByCode)
	assert.NotEmpty(t, queryCreate)
	assert.NotEmpty(t, queryUpdateStatus)
	assert.NotEmpty(t, queryDeleteByCodeID)
	assert.NotEmpty(t, queryDeleteExpired)
}

func TestSQLConstants_TableName(t *testing.T) {
	assert.Contains(t, queryGetByCodeID, "qr_code_sessions")
	assert.Contains(t, queryGetByCode, "qr_code_sessions")
	assert.Contains(t, queryGetByCode, "WHERE code = $1")
	assert.Contains(t, queryCreate, "qr_code_sessions")
	assert.Contains(t, queryUpdateStatus, "qr_code_sessions")
	assert.Contains(t, queryDeleteByCodeID, "qr_code_sessions")
	assert.Contains(t, queryDeleteExpired, "qr_code_sessions")
}
