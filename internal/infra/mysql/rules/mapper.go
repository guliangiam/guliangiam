package rules

import (
	"encoding/json"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/rules"
)

// scanRule 扫描一行到 domain.Rule，处理 conditions JSON 反序列化。
func scanRule(sc interface{ Scan(...interface{}) error }) (*domain.Rule, error) {
	var r domain.Rule
	var conditions []byte
	err := sc.Scan(
		&r.ID, &r.OrganizationID, &r.Name, &r.Scope, &r.RuleType, &conditions,
		&r.Action, &r.Priority, &r.IsEnabled, &r.CreatedAt, &r.UpdatedAt, &r.Revision,
	)
	if err != nil {
		return nil, err
	}
	if len(conditions) > 0 && string(conditions) != "null" {
		if err := json.Unmarshal(conditions, &r.Conditions); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal conditions")
		}
	}
	if r.Conditions == nil {
		r.Conditions = []map[string]any{}
	}
	return &r, nil
}
