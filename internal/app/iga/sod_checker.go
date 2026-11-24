package iga

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ErrSODConflict 表示检测到职责分离（Segregation of Duties）冲突。
// 调用方可通过 errors.Is(err, ErrSODConflict) 判断冲突类型。
var ErrSODConflict = errors.New("sod conflict")

// SODConflictError 携带触发冲突的 roleA / roleB，便于上层日志与提示。
type SODConflictError struct {
	RoleAID string
	RoleBID string
}

func (e *SODConflictError) Error() string {
	return fmt.Sprintf("%s: role %s conflicts with role %s", ErrSODConflict.Error(), e.RoleAID, e.RoleBID)
}

func (e *SODConflictError) Unwrap() error { return ErrSODConflict }

// SODChecker 执行 SoD 双时机校验：
//  1. 用户-角色时机（ValidateConflict）：在给某个用户授予新角色前，
//     校验该角色是否与用户当前持有的任何活跃角色存在 SoD 冲突。
//  2. 角色-角色时机（ValidateConflictRoles）：在某些不需要用户上下文的
//     场景，直接判定两个角色是否在 SODRules 表中被声明为冲突对。
type sodUoW interface {
	IGA() domain.Repository
}

// sodRulesByRoleLister 是可选的仓储能力：按角色集合单查询过滤规则
// （BE-D-17）。postgres 仓储实现；未实现时回退全量分页扫描。
type sodRulesByRoleLister interface {
	ListSODRulesByRoleIDs(ctx context.Context, orgID string, roleIDs []string) ([]*domain.SODRule, error)
}

type SODChecker struct {
	UOW   sodUoW
	Clock clock.Clock
}

// NewSODChecker 构造一个 SODChecker。
func NewSODChecker(uow sodUoW) SODChecker { return SODChecker{UOW: uow} }

func (c SODChecker) now() time.Time {
	if c.Clock != nil {
		return c.Clock.Now().UTC()
	}
	return time.Now().UTC()
}

const sodRulesPageSize = 100

func (c SODChecker) listAllRules(ctx context.Context, orgID string) ([]*domain.SODRule, error) {
	var all []*domain.SODRule
	for page := 1; ; page++ {
		rules, total, err := c.UOW.IGA().ListSODRules(ctx, ListSODRulesQuery{
			OrganizationID: orgID, Page: page, PageSize: sodRulesPageSize,
		})
		if err != nil {
			return nil, apperrors.WrapInternal(err, "sod: list rules")
		}
		all = append(all, rules...)
		if int64(len(all)) >= total {
			return all, nil
		}
		if len(rules) == 0 {
			return nil, apperrors.NewInternal("sod: rule pagination ended before the reported total")
		}
	}
}

// rulesForRoles 优先用角色过滤单查询；仓储不支持时回退全量扫描。
func (c SODChecker) rulesForRoles(ctx context.Context, orgID string, roleIDs []string) ([]*domain.SODRule, error) {
	if lister, ok := c.UOW.IGA().(sodRulesByRoleLister); ok {
		rules, err := lister.ListSODRulesByRoleIDs(ctx, orgID, roleIDs)
		if err != nil {
			return nil, apperrors.WrapInternal(err, "sod: list rules by roles")
		}
		return rules, nil
	}
	return c.listAllRules(ctx, orgID)
}

// ValidateConflict 校验：在给 user 授予 roleID 前，该角色是否与 user
// 当前活跃持有的任何角色构成 SoD 冲突。orgID + userID + roleID 三者都必填。
func (c SODChecker) ValidateConflict(ctx context.Context, orgID, roleID, userID string) error {
	if orgID == "" || roleID == "" || userID == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "sod: orgID, roleID and userID are all required")
	}

	// 1. 永久绑定与未过期 JIT grant 都计入当前有效角色。
	effectiveRoles, err := c.UOW.IGA().ListEffectiveRoleIDs(ctx, orgID, userID, c.now())
	if err != nil {
		return apperrors.WrapInternal(err, "sod: list effective roles")
	}
	if len(effectiveRoles) == 0 {
		// 用户未持有任何角色，不可能构成冲突对。
		return nil
	}

	// 2. 只拉取与候选角色/已持有角色相关的规则（单查询，BE-D-17）。
	candidates := append(append([]string{}, effectiveRoles...), roleID)
	rules, err := c.rulesForRoles(ctx, orgID, candidates)
	if err != nil {
		return err
	}

	// 3. 将用户已有角色放入 set，便于 O(1) 查找
	held := make(map[string]struct{}, len(effectiveRoles))
	for _, id := range effectiveRoles {
		held[id] = struct{}{}
	}

	// 4. 对每条规则（RoleA, RoleB）检查：
	//    - roleID == RoleA 且 user 持有 RoleB → 冲突
	//    - roleID == RoleB 且 user 持有 RoleA → 冲突
	for _, r := range rules {
		if (roleID == r.RoleAID) || (roleID == r.RoleBID) {
			var other string
			if roleID == r.RoleAID {
				other = r.RoleBID
			} else {
				other = r.RoleAID
			}
			if _, ok := held[other]; ok {
				return &SODConflictError{RoleAID: r.RoleAID, RoleBID: r.RoleBID}
			}
		}
	}
	return nil
}

// ValidateConflictRoles 校验两个角色在 SODRules 中是否被声明为冲突对
// （无序）。适用于创建 SODRule 或其他不涉及具体用户的双角色校验场景。
func (c SODChecker) ValidateConflictRoles(ctx context.Context, orgID, roleA, roleB string) error {
	if orgID == "" || roleA == "" || roleB == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "sod: orgID, roleA and roleB are all required")
	}
	rules, err := c.rulesForRoles(ctx, orgID, []string{roleA, roleB})
	if err != nil {
		return err
	}
	for _, r := range rules {
		if (r.RoleAID == roleA && r.RoleBID == roleB) || (r.RoleAID == roleB && r.RoleBID == roleA) {
			return &SODConflictError{RoleAID: roleA, RoleBID: roleB}
		}
	}
	return nil
}
