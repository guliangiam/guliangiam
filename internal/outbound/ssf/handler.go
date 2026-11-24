// Package ssf delivers CAEP/SSF security events to registered receivers via
// the shared Outbox worker (ADR-008 semantics: retry/dead-letter are owned by
// the worker, this handler only performs signed HTTP pushes).
package ssf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	ssfdomain "github.com/guliangiam/guliangiam/internal/domain/ssf"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	platformssf "github.com/guliangiam/guliangiam/internal/platform/ssf"
)

// AggregateType is the Outbox routing key for SSF security events.
const AggregateType = "ssf_event"

type headers struct {
	ReceiverID     string `json:"receiver_id"`
	OrganizationID string `json:"organization_id"`
}

type payload struct {
	Type      string         `json:"type"`
	Issuer    string         `json:"issuer"`
	Audience  string         `json:"audience"`
	EventID   string         `json:"eventId"`
	Subject   map[string]any `json:"subject"`
	EventData map[string]any `json:"eventData"`
}

// Signer is the SET signing port (platform/ssf.Signer).
type Signer = platformssf.Signer

// Handler delivers one SSF event per Outbox item. An item may target one
// receiver (receiver_id) or fan out to all active subscribers of the org.
type Handler struct {
	receivers ssfdomain.Repository
	signer    Signer
	client    *http.Client
}

// NewHandler constructs the DeliveryHandler.
func NewHandler(receivers ssfdomain.Repository, signer Signer, client *http.Client) platformoutbox.DeliveryHandler {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Handler{receivers: receivers, signer: signer, client: client}
}

// Handle loads the receiver(s), signs the SET, and POSTs it.
func (h *Handler) Handle(ctx context.Context, item *platformoutbox.OutboxItem) error {
	var meta headers
	if err := json.Unmarshal(item.Headers, &meta); err != nil {
		return fmt.Errorf("ssf: parse headers: %w", err)
	}
	if meta.OrganizationID == "" {
		return apperrors.NewInvalidArgument("ssf: headers must contain organization_id")
	}
	var event payload
	if err := json.Unmarshal(item.Payload, &event); err != nil {
		return fmt.Errorf("ssf: parse event payload: %w", err)
	}

	if meta.ReceiverID == "" {
		receivers, err := h.receivers.ListByOrganization(ctx, meta.OrganizationID)
		if err != nil {
			return fmt.Errorf("ssf: list receivers: %w", err)
		}
		for _, receiver := range receivers {
			if err := h.deliver(ctx, receiver, event); err != nil {
				return err
			}
		}
		return nil
	}

	receiver, err := h.receivers.GetByID(ctx, meta.OrganizationID, meta.ReceiverID)
	if err != nil {
		return fmt.Errorf("ssf: load receiver: %w", err)
	}
	return h.deliver(ctx, receiver, event)
}

func (h *Handler) deliver(ctx context.Context, receiver *ssfdomain.Receiver, event payload) error {
	if receiver.Status != "active" {
		// Disabled receivers are intentionally skipped (not retried).
		return nil
	}
	if !receiverAllows(receiver.EventTypes, event.Type) {
		return apperrors.NewInvalidArgumentf("ssf: receiver %s does not subscribe to %s", receiver.ID, event.Type)
	}
	token, err := platformssf.SignEvent(h.signer, platformssf.Event{
		Type:      event.Type,
		Issuer:    event.Issuer,
		Audience:  event.Audience,
		EventID:   event.EventID,
		IssuedAt:  time.Now().UTC(),
		Subject:   event.Subject,
		EventData: event.EventData,
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, receiver.ReceiverURL, bytes.NewReader([]byte(token)))
	if err != nil {
		return fmt.Errorf("ssf: build request: %w", err)
	}
	request.Header.Set("Content-Type", "application/secevent+jwt")
	response, err := h.client.Do(request)
	if err != nil {
		return fmt.Errorf("ssf: deliver event: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	return apperrors.NewUnavailablef("ssf: receiver returned status %d", response.StatusCode)
}

func receiverAllows(raw []byte, eventType string) bool {
	if len(raw) == 0 {
		return true
	}
	var types []string
	if err := json.Unmarshal(raw, &types); err != nil || len(types) == 0 {
		return true
	}
	for _, allowed := range types {
		if allowed == eventType || allowed == "*" {
			return true
		}
	}
	return false
}
