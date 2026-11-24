package tokens

import (
	"time"

	apptokens "github.com/guliangiam/guliangiam/internal/app/tokens"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type TokenListItemResponse struct {
	ID               string     `json:"id"`
	UserID           string     `json:"userId"`
	Username         string     `json:"username"`
	OrganizationID   string     `json:"organizationId"`
	OrganizationSlug string     `json:"organizationSlug"`
	ApplicationID    string     `json:"applicationId"`
	ApplicationSlug  string     `json:"applicationSlug"`
	TokenPrefix      string     `json:"tokenPrefix"`
	TokenHashSet     bool       `json:"tokenHashSet"`
	TokenHashMasked  string     `json:"tokenHashMasked"`
	Scopes           []string   `json:"scopes"`
	Audience         []string   `json:"audience"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	RevokedAt        *time.Time `json:"revokedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type ListTokensResponse struct {
	Items []TokenListItemResponse `json:"items"`
	Page  presenter.PageMeta      `json:"page"`
}

type TokenResponse struct {
	TokenListItemResponse
	Metadata map[string]any `json:"metadata"`
	Revision int64          `json:"revision"`
}

type CreateTokenResponse struct {
	Status       string        `json:"status"`
	TokenValue   string        `json:"tokenValue"`
	RefreshToken string        `json:"refreshToken,omitempty"`
	Token        TokenResponse `json:"token"`
}

type RevokeTokensResponse struct {
	Status       string `json:"status"`
	RevokedCount int64  `json:"revokedCount"`
}

type IntrospectTokenResponse struct {
	Active    bool     `json:"active"`
	Scope     string   `json:"scope,omitempty"`
	Username  string   `json:"username,omitempty"`
	TokenType string   `json:"tokenType,omitempty"`
	Exp       int64    `json:"exp,omitempty"`
	Iat       int64    `json:"iat,omitempty"`
	Sub       string   `json:"sub,omitempty"`
	Aud       []string `json:"aud,omitempty"`
	Jti       string   `json:"jti,omitempty"`
}

type RefreshTokenResponse struct {
	Status       string        `json:"status"`
	TokenValue   string        `json:"tokenValue"`
	RefreshToken string        `json:"refreshToken"`
	Token        TokenResponse `json:"token"`
}

func newListTokensResponse(items []apptokens.TokenListItem, page presenter.PageMeta) *ListTokensResponse {
	responseItems := make([]TokenListItemResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, toTokenListItemResponse(item))
	}

	return &ListTokensResponse{Items: responseItems, Page: page}
}

func newTokenResponse(detail *apptokens.TokenDetail) *TokenResponse {
	if detail == nil {
		return nil
	}

	return &TokenResponse{
		TokenListItemResponse: toTokenListItemResponse(detail.TokenListItem),
		Metadata:              cloneMap(detail.Metadata),
		Revision:              detail.Revision,
	}
}

func newCreateTokenResponse(result *apptokens.CreateTokenIssued) *CreateTokenResponse {
	if result == nil {
		return nil
	}

	token := newTokenResponse(&result.TokenDetail)
	if token == nil {
		return nil
	}

	return &CreateTokenResponse{
		Status:       "created",
		TokenValue:   result.TokenValue,
		RefreshToken: result.RefreshToken,
		Token:        *token,
	}
}

func newRefreshTokenResponse(result *apptokens.RefreshTokenIssued) *RefreshTokenResponse {
	if result == nil {
		return nil
	}

	token := newTokenResponse(&result.TokenDetail)
	if token == nil {
		return nil
	}

	return &RefreshTokenResponse{
		Status:       "refreshed",
		TokenValue:   result.TokenValue,
		RefreshToken: result.RefreshToken,
		Token:        *token,
	}
}

func newIntrospectTokenResponse(result *apptokens.Introspection) *IntrospectTokenResponse {
	if result == nil {
		return nil
	}

	return &IntrospectTokenResponse{
		Active:    result.Active,
		Scope:     result.Scope,
		Username:  result.Username,
		TokenType: result.TokenType,
		Exp:       result.Exp,
		Iat:       result.Iat,
		Sub:       result.Sub,
		Aud:       append([]string(nil), result.Aud...),
		Jti:       result.Jti,
	}
}

func toTokenListItemResponse(item apptokens.TokenListItem) TokenListItemResponse {
	return TokenListItemResponse{
		ID:               item.ID,
		UserID:           item.UserID,
		Username:         item.Username,
		OrganizationID:   item.OrganizationID,
		OrganizationSlug: item.OrganizationSlug,
		ApplicationID:    item.ApplicationID,
		ApplicationSlug:  item.ApplicationSlug,
		TokenPrefix:      item.TokenPrefix,
		TokenHashSet:     item.TokenHashSet,
		TokenHashMasked:  item.TokenHashMasked,
		Scopes:           append([]string(nil), item.Scopes...),
		Audience:         append([]string(nil), item.Audience...),
		ExpiresAt:        item.ExpiresAt,
		RevokedAt:        item.RevokedAt,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func cloneMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}
