package rules

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func validateCreateRuleCommand(command CreateRuleCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if strings.TrimSpace(command.Scope) == "" {
		return apperrors.NewInvalidArgument("scope is required")
	}
	if strings.TrimSpace(command.RuleType) == "" {
		return apperrors.NewInvalidArgument("ruleType is required")
	}
	if strings.TrimSpace(command.Action) == "" {
		return apperrors.NewInvalidArgument("action is required")
	}
	return validateRuleConditions(command.RuleType, command.Conditions)
}

// validateRuleConditions 把写路径接入 RuleValidator（BE-D-22）：此前任意
// JSON conditions 都能落库，WAF/IP/限流引擎只能静默跳过非法行。
func validateRuleConditions(ruleType string, conditions []map[string]any) error {
	result := RuleValidator{}.Validate(ruleType, conditionExpressions(ruleType, conditions))
	if !result.Valid {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid rule conditions",
			map[string]any{"errors": result.Errors})
	}
	return nil
}

// conditionExpressions 从通用 condition 行提取各类型校验器需要的表达式：
// WAF 取 pattern；其余类型取 value（IP range 拆成两端分别校验）。
func conditionExpressions(ruleType string, conditions []map[string]any) []string {
	isWAF := strings.EqualFold(ruleType, "waf")
	isIP := strings.EqualFold(ruleType, "ip")
	var out []string
	for _, cond := range conditions {
		if isWAF {
			if p, ok := cond["pattern"].(string); ok && strings.TrimSpace(p) != "" {
				out = append(out, p)
			}
			continue
		}
		v, _ := cond["value"].(string)
		if strings.TrimSpace(v) == "" {
			continue
		}
		if isIP && cond["operator"] == "range" {
			lo, hi, ok := strings.Cut(v, "-")
			if ok && strings.TrimSpace(lo) != "" && strings.TrimSpace(hi) != "" {
				out = append(out, strings.TrimSpace(lo), strings.TrimSpace(hi))
				continue
			}
		}
		out = append(out, v)
	}
	return out
}

func validatePatchRuleCommand(command PatchRuleCommand) error {
	if command.Name != nil && strings.TrimSpace(*command.Name) == "" {
		return apperrors.NewInvalidArgument("name must not be blank")
	}
	if command.Scope != nil && strings.TrimSpace(*command.Scope) == "" {
		return apperrors.NewInvalidArgument("scope must not be blank")
	}
	if command.RuleType != nil && strings.TrimSpace(*command.RuleType) == "" {
		return apperrors.NewInvalidArgument("ruleType must not be blank")
	}
	if command.Action != nil && strings.TrimSpace(*command.Action) == "" {
		return apperrors.NewInvalidArgument("action must not be blank")
	}
	return nil
}
