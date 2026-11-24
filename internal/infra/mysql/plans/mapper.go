package plans

import (
	"encoding/json"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/plans"
)

// scanPlan 把 MySQL 表扫描结果转成 domain.Plan。
// 列顺序必须与 sql.go queryGetPlanByID / queryListPlans 的 SELECT 严格一致（16 列）。
// payment_providers JSON 列从 []byte 反序列化为 []string。
func scanPlan(sc interface{ Scan(...interface{}) error }) (*domain.Plan, error) {
	var p domain.Plan
	var providersJSON []byte
	err := sc.Scan(
		&p.ID, &p.OrganizationID, &p.Name, &p.DisplayName, &p.Description,
		&p.Price, &p.Currency, &p.Period, &p.Product,
		&providersJSON, &p.State, &p.IsExclusive, &p.Role,
		&p.CreatedAt, &p.UpdatedAt, &p.Revision,
	)
	if err != nil {
		return nil, err
	}
	if len(providersJSON) > 0 && string(providersJSON) != "null" {
		if err := json.Unmarshal(providersJSON, &p.PaymentProviders); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal payment_providers")
		}
	}
	if p.PaymentProviders == nil {
		p.PaymentProviders = []string{}
	}
	return &p, nil
}
