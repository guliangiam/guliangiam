package lockoutattempts

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
)

func toDomain(row attemptRow) *domain.Attempt {
	return &domain.Attempt{
		ID:              row.ID,
		UserID:          row.UserID,
		Attempts:        row.Attempts,
		LockedUntil:     row.LockedUntil,
		WindowStartedAt: row.WindowStartedAt,
		LockoutLevel:    row.LockoutLevel,
		Revision:        row.Revision,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
