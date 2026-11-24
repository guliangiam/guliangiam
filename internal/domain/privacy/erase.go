package privacy

import (
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// EraseTarget 是请求擦除的一类数据。
type EraseTarget struct {
	Category string `json:"category"`
	TargetID string `json:"targetId"`
}

// RetentionItem 是留存义务：法律依据 + 是否阻断擦除（法律留存/审计链）。
type RetentionItem struct {
	Category   string `json:"category"`
	LegalBasis string `json:"legalBasis"`
	Blocks     bool   `json:"blocks"`
}

// EraseScope 是擦除范围清单：实际擦除项 + 逐项保留项与法律依据。
type EraseScope struct {
	Erase  []EraseTarget   `json:"erase"`
	Retain []RetentionItem `json:"retain"`
}

// PlanErase 计算擦除范围（design/privacy §4 双轨裁决）：
//   - 命中留存义务的数据不擦除，逐项列入 Retain 并附法律依据；
//   - 任一留存义务 Blocks=true（法律留存/审计链）→ 409 conflict，附保留依据；
//   - 其余目标进入 Erase；无可擦除项时返回 409，避免空操作被当作已完成。
func PlanErase(targets []EraseTarget, retentions []RetentionItem) (EraseScope, error) {
	if len(targets) == 0 {
		return EraseScope{}, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"at least one erase target is required",
			map[string]any{"field": "targets"})
	}
	retained := make(map[string]RetentionItem, len(retentions))
	for _, retention := range retentions {
		if retention.Category == "" || retention.LegalBasis == "" {
			return EraseScope{}, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"retention category and legalBasis are required",
				map[string]any{"field": "retentions"})
		}
		retained[retention.Category] = retention
	}
	scope := EraseScope{Erase: make([]EraseTarget, 0, len(targets)), Retain: make([]RetentionItem, 0, len(retentions))}
	seenRetain := make(map[string]bool, len(retentions))
	for _, target := range targets {
		if target.Category == "" || target.TargetID == "" {
			return EraseScope{}, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"erase target category and targetId are required",
				map[string]any{"field": "targets"})
		}
		if retention, ok := retained[target.Category]; ok {
			if retention.Blocks {
				return EraseScope{}, apperrors.NewWithMeta(apperrors.CodeConflict,
					"erasure is blocked by a legal retention obligation",
					map[string]any{"field": "category", "category": target.Category, "legalBasis": retention.LegalBasis})
			}
			if !seenRetain[retention.Category] {
				scope.Retain = append(scope.Retain, retention)
				seenRetain[retention.Category] = true
			}
			continue
		}
		scope.Erase = append(scope.Erase, target)
	}
	if len(scope.Erase) == 0 {
		return EraseScope{}, apperrors.NewWithMeta(apperrors.CodeConflict,
			"no erasable data remains after retention obligations",
			map[string]any{"field": "targets"})
	}
	return scope, nil
}
