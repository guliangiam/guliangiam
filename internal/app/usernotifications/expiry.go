package usernotifications

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	tentxtx "github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// 到期提醒工作器的默认节拍与扫描窗口。
const (
	DefaultExpiryReminderInterval = 6 * time.Hour
	DefaultExpiryReminderLeadTime = 7 * 24 * time.Hour
	expiryScanBatchSize           = 200
)

// 到期提醒的收件箱去重来源前缀：同一实体只提醒一次，与 outbox 事件来源区分。
const (
	expiryInvitationSourceType        = "expiry_invitation"
	expiryCredentialSourceType        = "expiry_credential" //nolint:gosec // 来源类型前缀，非凭据
	expiryClientCertificateSourceType = "expiry_client_certificate"
)

// expiryReminderNamespace 为确定性收件箱 ID 提供固定命名空间；即使两个实例
// 同时扫描，同一 (实体, 管理员) 也只会落到同一个主键。
var expiryReminderNamespace = uuid.MustParse("b9d4c1a7-2f56-4c8e-9a31-7e5f0c2d6b48")

// ExpiryKind 是到期提醒的业务来源。
type ExpiryKind string

const (
	ExpiryKindInvitation        ExpiryKind = "invitation"
	ExpiryKindCredential        ExpiryKind = "credential"
	ExpiryKindClientCertificate ExpiryKind = "client_certificate"
)

// ExpiryReminder 是扫描器返回的一条候选：提醒哪个组织、哪条实体、实体名称与
// 到期时间。文案与链接由工作器按 Kind 统一渲染，保证与事件注册表一致。
type ExpiryReminder struct {
	Kind           ExpiryKind
	OrganizationID string
	SourceID       string
	EntityName     string
	ExpiresAt      time.Time
}

// ExpiryScanner 由组合根实现：在系统作用域内跨租户扫描到期窗口内的实体。
// 之所以是窄接口而不是直接依赖各域仓储，是为了让工作器可用假实现单测，
// 同时把 RLS 系统作用域的切换留在组合根。
type ExpiryScanner interface {
	ScanExpiring(ctx context.Context, now, deadline time.Time, limit int) ([]ExpiryReminder, error)
}

// reminderSink 是工作器需要的投递能力。生产实现包着 Service（偏好驱动的多渠道
// 扇出）；测试可注入假实现，避免为一次扫描构造完整 UnitOfWork。
type reminderSink interface {
	Administrators(ctx context.Context, organizationID string) ([]string, error)
	Exists(ctx context.Context, organizationID, userID, sourceType, sourceID string) bool
	Publish(ctx context.Context, command PublishCommand) error
}

// serviceSink 把收件箱 Service 适配成 reminderSink。
type serviceSink struct {
	service Service
}

func (s serviceSink) Administrators(ctx context.Context, organizationID string) ([]string, error) {
	return inboxAdminUserIDs(ctx, s.service.UOW, organizationID)
}

func (s serviceSink) Exists(ctx context.Context, organizationID, userID, sourceType, sourceID string) bool {
	return inboxNotificationExists(ctx, s.service.UOW, organizationID, userID, sourceType, sourceID)
}

func (s serviceSink) Publish(ctx context.Context, command PublishCommand) error {
	_, err := s.service.Publish(ctx, command)
	return err
}

// ExpiryReminderWorker 周期扫描即将到期的邀请、凭据与应用客户端证书，并把
// 提醒投递到组织管理员的个人收件箱（外部渠道由 Service.Publish 按偏好扇出）。
type ExpiryReminderWorker struct {
	Scanner   ExpiryScanner
	Sink      reminderSink
	Logger    *slog.Logger
	Interval  time.Duration
	LeadTime  time.Duration
	BatchSize int
	Now       func() time.Time
}

func NewExpiryReminderWorker(service Service, scanner ExpiryScanner, logger *slog.Logger) *ExpiryReminderWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &ExpiryReminderWorker{
		Scanner: scanner,
		Sink:    serviceSink{service: service},
		Logger:  logger.With("component", "expiry-reminder-worker"),
	}
}

// Run 阻塞运行直到 ctx 取消：启动后立即扫描一次，之后按 Interval 轮询。
func (w *ExpiryReminderWorker) Run(ctx context.Context) {
	interval := w.Interval
	if interval <= 0 {
		interval = DefaultExpiryReminderInterval
	}
	w.tickSafely(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tickSafely(ctx)
		}
	}
}

func (w *ExpiryReminderWorker) tickSafely(ctx context.Context) {
	if err := w.Tick(ctx); err != nil && ctx.Err() == nil {
		w.Logger.ErrorContext(ctx, "expiry reminder scan failed", "error", err)
	}
}

