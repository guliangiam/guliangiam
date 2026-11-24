// Package providerlogout defines the Outbox contract for asynchronous upstream
// OAuth provider logout.
//
// The logout request path only revokes the local session and enqueues this
// event; the outbox worker performs the external POST. This keeps the request
// path free of synchronous outbound calls (S-12).
package providerlogout

import (
	"encoding/json"
	"strings"
	"time"

	guuid "github.com/google/uuid"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	// AggregateType is the Outbox routing key for provider logout deliveries.
	AggregateType = "provider_logout"
	// EventType is the domain event name recorded on the outbox row.
	EventType = "provider.logout.requested"
	// EventVersion is the payload schema version.
	EventVersion = 1
)

// Payload is the outbox payload for one provider logout.
//
// The upstream access token is stored sealed (the same field encryption used
// for sessions.device_info); the handler opens it only in memory. The provider
// configuration (logout URL, client credentials, headers) is intentionally not
// copied here: it is loaded at delivery time so a rotated secret or a changed
// endpoint is not replayed from a stale payload.
type Payload struct {
	ProviderID   string `json:"providerId"`
	ProviderType string `json:"providerType,omitempty"`
	SessionID    string `json:"sessionId,omitempty"`
	// SealedAccessToken is the provider access token sealed during social login.
	SealedAccessToken string `json:"sealedAccessToken"`
	// IdempotencyKey identifies one logical logout. It equals the outbox
	// aggregate_id and lets the consumer deduplicate within its own state;
	// the outbox table carries no unique constraint on
	// (aggregate_type, aggregate_id, event_type), so delivery is at-least-once
	// and final idempotency relies on the upstream revoke endpoint being
	// idempotent (see handler contract).
	IdempotencyKey string    `json:"idempotencyKey"`
	RequestedAt    time.Time `json:"requestedAt"`
}

// Request is the input for NewEvent.
type Request struct {
	OrganizationID    string
	SessionID         string
	ProviderID        string
	ProviderType      string
	SealedAccessToken string
	RequestedAt       time.Time
}

// NewEvent builds one provider-logout Outbox event. The caller owns the
// transaction and enqueues the returned event.
func NewEvent(req Request) (*domainoutbox.OutboxEvent, error) {
	organizationID := strings.TrimSpace(req.OrganizationID)
	if organizationID == "" {
		return nil, apperrors.NewInvalidArgument("provider logout: organization is required")
	}
	providerID := strings.TrimSpace(req.ProviderID)
	if providerID == "" {
		return nil, apperrors.NewInvalidArgument("provider logout: provider is required")
	}
	sealedToken := strings.TrimSpace(req.SealedAccessToken)
	if sealedToken == "" {
		return nil, apperrors.NewInvalidArgument("provider logout: sealed access token is required")
	}
	sessionID := strings.TrimSpace(req.SessionID)
	aggregateID := sessionID
	if aggregateID == "" {
		aggregateID = providerID
	}
	requestedAt := req.RequestedAt.UTC()
	if req.RequestedAt.IsZero() {
		requestedAt = time.Now().UTC()
	}

	payload, err := json.Marshal(Payload{
		ProviderID:        providerID,
		ProviderType:      strings.TrimSpace(req.ProviderType),
		SessionID:         sessionID,
		SealedAccessToken: sealedToken,
		IdempotencyKey:    aggregateID,
		RequestedAt:       requestedAt,
	})
	if err != nil {
		return nil, err
	}

	return &domainoutbox.OutboxEvent{
		ID:             guuid.NewString(),
		OrganizationID: &organizationID,
		AggregateType:  AggregateType,
		AggregateID:    aggregateID,
		EventType:      EventType,
		EventVersion:   EventVersion,
		Payload:        payload,
		AvailableAt:    requestedAt,
	}, nil
}
