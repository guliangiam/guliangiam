package mfachallenges

import (
	"log/slog"

	domain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func toDomain(row challengeRow) *domain.Challenge {
	challenge := &domain.Challenge{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		UserID:         row.UserID,
		ApplicationID:  row.ApplicationID,
		State:          row.State,
		MethodType:     row.MethodType,
		ExpiresAt:      row.ExpiresAt,
		VerifiedAt:     row.VerifiedAt,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
	hydrateMetadata(challenge, row.Metadata)
	return challenge
}

func hydrateMetadata(challenge *domain.Challenge, raw []byte) {
	if len(raw) == 0 {
		return
	}
	if err := shared.UnmarshalJSON(raw, &challenge.Metadata); err != nil {
		slog.Warn("failed to decode login challenge metadata", "challenge_id", challenge.ID, "error", err)
		return
	}
	if challenge.Metadata == nil {
		return
	}
	if attempts, ok := metadataInt(challenge.Metadata[domain.MetadataKeyFailedAttempts]); ok {
		challenge.FailedAttempts = attempts
	}
}

func metadataInt(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case nil:
		return 0, false
	default:
		return 0, false
	}
}
