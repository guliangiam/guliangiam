package lockoutattempts

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	"github.com/stretchr/testify/assert"
)

func TestToDomain(t *testing.T) {
	now := time.Now()
	row := attemptRow{
		ID:              "019abc",
		UserID:          "019user",
		Attempts:        3,
		LockedUntil:     nil,
		WindowStartedAt: now,
		Revision:        7,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	result := toDomain(row)

	assert.Equal(t, row.ID, result.ID)
	assert.Equal(t, row.UserID, result.UserID)
	assert.Equal(t, row.Attempts, result.Attempts)
	assert.Nil(t, result.LockedUntil)
	assert.Equal(t, row.WindowStartedAt, result.WindowStartedAt)
	assert.Equal(t, row.Revision, result.Revision)
	assert.Equal(t, row.CreatedAt, result.CreatedAt)
	assert.Equal(t, row.UpdatedAt, result.UpdatedAt)
}

func TestToDomain_Locked(t *testing.T) {
	now := time.Now()
	lockedUntil := now.Add(30 * time.Minute)
	row := attemptRow{
		ID:              "019def",
		UserID:          "019user",
		Attempts:        5,
		LockedUntil:     &lockedUntil,
		WindowStartedAt: now,
		LockoutLevel:    1,
		Revision:        2,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	result := toDomain(row)

	assert.Equal(t, 5, result.Attempts)
	assert.NotNil(t, result.LockedUntil)
	assert.Equal(t, lockedUntil, *result.LockedUntil)
	assert.Equal(t, 1, result.LockoutLevel)
	assert.Equal(t, int64(2), result.Revision)
}

func TestToDomain_ZeroAttempts(t *testing.T) {
	now := time.Now()
	row := attemptRow{
		ID:          "019zero",
		UserID:      "019user2",
		Attempts:    0,
		LockedUntil: nil,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	result := toDomain(row)

	assert.Equal(t, 0, result.Attempts)
	assert.Nil(t, result.LockedUntil)
}

func TestNewRepository(t *testing.T) {
	repo := NewRepository(nil)
	assert.NotNil(t, repo)
}

func TestRepository_ImplementsInterface(t *testing.T) {
	var _ domain.Repository = Repository{}
}

func TestSQLConstants_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, queryGetByUserID)
	assert.NotEmpty(t, queryCreate)
	assert.NotEmpty(t, queryUpdate)
	assert.NotEmpty(t, queryDeleteByUserID)
	assert.NotEmpty(t, queryDeleteExpired)
}

func TestSQLConstants_DeleteExpired_FiltersLocked(t *testing.T) {
	// DeleteExpired should only delete entries where lockout has expired
	assert.Contains(t, queryDeleteExpired, "locked_until IS NOT NULL")
	assert.Contains(t, queryDeleteExpired, "locked_until < NOW()")
}

func TestSQLConstants_TableName(t *testing.T) {
	assert.Contains(t, queryGetByUserID, "lockout_attempts")
	assert.Contains(t, queryCreate, "lockout_attempts")
	assert.Contains(t, queryUpdate, "lockout_attempts")
	assert.Contains(t, queryUpdate, "revision = revision + 1")
	assert.Contains(t, queryUpdate, "AND revision = $7")
	assert.Contains(t, queryDeleteByUserID, "lockout_attempts")
}
