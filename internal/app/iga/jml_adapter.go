package iga

import (
	"context"
	"time"

	"github.com/google/uuid"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// JMLAdapter 把 IGA 的 SoD 校验与 campaign 能力适配成 app/jml 需要的
// IGARoleService 子集（ADR-030 Joiner/Mover）。适配器绑定调用方传入的事务
// UoW：SoD 读取与 campaign 写入都发生在 JML 编排的同一事务内。
type JMLAdapter struct {
	UOW   appuow.UnitOfWork
	Clock clock.Clock
}

// NewJMLAdapter 构造事务绑定的 JML 适配器。tx 必须是 WithinTransaction
// 回调里拿到的事务 UoW，不能传组合根 UoW（否则 campaign 会脱离 JML 事务）。
func NewJMLAdapter(tx appuow.UnitOfWork, clk clock.Clock) JMLAdapter {
	return JMLAdapter{UOW: tx, Clock: clk}
}

func (a JMLAdapter) now() time.Time {
	if a.Clock != nil {
		return a.Clock.Now().UTC()
	}
	return time.Now().UTC()
}

// CheckSoD 判定 candidateRoleID 是否与 existingRoleIDs 中任一角色构成
// SoD 冲突对（无序）。返回冲突规则；nil 表示无冲突。
func (a JMLAdapter) CheckSoD(ctx context.Context, orgID, candidateRoleID string, existingRoleIDs []string) (*domain.SODRule, error) {
	if orgID == "" || candidateRoleID == "" {
		return nil, apperrors.NewInvalidArgument("sod: orgID and candidateRoleID are required")
	}
	if len(existingRoleIDs) == 0 {
		return nil, nil
	}
	checker := SODChecker{UOW: a.UOW, Clock: a.Clock}
	candidates := append(append([]string{}, existingRoleIDs...), candidateRoleID)
	rules, err := checker.rulesForRoles(ctx, orgID, candidates)
	if err != nil {
		return nil, err
	}
	held := make(map[string]struct{}, len(existingRoleIDs))
	for _, id := range existingRoleIDs {
		held[id] = struct{}{}
	}
	for _, r := range rules {
		var other string
		switch candidateRoleID {
		case r.RoleAID:
			other = r.RoleBID
		case r.RoleBID:
			other = r.RoleAID
		default:
			continue
		}
		if _, ok := held[other]; ok {
			return r, nil
		}
	}
	return nil, nil
}

// CreateAutoCampaign 为 Joiner 自动创建一条已通过 SoD 的 campaign。
// 状态枚举没有 auto-approved（ck_access_campaigns_status），落 completed
// 终态表达“自动批准”；created_by 无实体列，写进审计 payload。
func (a JMLAdapter) CreateAutoCampaign(ctx context.Context, orgID, userID string, initialRoleIDs []string, reason string) error {
	if orgID == "" || userID == "" {
		return apperrors.NewInvalidArgument("campaign: orgID and userID are required")
	}
	now := a.now()
	end := now.Add(365 * 24 * time.Hour)
	c := &domain.AccessCampaign{
		ID:             uuid.NewString(),
		OrganizationID: orgID,
		Name:           "jml-auto:" + reason + ":" + userID,
		Status:         domain.CampaignCompleted,
		StartDate:      &now,
		EndDate:        &end,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	if err := a.UOW.IGA().CreateCampaign(ctx, c); err != nil {
		return apperrors.WrapInternal(err, "jml: create auto campaign")
	}
	return audit.EmbedAuditEvent(
		a.Clock, a.UOW, ctx,
		"", "", "", "", "",
		orgID, "", "iga.campaign.created", "access_campaign", c.ID,
		"access_campaign", "iga.campaign.created",
		map[string]any{
			auditKeyCampaignID: c.ID,
			auditKeyName:       c.Name,
			"created_by":       "jml-system",
			"auto_approved":    true,
			"user_id":          userID,
			"reason":           reason,
			"role_ids":         initialRoleIDs,
		},
	)
}
