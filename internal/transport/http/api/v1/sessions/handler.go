package sessions

import (
	"context"

	appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appsessions.Service
}

func (h Handler) List(ctx context.Context, request ListSessionsRequest) (*ListSessionsResponse, error) {
	items, total, err := h.Service.ListSessions(ctx, request.toQuery())
	if err != nil {
		return nil, err
	}

	return newListSessionsResponse(items, presenter.PageMeta{
		Page:     request.Page,
		PageSize: request.PageSize,
		Total:    total,
	}), nil
}

func (h Handler) Get(ctx context.Context, request GetSessionRequest) (*SessionResponse, error) {
	response, err := h.Service.GetSessionForOrganization(ctx, request.ID, request.OrganizationID)
	if err != nil {
		return nil, err
	}

	return newSessionResponse(response), nil
}

func (h Handler) GetCurrent(ctx context.Context, request GetSessionRequest) (*SessionResponse, error) {
	response, err := h.Service.GetCurrentSession(ctx, request.ID)
	if err != nil {
		return nil, err
	}

	return newSessionResponse(response), nil
}

func (h Handler) Delete(ctx context.Context, request GetSessionRequest) error {
	return h.Service.DeleteSessionForOrganization(ctx, request.ID, request.OrganizationID)
}

func (h Handler) Revoke(ctx context.Context, request GetSessionRequest) error {
	return h.Service.RevokeSessionForOrganization(ctx, request.ID, request.OrganizationID)
}

func (h Handler) RevokeOthers(ctx context.Context, request GetSessionRequest) (*RevokeOtherSessionsResponse, error) {
	result, err := h.Service.RevokeOtherSessions(ctx, request.ID)
	if err != nil {
		return nil, err
	}

	return &RevokeOtherSessionsResponse{
		Status:       "revoked",
		RevokedCount: result.RevokedCount,
	}, nil
}

// IsDuplicated checks if a user has another active session on a different device.
func (h Handler) IsDuplicated(ctx context.Context, userID string, currentSessionID string) (bool, error) {
	return h.Service.IsSessionDuplicated(ctx, userID, currentSessionID)
}

// Update updates mutable session fields.
func (h Handler) Update(ctx context.Context, organizationID, id string, request PatchSessionRequest) error {
	return h.Service.UpdateSessionForOrganization(ctx, request.toCommand(id), organizationID)
}

// ForceLogoutUserForOrganization revokes all sessions of a user inside the
// trusted tenant (administrative emergency). The target user is resolved
// through the tenant first, so a foreign user surfaces as 404 with no writes.
func (h Handler) ForceLogoutUserForOrganization(ctx context.Context, organizationID, userID string) (*ForceLogoutResponse, error) {
	result, err := h.Service.ForceLogoutUserForOrganization(ctx, organizationID, userID)
	if err != nil {
		return nil, err
	}
	return &ForceLogoutResponse{
		Status:       "revoked",
		RevokedCount: result.SessionCount,
		RefreshCount: result.RefreshCount,
		AccessCount:  result.AccessCount,
	}, nil
}

// ForceLogoutOrganization revokes all sessions in an organization.
func (h Handler) ForceLogoutOrganization(ctx context.Context, organizationID string) (*ForceLogoutResponse, error) {
	result, err := h.Service.ForceLogoutOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	return &ForceLogoutResponse{
		Status:       "revoked",
		RevokedCount: result.SessionCount,
		RefreshCount: result.RefreshCount,
		AccessCount:  result.AccessCount,
	}, nil
}
