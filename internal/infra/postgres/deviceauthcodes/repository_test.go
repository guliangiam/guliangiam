package deviceauthcodes

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	"github.com/stretchr/testify/assert"
)

func TestToDomain(t *testing.T) {
	now := time.Now()
	verifiedAt := now.Add(2 * time.Minute)
	row := deviceAuthCodeRow{
		ID:              "019abc",
		DeviceCode:      "dc-abc123",
		UserCode:        "ABCD-1234",
		ClientID:        "client-xyz",
		Scope:           "read write",
		ExpiresAt:       now.Add(15 * time.Minute),
		PollIntervalSec: 5,
		Verified:        true,
		VerifiedAt:      &verifiedAt,
		UserID:          "019user",
		Username:        "bob",
		OrganizationID:  "019org",
		CreatedAt:       now,
		UpdatedAt:       now,
		Revision:        1,
	}

	result := toDomain(row)

	assert.Equal(t, row.ID, result.ID)
	assert.Equal(t, row.DeviceCode, result.DeviceCode)
	assert.Equal(t, row.UserCode, result.UserCode)
	assert.Equal(t, row.ClientID, result.ClientID)
	assert.Equal(t, row.Scope, result.Scope)
	assert.Equal(t, row.ExpiresAt, result.ExpiresAt)
	assert.Equal(t, row.PollIntervalSec, result.PollIntervalSec)
	assert.True(t, result.Verified)
	assert.NotNil(t, result.VerifiedAt)
	assert.Equal(t, verifiedAt, *result.VerifiedAt)
	assert.Equal(t, row.UserID, result.UserID)
	assert.Equal(t, row.Username, result.Username)
	assert.Equal(t, row.OrganizationID, result.OrganizationID)
	assert.Equal(t, row.CreatedAt, result.CreatedAt)
	assert.Equal(t, row.UpdatedAt, result.UpdatedAt)
	assert.Equal(t, row.Revision, result.Revision)
}

func TestToDomain_Unverified(t *testing.T) {
	now := time.Now()
	row := deviceAuthCodeRow{
		ID:              "019def",
		DeviceCode:      "dc-def456",
		UserCode:        "EFGH-5678",
		ClientID:        "client-xyz",
		Scope:           "read",
		ExpiresAt:       now.Add(15 * time.Minute),
		PollIntervalSec: 5,
		Verified:        false,
		VerifiedAt:      nil,
		UserID:          "",
		Username:        "",
		OrganizationID:  "",
		CreatedAt:       now,
		UpdatedAt:       now,
		Revision:        1,
	}

	result := toDomain(row)

	assert.False(t, result.Verified)
	assert.Nil(t, result.VerifiedAt)
	assert.Empty(t, result.UserID)
	assert.Empty(t, result.Username)
}

func TestNewRepository(t *testing.T) {
	repo := NewRepository(nil)
	assert.NotNil(t, repo)
}

func TestRepository_ImplementsInterface(t *testing.T) {
	var _ domain.Repository = Repository{}
}

func TestSQLConstants_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, queryGetByDeviceCode)
	assert.NotEmpty(t, queryGetByUserCode)
	assert.NotEmpty(t, queryCreate)
	assert.NotEmpty(t, queryUpdateVerification)
	assert.NotEmpty(t, queryDeleteByDeviceCode)
	assert.NotEmpty(t, queryDeleteByUserCode)
	assert.NotEmpty(t, queryDeleteExpired)
}

func TestSQLConstants_TableName(t *testing.T) {
	assert.Contains(t, queryGetByDeviceCode, "device_auth_codes")
	assert.Contains(t, queryGetByUserCode, "device_auth_codes")
	assert.Contains(t, queryCreate, "device_auth_codes")
	assert.Contains(t, queryUpdateVerification, "device_auth_codes")
}
