// Package usernotifications implements the per-user notification center use cases.
package usernotifications

import (
	"context"

	"github.com/guliangiam/guliangiam/internal/app/uow"
	usernotifications "github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// Service orchestrates the personal inbox. UOW carries the narrow
// UserNotificationsUoW capability: inbox tables are FORCE RLS, so every read
// and write runs inside WithinTransaction where the tenant GUC is set.
type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   uow.UserNotificationsUoW
	// Contacts 可选：注入后 Publish 会自动解析用户的邮箱/手机/渠道绑定，
	// 把多渠道路由从调用方解耦。为 nil 时仅写站内信。
	Contacts ContactResolver
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.UserNotificationsUoW) Service {
	return Service{
		Clock: clk,
		UUID:  uuidGenerator,
		UOW:   unitOfWork,
	}
}

// withInbox runs fn inside a unit of work transaction with the tenant GUC set.
// The transaction handle is narrowed to UserNotificationsUoW because the
// shared UnitOfWork interface intentionally omits UserNotifications (see
// domain/uow.UserNotificationsUoW).
func withInbox(ctx context.Context, unitOfWork uow.UserNotificationsUoW, fn func(context.Context, usernotifications.Repository) error) error {
	return unitOfWork.WithinTransaction(ctx, func(ctx context.Context, tx uow.UnitOfWork) error {
		scoped, ok := tx.(uow.UserNotificationsUoW)
		if !ok {
			return apperrors.NewInternalf("unit of work %T does not expose UserNotifications", tx)
		}
		return fn(ctx, scoped.UserNotifications())
	})
}
