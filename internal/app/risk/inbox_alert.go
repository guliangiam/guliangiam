package risk

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	usernotificationdomain "github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

// riskAlertEventType 与 contracts/registries/notification-events.yaml 的
// risk.critical_alert.dispatched 对齐，通知中心据此做分类与筛选。
const riskAlertEventType = "risk.critical_alert.dispatched"

// inboxAlertUserIDs 解析需要收到站内告警的用户：被阻断的用户本人 + 组织管理员。
// 与 criticalAlertRecipients（邮箱口径）不同，这里返回用户 ID，用于写收件箱。
func (s *Service) inboxAlertUserIDs(ctx context.Context, tx appuow.UnitOfWork, event criticalAlertEvent) []string {
	seen := make(map[string]struct{}, criticalAlertMaxRecipients)
	ids := make([]string, 0, criticalAlertMaxRecipients)
	add := func(id string) {
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	add(event.UserID)

	isAdmin := true
	admins, _, err := tx.Users().List(ctx, userdomain.ListQuery{
		OrganizationID: event.OrganizationID,
		IsAdmin:        &isAdmin,
		Page:           1,
		PageSize:       criticalAlertAdminPageSize,
	})
	if err != nil {
		slog.WarnContext(ctx, "critical risk alert: inbox administrator lookup failed",
			"organization_id", event.OrganizationID, "error", err)
	} else {
		for _, admin := range admins {
			add(admin.ID)
		}
	}
	if len(ids) > criticalAlertMaxRecipients {
		ids = ids[:criticalAlertMaxRecipients]
	}
	return ids
}

// notifyCriticalAlertInbox 把风控告警同步写入收件箱。
//
// 这条路径与 notifyCriticalLoginDenied（外发台账）相互独立：即使组织没有配置
// 任何通知提供商、或没有可投递的邮箱，站内告警也必须出现在通知中心，否则
// "所有告警都同步到通知页面" 会随外部渠道缺失而断链。写入复用调用方事务，
// 不新开事务（MySQL UoW 不允许嵌套）。
func (s *Service) notifyCriticalAlertInbox(ctx context.Context, tx appuow.UnitOfWork, event criticalAlertEvent, now time.Time) {
	scoped, ok := tx.(appuow.UserNotificationsUoW)
	if !ok {
		return
	}
	userIDs := s.inboxAlertUserIDs(ctx, tx, event)
	if len(userIDs) == 0 {
		return
	}

	sourceID := event.DecisionID
	title := "安全告警：登录被风控阻断"
	content := fmt.Sprintf(
		"检测到一次被风控引擎阻断的登录尝试。\n渠道：%s，风险分：%d，时间：%s。\n如非本人操作，请在安全控制台核查该用户的会话与凭据状态。",
		event.Channel, event.Score, now.Format(time.RFC3339),
	)
	written := 0
	for _, userID := range userIDs {
		id, err := s.UUID.NewString()
		if err != nil {
			slog.ErrorContext(ctx, "critical risk alert: generate inbox id",
				"organization_id", event.OrganizationID, "error", err)
			continue
		}
		row := &usernotificationdomain.Notification{
			ID:             id,
			OrganizationID: event.OrganizationID,
			UserID:         userID,
			Category:       string(usernotificationdomain.CategorySecurity),
			EventType:      riskAlertEventType,
			Title:          title,
			Content:        content,
			Link:           "/risk",
			SourceType:     "risk_decision",
			SourceID:       &sourceID,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := scoped.UserNotifications().Create(ctx, row); err != nil {
			slog.ErrorContext(ctx, "critical risk alert: inbox write failed",
				"organization_id", event.OrganizationID, "user_id", userID, "error", err)
			continue
		}
		written++
	}
	slog.InfoContext(ctx, "critical risk alert written to inbox",
		"organization_id", event.OrganizationID, "decision_id", event.DecisionID,
		"recipients", len(userIDs), "written", written)
}
