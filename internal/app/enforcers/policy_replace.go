package enforcers

import (
	"log/slog"
	"regexp"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// policyPTypePattern 接受 casbin 的 p/g 及其编号变体（p2、g2…）。
var policyPTypePattern = regexp.MustCompile(`^[pg][0-9]*$`)

// validatePolicyRows 在触碰现有策略之前校验全量导入行（BE-D-8）：
// 任何一行形状非法都直接 422，现有策略集保持原样。
func validatePolicyRows(policies [][]string) error {
	for i, row := range policies {
		if len(row) < 2 {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"policy row must contain pType and at least one value",
				map[string]any{"row": i + 1})
		}
		if !policyPTypePattern.MatchString(row[0]) {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"policy pType must be p or g (optionally numbered)",
				map[string]any{"row": i + 1, "pType": row[0]})
		}
	}
	return nil
}

// replacePolicies 原子化替换 enforcer 的策略集（BE-D-8）。
// casbin 写路径不在数据库事务内，无法依赖 UOW 回滚；这里采用
// “先全量校验 → 快照现有策略 → 替换 → 失败补偿恢复” 的语义：
// 任一 Add/Remove 中途失败都会尽力恢复快照，避免留下全空策略
// （授权面瘫痪）。补偿本身失败时记录错误日志并返回 Internal。
func (s Service) replacePolicies(enforcerID, modelText string, policies [][]string) error {
	if err := validatePolicyRows(policies); err != nil {
		return err
	}

	existing, err := s.CasbinManager.GetPolicies(enforcerID, modelText)
	if err != nil {
		return err
	}

	removed := make([][]string, 0, len(existing))
	for _, policy := range existing {
		if err := s.CasbinManager.RemovePolicy(enforcerID, modelText, policy); err != nil {
			s.restorePolicies(enforcerID, modelText, nil, removed)
			return apperrors.WrapInternal(err, "replace policies: remove existing failed")
		}
		removed = append(removed, policy)
	}

	added := make([][]string, 0, len(policies))
	for _, policy := range policies {
		if err := s.CasbinManager.AddPolicy(enforcerID, modelText, policy); err != nil {
			s.restorePolicies(enforcerID, modelText, added, existing)
			return apperrors.WrapInternal(err, "replace policies: add failed, snapshot restored")
		}
		added = append(added, policy)
	}
	return nil
}

// restorePolicies 尽力补偿：删除本次已加入的行，恢复被删除的快照行。
func (s Service) restorePolicies(enforcerID, modelText string, added, snapshot [][]string) {
	for _, policy := range added {
		if err := s.CasbinManager.RemovePolicy(enforcerID, modelText, policy); err != nil {
			slog.Error("enforcers: rollback remove added policy failed",
				"enforcer_id", enforcerID, "error", err)
		}
	}
	for _, policy := range snapshot {
		if err := s.CasbinManager.AddPolicy(enforcerID, modelText, policy); err != nil {
			slog.Error("enforcers: rollback restore policy failed",
				"enforcer_id", enforcerID, "error", err)
		}
	}
}
