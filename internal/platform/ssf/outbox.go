package ssf

import (
	"encoding/json"
	"time"

	guuid "github.com/google/uuid"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// NewRevocationEvent builds one CAEP session-revoked Outbox event. Callers own
// the transaction (when they have one) and enqueue the returned event.
func NewRevocationEvent(issuer, orgID, userID, sessionID, reason string, now time.Time) (*domainoutbox.OutboxEvent, error) {
	if issuer == "" {
		return nil, apperrors.NewInvalidArgument("ssf issuer is required")
	}
	if orgID == "" {
		return nil, apperrors.NewInvalidArgument("ssf organization is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	eventID, err := guuid.NewV7()
	if err != nil {
		return nil, err
	}
	subject := map[string]any{"org_id": orgID}
	if userID != "" {
		subject["sub"] = userID
	}
	if sessionID != "" {
		subject["session_id"] = sessionID
	}
	payload, err := json.Marshal(map[string]any{
		"type":      EventSessionRevoked,
		"issuer":    issuer,
		"eventId":   eventID.String(),
		"subject":   subject,
		"eventData": map[string]any{"reason": reason},
	})
	if err != nil {
		return nil, err
	}
	headers, err := json.Marshal(map[string]string{"organization_id": orgID})
	if err != nil {
		return nil, err
	}
	aggregateID := sessionID
	if aggregateID == "" {
		aggregateID = userID
	}
	organizationID := orgID
	return &domainoutbox.OutboxEvent{
		ID:             guuid.NewString(),
		OrganizationID: &organizationID,
		AggregateType:  "ssf_event",
		AggregateID:    aggregateID,
		EventType:      "ssf.session.revoked",
		EventVersion:   1,
		Payload:        payload,
		Headers:        headers,
		AvailableAt:    now,
	}, nil
}
