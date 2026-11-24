package providers

import (
	"context"

	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appproviders.Service
}

func (h Handler) List(ctx context.Context, request ListProvidersRequest) (ListProvidersResponse, error) {
	items, total, err := h.Service.ListProviders(ctx, appproviders.ListProvidersQuery{
		Page:           request.Page,
		PageSize:       request.PageSize,
		OrganizationID: request.OrganizationID,
		Keyword:        request.Keyword,
		Category:       request.Category,
		ProviderType:   request.ProviderType,
	})
	if err != nil {
		return ListProvidersResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

// The by-ID methods receive the trusted organization resolved by
// trustedOrgID; the app layer compares it with the persisted row.

func (h Handler) Get(ctx context.Context, id, organizationID string) (*ProviderResponse, error) {
	response, err := h.Service.GetProvider(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	typed := ProviderResponse(*response)
	return &typed, nil
}

// operatorID returns the authenticated session's user id, or "" for system
// paths that carry no session. Callers write it into the command so the audit
// provider can attribute the write to the operator.
func operatorID(ctx context.Context) string {
	if session, ok := authtransport.SessionContextFromContext(ctx); ok {
		return session.UserID
	}
	return ""
}

func (h Handler) Create(ctx context.Context, request CreateProviderRequest) (string, error) {
	command := request.toCommand()
	command.ActorID = operatorID(ctx)
	provider, err := h.Service.CreateProvider(ctx, command)
	if err != nil {
		return "", err
	}
	return provider.Registration.ID, nil
}

func (h Handler) Patch(ctx context.Context, id, organizationID string, request PatchProviderRequest) error {
	command := request.toCommand(id)
	command.OrganizationID = organizationID
	command.ActorID = operatorID(ctx)
	return h.Service.PatchProvider(ctx, command)
}

func (h Handler) Delete(ctx context.Context, id, organizationID string) error {
	return h.Service.DeleteProvider(ctx, id, organizationID, operatorID(ctx))
}

func (h Handler) GetConfig(ctx context.Context, id, organizationID string) (*ProviderConfigResponse, error) {
	response, err := h.Service.GetProviderConfig(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	typed := ProviderConfigResponse(*response)
	return &typed, nil
}

func (h Handler) UpdateConfig(ctx context.Context, id, organizationID string, request UpdateProviderConfigRequest) error {
	command := request.toCommand(id)
	command.OrganizationID = organizationID
	command.ActorID = operatorID(ctx)
	return h.Service.UpdateProviderConfig(ctx, command)
}

func (h Handler) ListSecretStatuses(ctx context.Context, id, organizationID string) (*ProviderSecretStatusListResponse, error) {
	items, err := h.Service.ListProviderSecretStatuses(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	return &ProviderSecretStatusListResponse{Items: items}, nil
}

func (h Handler) TestConnection(ctx context.Context, id, organizationID string) (*TestConnectionResponse, error) {
	result, err := h.Service.TestConnection(ctx, id, organizationID, operatorID(ctx))
	if err != nil {
		return nil, err
	}

	return &TestConnectionResponse{Connected: result.Connected, Error: result.Error}, nil
}

// Enable activates a provider after an explicit operator decision. It is
// separate from TestConnection so probing stays read-only.
func (h Handler) Enable(ctx context.Context, id, organizationID string) error {
	_, err := h.Service.EnableProvider(ctx, id, organizationID, operatorID(ctx))
	return err
}
