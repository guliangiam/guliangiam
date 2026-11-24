package plans

import (
	"encoding/json"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/plans"
)

// scanPlan scans a full plan row into a domain Plan.
func scanPlan(sc interface{ Scan(...interface{}) error }) (*domain.Plan, error) {
	var p domain.Plan
	var providers []byte
	err := sc.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.DisplayName, &p.Description, &p.Price, &p.Currency, &p.Period, &p.Product, &providers, &p.State, &p.IsExclusive, &p.Role, &p.CreatedAt, &p.UpdatedAt, &p.Revision)
	if err != nil {
		return nil, err
	}
	if len(providers) > 0 && string(providers) != "null" {
		if err := json.Unmarshal(providers, &p.PaymentProviders); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal payment_providers")
		}
	}
	if p.PaymentProviders == nil {
		p.PaymentProviders = []string{}
	}
	return &p, nil
}
