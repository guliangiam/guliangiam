package faceidsessions

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/faceidsessions"
)

func toDomain(row sessionRow) *domain.Session {
	return &domain.Session{
		ID:           row.ID,
		SessionID:    row.SessionID,
		Challenge:    row.Challenge,
		UserID:       row.UserID,
		ExpiresAt:    row.ExpiresAt,
		Verified:     row.Verified,
		LivenessOK:   row.LivenessOK,
		AttemptCount: row.AttemptCount,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
