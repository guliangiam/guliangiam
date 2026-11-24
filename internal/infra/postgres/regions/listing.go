package regions

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/regions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var _ domain.DeploymentRegionLister = Repository{}

// ListDeploymentRegions returns the registered deployment regions, primary first.
func (r Repository) ListDeploymentRegions(ctx context.Context) ([]*domain.DeploymentRegion, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, region_code, name, country_code, timezone, is_primary, rto_minutes, rpo_minutes, created_at, updated_at
		FROM guliangiam.deployment_regions
		ORDER BY is_primary DESC, region_code ASC`)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list deployment regions")
	}
	defer rows.Close()
	regions := make([]*domain.DeploymentRegion, 0)
	for rows.Next() {
		var region domain.DeploymentRegion
		if err := rows.Scan(&region.ID, &region.RegionCode, &region.Name, &region.CountryCode, &region.Timezone,
			&region.IsPrimary, &region.RTOMinutes, &region.RPOMinutes, &region.CreatedAt, &region.UpdatedAt); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan deployment region")
		}
		regions = append(regions, &region)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "iterate deployment regions")
	}
	return regions, nil
}
