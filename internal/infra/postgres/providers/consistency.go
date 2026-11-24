package providers

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	queryConsistencyRegistrations = `
SELECT p.id, p.organization_id, p.name, p.display_name,
       p.family IS NOT NULL AND p.kind IS NOT NULL AND p.variant IS NOT NULL AS classified,
       COALESCE(p.family, ''), COALESCE(p.kind, ''), COALESCE(p.variant, ''),
       COALESCE(c.config->>'status', ''), COALESCE(c.config->>'detailId', ''), p.enabled, p.deleted_at
FROM guliangiam.providers p
LEFT JOIN guliangiam.provider_configs c
  ON c.provider_id = p.id AND c.config_type = '_provider_registry_v1'
ORDER BY p.organization_id, p.id`

	queryConsistencyAI = `
SELECT id, COALESCE(provider_id::text, ''), organization_id, name, display_name,
       provider_type, status, deleted_at
FROM guliangiam.llm_providers
ORDER BY organization_id, id`

	queryConsistencyPayment = `
SELECT id, COALESCE(provider_id::text, ''), organization_id, channel, environment, status, deleted_at
FROM guliangiam.pay_channel_configs
ORDER BY organization_id, id`

	queryConsistencyNotification = `
SELECT id, COALESCE(provider_id::text, ''), organization_id, name, provider_type, is_enabled
FROM guliangiam.notification_providers
ORDER BY organization_id, id`
)

var _ domain.ConsistencyRepository = Repository{}

func (r Repository) ListConsistencyRegistrations(ctx context.Context) ([]domain.RegistrationConsistency, error) {
	rows, err := r.db.Query(ctx, queryConsistencyRegistrations)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.RegistrationConsistency, 0)
	for rows.Next() {
		var item domain.RegistrationConsistency
		var family, kind, variant, status string
		var deletedAt *time.Time
		if err := rows.Scan(
			&item.ID, &item.OrganizationID, &item.Name, &item.DisplayName, &item.Classified,
			&family, &kind, &variant, &status, &item.DetailID, &item.Enabled, &deletedAt,
		); err != nil {
			return nil, err
		}
		item.Family = domain.Family(family)
		item.Kind = domain.Kind(kind)
		item.Variant = domain.Variant(variant)
		item.Status = domain.Status(status)
		item.DeletedAt = deletedAt
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) ListConsistencyDetails(ctx context.Context, family domain.Family) ([]domain.DetailConsistency, error) {
	var query string
	switch family {
	case domain.FamilyIdentity:
		return []domain.DetailConsistency{}, nil
	case domain.FamilyAI:
		query = queryConsistencyAI
	case domain.FamilyPayment:
		query = queryConsistencyPayment
	case domain.FamilyNotification:
		query = queryConsistencyNotification
	default:
		return nil, apperrors.NewInvalidArgumentf("unsupported provider family: %s", family)
	}
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.DetailConsistency, 0)
	for rows.Next() {
		var item domain.DetailConsistency
		var deletedAt *time.Time
		switch family {
		case domain.FamilyAI:
			err = rows.Scan(&item.ID, &item.ProviderID, &item.OrganizationID, &item.Name, &item.DisplayName, &item.Kind, &item.Status, &deletedAt)
		case domain.FamilyPayment:
			err = rows.Scan(&item.ID, &item.ProviderID, &item.OrganizationID, &item.Kind, &item.Variant, &item.Status, &deletedAt)
		case domain.FamilyNotification:
			err = rows.Scan(&item.ID, &item.ProviderID, &item.OrganizationID, &item.Name, &item.Kind, &item.Enabled)
			if item.Enabled {
				item.Status = "active"
			} else {
				item.Status = "disabled"
			}
		case domain.FamilyIdentity:
			// identity 无 detail 一致性表：外层查询 switch 已以 "unsupported provider family" 拒绝。
		}
		if err != nil {
			return nil, err
		}
		item.DeletedAt = deletedAt
		items = append(items, item)
	}
	return items, rows.Err()
}
