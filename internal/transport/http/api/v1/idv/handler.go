package idv

import (
	"context"

	appidv "github.com/guliangiam/guliangiam/internal/app/idv"
)

// Handler exposes the IDV app service to HTTP transport.
type Handler struct {
	Service appidv.Service
}

// ─── Create ───────────────────────────────────────────────────────────────

type CreateRequest struct {
	UserID       *string `json:"userId,omitempty"`
	Provider     string  `json:"provider"`
	RedirectURL  *string `json:"redirectUrl,omitempty"`
	SessionToken *string `json:"sessionToken,omitempty"`
	ProviderRef  *string `json:"providerRef,omitempty"`
}

func (h Handler) Create(ctx context.Context, organizationID string, req CreateRequest) (appidv.VerificationDTO, error) {
	return h.Service.Create(ctx, appidv.CreateCommand{
		OrganizationID: organizationID,
		UserID:         req.UserID,
		Provider:       req.Provider,
		RedirectURL:    req.RedirectURL,
		SessionToken:   req.SessionToken,
		ProviderRef:    req.ProviderRef,
	})
}

// ─── List ─────────────────────────────────────────────────────────────────

type ListQuery struct {
	OrganizationID string
	UserID         *string
	Provider       *string
	Status         *string
	Page           int
	PageSize       int
}

func (h Handler) List(ctx context.Context, q ListQuery) ([]appidv.VerificationDTO, int64, error) {
	var statusPtr *appidv.VerificationStatus
	if q.Status != nil {
		s := appidv.VerificationStatus(*q.Status)
		statusPtr = &s
	}
	return h.Service.List(ctx, appidv.ListQuery{
		OrganizationID: q.OrganizationID,
		UserID:         q.UserID,
		Provider:       q.Provider,
		Status:         statusPtr,
		Page:           q.Page,
		PageSize:       q.PageSize,
	})
}

// ─── GetByID ─────────────────────────────────────────────────────────────

func (h Handler) GetByID(ctx context.Context, id, organizationID string) (*appidv.VerificationDTO, error) {
	dto, err := h.Service.GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	if appidv.VerificationStatus(dto.Status) == appidv.StatusPending ||
		appidv.VerificationStatus(dto.Status) == appidv.StatusInProgress {
		refreshed, refreshErr := h.Service.Refresh(ctx, id, organizationID, "")
		if refreshErr != nil {
			return nil, refreshErr
		}
		return &refreshed, nil
	}
	return dto, nil
}

// ─── Callback ─────────────────────────────────────────────────────────────

// maxCallbackBodyBytes bounds the raw webhook body. Identity-provider callbacks
// are small JSON documents; a larger body is rejected before any parsing.
const maxCallbackBodyBytes = 256 << 10

// HandleCallback processes a public provider webhook. The raw bytes are passed
// through untouched: signature verification must run over exactly what the
// provider signed.
func (h Handler) HandleCallback(ctx context.Context, provider string, payload []byte, headers map[string]string) (*appidv.VerificationResult, error) {
	return h.Service.HandlePublicCallback(ctx, provider, payload, headers)
}

// ─── Delete ───────────────────────────────────────────────────────────────

func (h Handler) Delete(ctx context.Context, id, organizationID string) error {
	return h.Service.Delete(ctx, id, organizationID)
}
