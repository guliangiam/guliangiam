package invitations

import (
	"context"

	appinvitations "github.com/guliangiam/guliangiam/internal/app/invitations"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appinvitations.Service
}

func (h Handler) List(ctx context.Context, request ListInvitationsRequest) (ListInvitationsResponse, error) {
	items, total, err := h.Service.ListInvitations(ctx, appinvitations.ListInvitationsQuery(request))
	if err != nil {
		return ListInvitationsResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*InvitationResponse, error) {
	invitation, err := h.Service.GetInvitation(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	return toResponse(invitation), nil
}

func (h Handler) Create(ctx context.Context, request CreateInvitationRequest) (*CreateInvitationResponse, error) {
	invitation, err := h.Service.CreateInvitation(ctx, request.toCommand())
	if err != nil {
		return nil, err
	}

	return &CreateInvitationResponse{ID: invitation.ID, Status: "created"}, nil
}

func (h Handler) Revoke(ctx context.Context, request RevokeInvitationRequest) error {
	return h.Service.RevokeInvitation(ctx, appinvitations.RevokeInvitationCommand{
		ID:             request.ID,
		OrganizationID: request.OrganizationID,
	})
}

func (h Handler) Resend(ctx context.Context, request ResendInvitationRequest) error {
	return h.Service.ResendInvitation(ctx, appinvitations.ResendInvitationCommand{
		ID:             request.ID,
		OrganizationID: request.OrganizationID,
	})
}

// VerifyInvitation checks if an invitation code is valid and returns the invitation details.
func (h Handler) VerifyInvitation(ctx context.Context, code string) (*appinvitations.VerifyInvitationResult, error) {
	return h.Service.VerifyInvitation(ctx, code)
}

// SendInvitation sends an invitation email for the invitation ID resolved in
// the caller's trusted tenant.
func (h Handler) SendInvitation(ctx context.Context, organizationID, invitationID string) error {
	return h.Service.SendInvitation(ctx, organizationID, invitationID)
}