// Tick 执行一次扫描与投递。导出以便测试与运维手动触发。
func (w *ExpiryReminderWorker) Tick(ctx context.Context) error {
	if w.Scanner == nil || w.Sink == nil {
		return nil
	}
	now := w.now()
	leadTime := w.LeadTime
	if leadTime <= 0 {
		leadTime = DefaultExpiryReminderLeadTime
	}
	limit := w.BatchSize
	if limit <= 0 {
		limit = expiryScanBatchSize
	}

	// 扫描与后续写入都必须带系统作用域：收件箱表是 FORCE RLS。
	ctx = tentxtx.System(ctx)
	reminders, scanErr := w.Scanner.ScanExpiring(ctx, now, now.Add(leadTime), limit)
	if scanErr != nil {
		// 单一来源扫描失败不应吞掉其他来源的提醒：先投递已拿到的候选。
		w.Logger.ErrorContext(ctx, "expiry reminder scan partially failed", "error", scanErr)
	}
	if len(reminders) == 0 {
		return scanErr
	}

	grouped := make(map[string][]ExpiryReminder, 4)
	for _, reminder := range reminders {
		if reminder.OrganizationID == "" || reminder.SourceID == "" {
			continue
		}
		grouped[reminder.OrganizationID] = append(grouped[reminder.OrganizationID], reminder)
	}

	var firstErr error
	if scanErr != nil {
		firstErr = scanErr
	}
	for organizationID, items := range grouped {
		adminIDs, adminErr := w.Sink.Administrators(ctx, organizationID)
		if adminErr != nil {
			w.Logger.WarnContext(ctx, "expiry reminder: administrator lookup failed",
				"organization_id", organizationID, "error", adminErr)
			if firstErr == nil {
				firstErr = adminErr
			}
			continue
		}
		for _, reminder := range items {
			if pubErr := w.publishReminder(ctx, organizationID, adminIDs, reminder); pubErr != nil && firstErr == nil {
				firstErr = pubErr
			}
		}
	}
	return firstErr
}

func (w *ExpiryReminderWorker) publishReminder(ctx context.Context, organizationID string, adminIDs []string, reminder ExpiryReminder) error {
	title, content, link, sourceType, ok := renderExpiryReminder(reminder)
	if !ok {
		return nil
	}

	var firstErr error
	for _, userID := range adminIDs {
		if w.Sink.Exists(ctx, organizationID, userID, sourceType, reminder.SourceID) {
			continue
		}
		err := w.Sink.Publish(ctx, PublishCommand{
			NotificationID: uuid.NewSHA1(expiryReminderNamespace,
				[]byte(sourceType+":"+reminder.SourceID+":"+userID)).String(),
			OrganizationID: organizationID,
			UserID:         userID,
			Category:       string(expiryCategory(reminder.Kind)),
			EventType:      expiryEventType(reminder.Kind),
			Title:          title,
			Content:        content,
			Link:           link,
			SourceType:     sourceType,
			SourceID:       reminder.SourceID,
		})
		if err != nil {
			w.Logger.ErrorContext(ctx, "failed to publish expiry reminder",
				"organization_id", organizationID, "user_id", userID,
				"kind", reminder.Kind, "source_id", reminder.SourceID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (w *ExpiryReminderWorker) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now().UTC()
}

// renderExpiryReminder 生成与 contracts/registries/notification-events.yaml 中
// 同名事件一致的标题与正文；未知 Kind 返回 false（跳过而不是落一条空消息）。
func renderExpiryReminder(reminder ExpiryReminder) (title, content, link, sourceType string, ok bool) {
	expiresAt := reminder.ExpiresAt.Local().Format("2006-01-02 15:04")
	switch reminder.Kind {
	case ExpiryKindInvitation:
		return "邀请码即将过期",
			fmt.Sprintf("邀请码 %s 将于 %s 过期。", reminder.EntityName, expiresAt),
			"/invitations", expiryInvitationSourceType, true
	case ExpiryKindCredential:
		return "凭据即将过期",
			fmt.Sprintf("凭据 %s 将于 %s 过期，请及时轮换。", reminder.EntityName, expiresAt),
			"/certs", expiryCredentialSourceType, true
	case ExpiryKindClientCertificate:
		return "应用证书即将过期",
			fmt.Sprintf("应用 %s 的证书将于 %s 过期。", reminder.EntityName, expiresAt),
			"/applications", expiryClientCertificateSourceType, true
	default:
		return "", "", "", "", false
	}
}

func expiryEventType(kind ExpiryKind) string {
	switch kind {
	case ExpiryKindInvitation:
		return "invitation.expiring"
	case ExpiryKindCredential:
		return "credential.expiring"
	case ExpiryKindClientCertificate:
		return "application.client_certificate.expiring"
	default:
		return ""
	}
}

func expiryCategory(kind ExpiryKind) domain.Category {
	if kind == ExpiryKindCredential {
		return domain.CategorySecurity
	}
	return domain.CategoryBusiness
}

// inboxAdminUserIDs 解析组织的管理员收件人。调用方负责提供系统作用域 ctx。
func inboxAdminUserIDs(ctx context.Context, unitOfWork appuow.UserNotificationsUoW, organizationID string) ([]string, error) {
	var ids []string
	err := unitOfWork.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		isAdmin := true
		users, _, listErr := tx.Users().List(ctx, userdomain.ListQuery{
			OrganizationID: organizationID,
			IsAdmin:        &isAdmin,
			Page:           1,
			PageSize:       inboxAdminPageSize,
		})
		if listErr != nil {
			return listErr
		}
		for _, user := range users {
			if user.ID != "" {
				ids = append(ids, user.ID)
			}
		}
		return nil
	})
	return ids, err
}

// inboxNotificationExists 判断同一来源是否已经为该用户写过收件箱消息，
// 用于扫描器的跨轮次幂等。调用方负责提供系统作用域 ctx。
func inboxNotificationExists(ctx context.Context, unitOfWork appuow.UserNotificationsUoW, organizationID, userID, sourceType, sourceID string) bool {
	var exists bool
	err := withInbox(ctx, unitOfWork, func(ctx context.Context, repo domain.Repository) error {
		found, findErr := repo.ExistsBySource(ctx, organizationID, userID, sourceType, sourceID)
		if findErr != nil {
			return findErr
		}
		exists = found
		return nil
	})
	return err == nil && exists
}
