package consents

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/consents"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func toConsent(row consentRow) (*domain.Consent, error) {
	consent := &domain.Consent{
		ID:               row.ID,
		UserID:           row.UserID,
		Username:         row.Username,
		OrganizationID:   row.OrganizationID,
		OrganizationSlug: row.OrganizationSlug,
		ApplicationID:    row.ApplicationID,
		ApplicationSlug:  row.ApplicationSlug,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		RevokedAt:        row.RevokedAt,
		Revision:         row.Revision,
	}
	if err := shared.UnmarshalJSON(row.GrantedScopes, &consent.GrantedScopes); err != nil {
		return nil, err
	}
	return consent, nil
}
