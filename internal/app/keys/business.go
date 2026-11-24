package keys

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// GetEnabledKeyByName retrieves an enabled key by organization and name for signing/verification.
//
// The repository's List query exposes only a fuzzy `Keyword` predicate, so an
// exact-name lookup cannot be delegated to SQL without widening the domain port
// (and its PostgreSQL/MySQL implementations). This method therefore pins the
// organization explicitly and scans the keyword matches for an exact name. Two
// behaviours matter for callers:
//
//   - An empty organization is rejected instead of turning the SQL predicate
//     into an unfiltered scan, which could select keys from other tenants.
//   - A keyword match whose name merely contains the requested name is not the
//     answer; that is the defect a prefix collision (`app-signing-app1` vs
//     `app-signing-app10`) exploited.
func (s Service) GetEnabledKeyByName(ctx context.Context, orgID, name string) (*KeyDetail, error) {
	orgID = strings.TrimSpace(orgID)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, apperrors.NewInvalidArgument("name is required")
	}
	if orgID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}

	enabled := true
	const pageSize = 100
	for page := 1; ; page++ {
		items, total, err := s.UOW.Keys().List(ctx, domain.ListQuery{
			Page: page, PageSize: pageSize, OrganizationID: orgID, Keyword: name, IsEnabled: &enabled,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item == nil || item.Name != name {
				continue
			}
			// Defense in depth: a UUID organization must appear on the row as
			// well. PostgreSQL RLS already filters, but the MySQL mirror has no
			// RLS, so a repository bug returning a wider page must not turn into
			// a cross-tenant key response.
			if platformuuid.IsValid(orgID) && item.OrganizationID != orgID {
				continue
			}
			d := toDetail(item)
			return &d, nil
		}
		if len(items) == 0 || int64(page*pageSize) >= total {
			break
		}
	}
	return nil, apperrors.NewNotFound("enabled key not found")
}

// DisableKey sets a key's IsEnabled flag to false.
func (s Service) DisableKey(ctx context.Context, organizationID, id string, revision int64) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		k, err := tx.Keys().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if revision > 0 && k.Revision != revision {
			return apperrors.NewConflict("key revision mismatch")
		}
		k.IsEnabled = false
		k.UpdatedAt = s.Clock.Now()
		k.Revision++

		if err := tx.Keys().Update(ctx, k); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			k.OrganizationID,
			"key.disabled",
			k.ID,
			"key.disabled",
			map[string]any{

				keyKeyID: k.ID,

				keyName: k.Name,

				keyOrgID: k.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
}
