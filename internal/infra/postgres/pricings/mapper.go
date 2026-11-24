package pricings

import (
	"encoding/json"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/pricings"
)

// scanPricing scans a full pricing row into a domain Pricing.
func scanPricing(sc interface{ Scan(...interface{}) error }) (*domain.Pricing, error) {
	var p domain.Pricing
	var plans []byte
	err := sc.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.DisplayName, &p.Description, &plans, &p.IsEnabled, &p.TrialDuration, &p.Application, &p.CreatedAt, &p.UpdatedAt, &p.Revision)
	if err != nil {
		return nil, err
	}
	if len(plans) > 0 && string(plans) != "null" {
		if err := json.Unmarshal(plans, &p.Plans); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal plans")
		}
	}
	if p.Plans == nil {
		p.Plans = []string{}
	}
	return &p, nil
}
