package organizations

import (
	"context"
)

// DeploymentRegionDTO is the management projection of a deployment region.
type DeploymentRegionDTO struct {
	ID          string `json:"id"`
	RegionCode  string `json:"regionCode"`
	Name        string `json:"name"`
	CountryCode string `json:"countryCode"`
	Timezone    string `json:"timezone"`
	IsPrimary   bool   `json:"isPrimary"`
}

// ListDeploymentRegions returns the regions an organization can be assigned to.
func (s *Service) ListDeploymentRegions(ctx context.Context) ([]DeploymentRegionDTO, error) {
	if s.RegionLister == nil {
		return []DeploymentRegionDTO{}, nil
	}
	items, err := s.RegionLister.ListDeploymentRegions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]DeploymentRegionDTO, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, DeploymentRegionDTO{
			ID: item.ID, RegionCode: item.RegionCode, Name: item.Name,
			CountryCode: item.CountryCode, Timezone: item.Timezone, IsPrimary: item.IsPrimary,
		})
	}
	return out, nil
}
