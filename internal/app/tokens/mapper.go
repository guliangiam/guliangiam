package tokens

import domain "github.com/guliangiam/guliangiam/internal/domain/tokens"

func toListItem(token *domain.Token) TokenListItem {
	if token == nil {
		return TokenListItem{}
	}

	return TokenListItem{
		ID:               token.ID,
		UserID:           token.UserID,
		Username:         token.Username,
		OrganizationID:   token.OrganizationID,
		OrganizationSlug: token.OrganizationSlug,
		ApplicationID:    token.ApplicationID,
		ApplicationSlug:  token.ApplicationSlug,
		TokenPrefix:      token.TokenPrefix,
		TokenHashSet:     token.TokenHash != "",
		TokenHashMasked:  maskTokenHash(token.TokenHash),
		Scopes:           append([]string(nil), token.Scopes...),
		Audience:         append([]string(nil), token.Audience...),
		ExpiresAt:        token.ExpiresAt,
		RevokedAt:        token.RevokedAt,
		CreatedAt:        token.CreatedAt,
		UpdatedAt:        token.UpdatedAt,
	}
}

func toDetail(token *domain.Token) *TokenDetail {
	if token == nil {
		return nil
	}

	return &TokenDetail{
		TokenListItem: toListItem(token),
		Metadata:      cloneMap(token.Metadata),
		Revision:      token.Revision,
	}
}

// maskTokenHash 读接口不回显原始 token 哈希（BE-D-19）：无盐 SHA-256 与
// createdAt 组合会构成离线猜解 oracle，因此固定掩码，不保留任何原文片段。
func maskTokenHash(hash string) string {
	if hash == "" {
		return ""
	}
	return "******"
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
