package tokens

import apptokens "github.com/guliangiam/guliangiam/internal/app/tokens"

type ListTokensRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	UserID         string
	ApplicationID  string
	ActiveOnly     *bool
	Keyword        string
}

type GetTokenRequest struct {
	ID string
	// OrganizationID is filled from the authenticated tenant scope in chi.go.
	// It is not part of the wire contract.
	OrganizationID string
}

type CreateTokenRequest struct {
	OrganizationID          string         `json:"organizationId"`
	UserID                  string         `json:"userId"`
	ApplicationID           string         `json:"applicationId"`
	Scopes                  []string       `json:"scopes"`
	Audience                []string       `json:"audience,omitempty"`
	Metadata                map[string]any `json:"metadata,omitempty"`
	ExpiresInSeconds        int64          `json:"expiresInSeconds"`
	IssueRefreshToken       bool           `json:"issueRefreshToken,omitempty"`
	RefreshExpiresInSeconds int64          `json:"refreshExpiresInSeconds,omitempty"`
}

type RevokeTokensRequest struct {
	OrganizationID string `json:"organizationId,omitempty"`
	UserID         string `json:"userId,omitempty"`
	ApplicationID  string `json:"applicationId,omitempty"`
}

type IntrospectTokenRequest struct {
	Token         string `json:"token"`
	TokenTypeHint string `json:"tokenTypeHint,omitempty"`
	// OrganizationID is filled from the authenticated tenant scope in chi.go.
	// json:"-" keeps it out of the wire contract (OpenAPI has no such field).
	OrganizationID string `json:"-"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
	// OrganizationID is filled from the authenticated tenant scope in chi.go.
	OrganizationID string `json:"-"`
}

func (r ListTokensRequest) toQuery() apptokens.ListTokensQuery {
	return apptokens.ListTokensQuery{
		Page:           r.Page,
		PageSize:       r.PageSize,
		OrganizationID: r.OrganizationID,
		UserID:         r.UserID,
		ApplicationID:  r.ApplicationID,
		ActiveOnly:     r.ActiveOnly,
		Keyword:        r.Keyword,
	}
}

func (r CreateTokenRequest) toCommand() apptokens.CreateTokenCommand {
	return apptokens.CreateTokenCommand{
		OrganizationID:          r.OrganizationID,
		UserID:                  r.UserID,
		ApplicationID:           r.ApplicationID,
		Scopes:                  append([]string(nil), r.Scopes...),
		Audience:                append([]string(nil), r.Audience...),
		Metadata:                cloneMap(r.Metadata),
		ExpiresInSeconds:        r.ExpiresInSeconds,
		IssueRefreshToken:       r.IssueRefreshToken,
		RefreshExpiresInSeconds: r.RefreshExpiresInSeconds,
	}
}

func (r RevokeTokensRequest) toCommand() apptokens.RevokeTokensCommand {
	return apptokens.RevokeTokensCommand{
		OrganizationID: r.OrganizationID,
		UserID:         r.UserID,
		ApplicationID:  r.ApplicationID,
	}
}

func (r IntrospectTokenRequest) toCommand() apptokens.IntrospectTokenCommand {
	return apptokens.IntrospectTokenCommand{
		OrganizationID: r.OrganizationID,
		Token:          r.Token,
		TokenTypeHint:  r.TokenTypeHint,
	}
}

func (r RefreshTokenRequest) toCommand() apptokens.RefreshAccessTokenCommand {
	return apptokens.RefreshAccessTokenCommand{
		OrganizationID: r.OrganizationID,
		RefreshToken:   r.RefreshToken,
	}
}
