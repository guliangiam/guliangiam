package tokens

import (
	"context"

	apptokens "github.com/guliangiam/guliangiam/internal/app/tokens"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service apptokens.Service
}

func (h Handler) List(ctx context.Context, request ListTokensRequest) (*ListTokensResponse, error) {
	items, total, err := h.Service.ListTokens(ctx, request.toQuery())
	if err != nil {
		return nil, err
	}

	return newListTokensResponse(items, presenter.PageMeta{
		Page:     request.Page,
		PageSize: request.PageSize,
		Total:    total,
	}), nil
}

func (h Handler) Create(ctx context.Context, request CreateTokenRequest) (*CreateTokenResponse, error) {
	result, err := h.Service.CreateToken(ctx, request.toCommand())
	if err != nil {
		return nil, err
	}

	return newCreateTokenResponse(result), nil
}

func (h Handler) RevokeMany(ctx context.Context, request RevokeTokensRequest) (*RevokeTokensResponse, error) {
	result, err := h.Service.RevokeTokens(ctx, request.toCommand())
	if err != nil {
		return nil, err
	}

	return &RevokeTokensResponse{Status: "revoked", RevokedCount: result.RevokedCount}, nil
}

func (h Handler) Introspect(ctx context.Context, request IntrospectTokenRequest) (*IntrospectTokenResponse, error) {
	result, err := h.Service.IntrospectToken(ctx, request.toCommand())
	if err != nil {
		return nil, err
	}

	return newIntrospectTokenResponse(result), nil
}

func (h Handler) Refresh(ctx context.Context, request RefreshTokenRequest) (*RefreshTokenResponse, error) {
	result, err := h.Service.RefreshAccessToken(ctx, request.toCommand())
	if err != nil {
		return nil, err
	}

	return newRefreshTokenResponse(result), nil
}

func (h Handler) Get(ctx context.Context, request GetTokenRequest) (*TokenResponse, error) {
	detail, err := h.Service.GetToken(ctx, apptokens.GetTokenQuery{
		ID:             request.ID,
		OrganizationID: request.OrganizationID,
	})
	if err != nil {
		return nil, err
	}

	return newTokenResponse(detail), nil
}

func (h Handler) Revoke(ctx context.Context, request GetTokenRequest) error {
	return h.Service.RevokeToken(ctx, apptokens.RevokeTokenCommand{
		ID:             request.ID,
		OrganizationID: request.OrganizationID,
	})
}

func (h Handler) Delete(ctx context.Context, request GetTokenRequest) error {
	return h.Service.DeleteToken(ctx, apptokens.DeleteTokenCommand{
		ID:             request.ID,
		OrganizationID: request.OrganizationID,
	})
}
