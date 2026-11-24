package tokens

import "time"

type TokenListItem struct {
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
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type TokenDetail struct {
	TokenListItem
	Metadata map[string]any `json:"metadata"`
	Revision int64          `json:"revision"`
}

type CreateTokenIssued struct {
	TokenDetail
	TokenValue   string `json:"tokenValue"`
	RefreshToken string `json:"refreshToken"`
}

type RefreshTokenIssued struct {
	TokenDetail
	TokenValue   string `json:"tokenValue"`
	RefreshToken string `json:"refreshToken"`
}
