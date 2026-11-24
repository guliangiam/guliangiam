package applications

import (
	"context"
	"strings"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
)

// ValidateSCIMBearerToken validates a SCIM bearer token against the application's
// client credentials in the specified organization. It returns true if the token
// matches any application's client secret.
func (s *Service) ValidateSCIMBearerToken(ctx context.Context, orgID, token string) (bool, error) {
	if strings.TrimSpace(orgID) == "" || strings.TrimSpace(token) == "" {
		return false, nil
	}

	// List all applications for this organization. 单页 PageSize:100 会让
	// 第 2 页起的应用合法 token 恒 401（INC-BE-02）：按 total 翻完全部页，
	// 与 ResolveOIDCApplication 的分页范式一致。
	repo := s.UOW.Applications()
	const pageSize = 100
	now := time.Now().UTC()
	for page := 1; ; page++ {
		apps, total, err := repo.List(ctx, domain.ListQuery{
			OrganizationID: orgID,
			Page:           page,
			PageSize:       pageSize,
		})
		if err != nil {
			return false, err
		}

		// Check each application's usable client secrets (active + grace), with a
		// read-only fallback to the legacy hash column for pre-migration rows.
		for _, app := range apps {
			secrets, err := repo.ListUsableClientSecrets(ctx, app.OrganizationID, app.ID, now)
			if err != nil {
				return false, err
			}
			for _, secret := range secrets {
				match, verifyErr := platformcrypto.VerifyClientSecret(token, secret.SecretHash)
				if verifyErr == nil && match {
					return true, nil
				}
			}
			if app.ClientSecretHash == "" {
				continue
			}
			match, verifyErr := platformcrypto.VerifyClientSecret(token, app.ClientSecretHash)
			if verifyErr == nil && match {
				return true, nil
			}
		}

		if len(apps) == 0 || int64(page*pageSize) >= total {
			break
		}
	}

	return false, nil
}
