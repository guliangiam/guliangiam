// wire_expiry_reminders.go — 到期提醒工作器：把邀请、凭据与应用客户端证书的
// 临期数据转成个人收件箱消息（外部渠道由 Service.Publish 按用户偏好扇出）。
package bootstrap

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appusernotifications "github.com/guliangiam/guliangiam/internal/app/usernotifications"
	applicationsdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	certsdomain "github.com/guliangiam/guliangiam/internal/domain/certs"
	invitationsdomain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// expiryScanner 在系统作用域事务内跨租户扫描到期窗口。三个域仓储的可选扫描
// 能力由类型断言探测：方言镜像未实现时只是少一类提醒，不会阻断启动。
type expiryScanner struct {
	uow appuow.UnitOfWork
}

func (s expiryScanner) ScanExpiring(ctx context.Context, now, deadline time.Time, limit int) ([]appusernotifications.ExpiryReminder, error) {
	reminders := make([]appusernotifications.ExpiryReminder, 0, limit)
	var firstErr error
	err := s.uow.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if scanner, ok := tx.Invitations().(invitationsdomain.ExpiringScanner); ok {
			items, listErr := scanner.ListExpiring(ctx, now, deadline, limit)
			if listErr != nil {
				// 单一来源失败不阻断其余来源；错误在事务外统一返回。
				firstErr = listErr
			} else {
				for _, item := range items {
					reminders = append(reminders, appusernotifications.ExpiryReminder{
						Kind:           appusernotifications.ExpiryKindInvitation,
						OrganizationID: item.OrganizationID,
						SourceID:       item.ID,
						EntityName:     item.Code,
						ExpiresAt:      item.ExpiresAt,
					})
				}
			}
		}
		if scanner, ok := tx.Certs().(certsdomain.ExpiringScanner); ok {
			items, listErr := scanner.ListExpiring(ctx, now, deadline, limit)
			if listErr != nil {
				if firstErr == nil {
					firstErr = listErr
				}
			} else {
				for _, item := range items {
					if item.ExpireTime == nil {
						continue
					}
					reminders = append(reminders, appusernotifications.ExpiryReminder{
						Kind:           appusernotifications.ExpiryKindCredential,
						OrganizationID: item.OrganizationID,
						SourceID:       item.ID,
						EntityName:     certDisplayName(item.DisplayName, item.Name),
						ExpiresAt:      *item.ExpireTime,
					})
				}
			}
		}
		if scanner, ok := tx.Applications().(applicationsdomain.ExpiringClientCertificateScanner); ok {
			items, listErr := scanner.ListExpiringClientCertificates(ctx, now, deadline, limit)
			if listErr != nil {
				if firstErr == nil {
					firstErr = listErr
				}
			} else {
				for _, item := range items {
					if item.NotAfter == nil {
						continue
					}
					name := strings.TrimSpace(item.ApplicationName)
					if name == "" {
						name = item.ApplicationID
					}
					reminders = append(reminders, appusernotifications.ExpiryReminder{
						Kind:           appusernotifications.ExpiryKindClientCertificate,
						OrganizationID: item.OrganizationID,
						SourceID:       item.ID,
						EntityName:     name,
						ExpiresAt:      *item.NotAfter,
					})
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return reminders, firstErr
}

func certDisplayName(displayName, name string) string {
	if value := strings.TrimSpace(displayName); value != "" {
		return value
	}
	return name
}

func startExpiryReminderWorker(ctx context.Context, uow appuow.UnitOfWork, service appusernotifications.Service) *appusernotifications.ExpiryReminderWorker {
	worker := appusernotifications.NewExpiryReminderWorker(service, expiryScanner{uow: uow}, nil)
	recover.SafeGo("expiry-reminder-worker", func() {
		worker.Run(ctx)
	})
	return worker
}
