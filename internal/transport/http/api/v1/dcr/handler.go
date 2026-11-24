package dcr

import (
	"context"
	"encoding/json"

	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
)

type Handler struct {
	Service appapplications.Service
}

func NewHandler(service appapplications.Service) Handler {
	return Handler{Service: service}
}

type RegisterClientRequest struct {
	ClientName              string          `json:"client_name"`
	RedirectURIs            []string        `json:"redirect_uris"`
	GrantTypes              []string        `json:"grant_types"`
	ResponseTypes           []string        `json:"response_types"`
	TokenEndpointAuthMethod string          `json:"token_endpoint_auth_method"`
	Scope                   string          `json:"scope"`
	ClientURI               string          `json:"client_uri"`
	LogoURI                 string          `json:"logo_uri"`
	OrganizationSlug        string          `json:"organization_slug"`
	JWKS                    json.RawMessage `json:"jwks"`
	// JWKSURI 不被支持，但必须在传输层解析后交给服务层返回 RFC 7591 的
	// invalid_client_metadata；否则 DisallowUnknownFields 会把它降级成普通 400。
	JWKSURI string `json:"jwks_uri"`
}

type RegisterClientResponse = appapplications.DCRResponse

func (h Handler) RegisterClient(ctx context.Context, request RegisterClientRequest) (*RegisterClientResponse, error) {
	return h.Service.RegisterDynamicClient(ctx, appapplications.DCRRequest{
		ClientName:              request.ClientName,
		RedirectURIs:            request.RedirectURIs,
		GrantTypes:              request.GrantTypes,
		ResponseTypes:           request.ResponseTypes,
		TokenEndpointAuthMethod: request.TokenEndpointAuthMethod,
		Scope:                   request.Scope,
		ClientURI:               request.ClientURI,
		LogoURI:                 request.LogoURI,
		OrganizationSlug:        request.OrganizationSlug,
		JWKS:                    request.JWKS,
		JWKSURI:                 request.JWKSURI,
	})
}

type GetClientRequest struct {
	ClientID                string
	RegistrationAccessToken string
}

type GetClientResponse = appapplications.DCRResponse

func (h Handler) GetClient(ctx context.Context, request GetClientRequest) (*GetClientResponse, error) {
	return h.Service.GetDynamicClient(ctx, request.ClientID, request.RegistrationAccessToken)
}

type UpdateClientRequest struct {
	ClientID                string          `json:"-"`
	ClientName              string          `json:"client_name"`
	RedirectURIs            []string        `json:"redirect_uris"`
	GrantTypes              []string        `json:"grant_types"`
	ResponseTypes           []string        `json:"response_types"`
	TokenEndpointAuthMethod string          `json:"token_endpoint_auth_method"`
	Scope                   string          `json:"scope"`
	ClientURI               string          `json:"client_uri"`
	LogoURI                 string          `json:"logo_uri"`
	JWKS                    json.RawMessage `json:"jwks"`
	JWKSURI                 string          `json:"jwks_uri"`
	// RegistrationAccessToken is accepted in the body as an alternative to the
	// Authorization: Bearer header (RFC 7591 §2.3 prefers the header).
	RegistrationAccessToken string `json:"registration_access_token,omitempty"`
}

type UpdateClientResponse = appapplications.DCRResponse

func (h Handler) UpdateClient(ctx context.Context, clientID string, registrationAccessToken string, request UpdateClientRequest) (*UpdateClientResponse, error) {
	return h.Service.UpdateDynamicClient(ctx, clientID, registrationAccessToken, appapplications.DCRRequest{
		ClientName:              request.ClientName,
		RedirectURIs:            request.RedirectURIs,
		GrantTypes:              request.GrantTypes,
		ResponseTypes:           request.ResponseTypes,
		TokenEndpointAuthMethod: request.TokenEndpointAuthMethod,
		Scope:                   request.Scope,
		ClientURI:               request.ClientURI,
		LogoURI:                 request.LogoURI,
		JWKS:                    request.JWKS,
		JWKSURI:                 request.JWKSURI,
	})
}

type DeleteClientRequest struct {
	ClientID                string
	RegistrationAccessToken string
}

func (h Handler) DeleteClient(ctx context.Context, request DeleteClientRequest) error {
	return h.Service.DeleteDynamicClient(ctx, request.ClientID, request.RegistrationAccessToken)
}
