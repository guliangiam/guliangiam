// Package ssf implements the Shared Signals Framework (CAEP/RISC) security
// event token (SET) building blocks used by outbound risk notifications.
//
// The transport (Outbox delivery worker) is intentionally separate: this
// package only owns the RFC 8417 claims shape and the signing call so the
// token format has one implementation and one test surface.
package ssf

import (
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// CAEP/RISC event type URIs used by this IAM.
const (
	EventSessionRevoked   = "https://schemas.openid.net/secevent/caep/event-type/session-revoked"
	EventCredentialChange = "https://schemas.openid.net/secevent/caep/event-type/credential-change" //nolint:gosec // G101: SSF event-type URI, not a credential
	EventRiskLevelChange  = "https://schemas.openid.net/secevent/risc/event-type/risk-level-change"
)

// Signer is the JWT signing port (domain auth.JWKSProvider satisfies it).
type Signer interface {
	SignToken(claims map[string]any, tokenType string) (string, error)
}

// Event describes one security event to transmit.
type Event struct {
	Type      string
	Issuer    string
	Audience  string
	EventID   string
	IssuedAt  time.Time
	Subject   map[string]any
	EventData map[string]any
}

// BuildClaims builds RFC 8417 SET claims. The event type is the key of the
// events map and carries the CAEP/RISC event-specific payload.
func BuildClaims(event Event) (map[string]any, error) {
	if event.Type == "" {
		return nil, apperrors.NewInvalidArgument("security event type is required")
	}
	if event.Issuer == "" {
		return nil, apperrors.NewInvalidArgument("security event issuer is required")
	}
	if event.EventID == "" {
		return nil, apperrors.NewInvalidArgument("security event id is required")
	}
	issuedAt := event.IssuedAt
	if issuedAt.IsZero() {
		issuedAt = time.Now().UTC()
	}
	eventPayload := event.EventData
	if eventPayload == nil {
		eventPayload = map[string]any{}
	}
	if len(event.Subject) > 0 {
		eventPayload = cloneMap(eventPayload)
		eventPayload["subject"] = event.Subject
	}
	claims := map[string]any{
		"iss": event.Issuer,
		"jti": event.EventID,
		"iat": issuedAt.Unix(),
		"events": map[string]any{
			event.Type: eventPayload,
		},
	}
	if event.Audience != "" {
		claims["aud"] = event.Audience
	}
	return claims, nil
}

// SignEvent builds and signs a SET. tokenType is "secevent+jwt" per RFC 8417.
func SignEvent(signer Signer, event Event) (string, error) {
	if signer == nil {
		return "", apperrors.NewUnavailable("security event signer is not configured")
	}
	claims, err := BuildClaims(event)
	if err != nil {
		return "", err
	}
	token, err := signer.SignToken(claims, "secevent+jwt")
	if err != nil {
		return "", apperrors.WrapInternalf(err, "sign security event token")
	}
	return token, nil
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input)+1)
	for key, value := range input {
		out[key] = value
	}
	return out
}
