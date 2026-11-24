package bootstrap

import (
	"context"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appidv "github.com/guliangiam/guliangiam/internal/app/idv"
	apptickets "github.com/guliangiam/guliangiam/internal/app/tickets"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appusernotifications "github.com/guliangiam/guliangiam/internal/app/usernotifications"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	tentxtx "github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// authInboxNotifier adapts the auth security-alert port onto the personal
// notification center.
type authInboxNotifier struct {
	service appusernotifications.Service
}

func newAuthInboxNotifier(service appusernotifications.Service) appauth.NotificationPublisher {
	return authInboxNotifier{service: service}
}

func (n authInboxNotifier) Publish(ctx context.Context, command appauth.NotificationCommand) error {
	_, err := n.service.Publish(ctx, appusernotifications.PublishCommand{
		OrganizationID: command.OrganizationID,
		UserID:         command.UserID,
		Category:       command.Category,
		EventType:      command.EventType,
		Title:          command.Title,
		Content:        command.Content,
		Link:           command.Link,
		SourceType:     command.SourceType,
		SourceID:       command.SourceID,
	})
	return err
}

// idvInboxNotifier adapts the identity-verification result port onto the
// personal notification center. Provider callbacks arrive without tenant
// context, so the adapter pins system scope and relies on the explicit
// organization id carried by the verification row.
type idvInboxNotifier struct {
	service appusernotifications.Service
}

func newIDVInboxNotifier(service appusernotifications.Service) appidv.Notifier {
	return idvInboxNotifier{service: service}
}

func (n idvInboxNotifier) Notify(ctx context.Context, command appidv.NotificationCommand) error {
	_, err := n.service.Publish(tentxtx.System(ctx), appusernotifications.PublishCommand{
		OrganizationID: command.OrganizationID,
		UserID:         command.UserID,
		Category:       command.Category,
		EventType:      command.EventType,
		Title:          command.Title,
		Content:        command.Content,
		Link:           command.Link,
		SourceType:     command.SourceType,
		SourceID:       command.SourceID,
	})
	return err
}

// usersInboxNotifier adapts the users use cases' security-alert port onto the
// personal notification center. Administrator password resets run inside a
// tenant-scoped request, but the inbox table is FORCE RLS; pin system scope so
// the explicit organization id carried by the reset command decides the row.
type usersInboxNotifier struct {
	service appusernotifications.Service
}

func newUsersInboxNotifier(service appusernotifications.Service) appusers.Notifier {
	return usersInboxNotifier{service: service}
}

func (n usersInboxNotifier) Notify(ctx context.Context, command appusers.NotificationCommand) error {
	_, err := n.service.Publish(tentxtx.System(ctx), appusernotifications.PublishCommand{
		OrganizationID: command.OrganizationID,
		UserID:         command.UserID,
		Category:       command.Category,
		EventType:      command.EventType,
		Title:          command.Title,
		Content:        command.Content,
		Link:           command.Link,
		SourceType:     command.SourceType,
		SourceID:       command.SourceID,
	})
	return err
}

// ticketInboxNotifier adapts the ticket use cases' notification port onto the
// personal notification center. Tickets identify people by username (the
// product's creator/assignee fields), so the adapter resolves the username to a
// user id inside a tenant-scoped transaction before publishing.
type ticketInboxNotifier struct {
	uow     appuow.UnitOfWork
	service appusernotifications.Service
}

func newTicketInboxNotifier(uow appuow.UnitOfWork, service appusernotifications.Service) apptickets.Notifier {
	return ticketInboxNotifier{uow: uow, service: service}
}

func (n ticketInboxNotifier) Notify(ctx context.Context, command apptickets.NotificationCommand) error {
	userID := command.RecipientUserID
	if userID == "" {
		if command.RecipientUsername == "" {
			return nil
		}
		var resolved string
		err := n.uow.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
			user, findErr := tx.Users().GetByUsername(ctx, command.OrganizationID, command.RecipientUsername)
			if findErr != nil {
				return findErr
			}
			resolved = user.ID
			return nil
		})
		if err != nil {
			return apperrors.WrapInternalf(err, "resolve ticket notification recipient %q", command.RecipientUsername)
		}
		userID = resolved
	}
	if userID == "" {
		return nil
	}

	_, err := n.service.Publish(ctx, appusernotifications.PublishCommand{
		OrganizationID: command.OrganizationID,
		UserID:         userID,
		Category:       command.Category,
		EventType:      command.EventType,
		Title:          command.Title,
		Content:        command.Content,
		Link:           command.Link,
		SourceType:     command.SourceType,
		SourceID:       command.SourceID,
		AllowReply:     command.AllowReply,
	})
	return err
}
