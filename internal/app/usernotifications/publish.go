package usernotifications

import (
	"context"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainnotificationproviders "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	domainnotifications "github.com/guliangiam/guliangiam/internal/domain/notifications"
	domain "github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Contacts carries the addresses a user has on file for external channels.
// Bindings maps a channel name (wecom_app / wechat_official) to the user's
// external account id (WeCom UserID / WeChat OpenID).
type Contacts struct {
	Email    string
	Phone    string
	Bindings map[string]string
}

// ContactResolver lets Publish fan out to channels without every caller having
// to know how to load a user's profile or channel bindings. Implemented in the
// composition root so app packages stay independent of each other.
type ContactResolver interface {
	ResolveContacts(ctx context.Context, organizationID, userID string) (Contacts, error)
}

// ChannelTarget is one external delivery the caller can reach for this user.
// The inbox row is always written; targets are only enqueued when the user has
// enabled that (category, channel) pair and the organization has a matching
// notification provider.
type ChannelTarget struct {
	Channel   string
	Recipient string
}

// PublishCommand creates one inbox notification and enqueues the enabled
// external channels. It is the single write path every business module uses, so
// "everything that reaches a user also appears in the notification center"
// holds by construction.
type PublishCommand struct {
	// 幂等键：事件驱动路径（outbox 重投）用它派生确定性 ID，重放时命中主键
	// 冲突而不是写入第二条收件箱消息。留空则生成随机 UUID。
	NotificationID string
	OrganizationID string
	UserID         string
	Category       string
	EventType      string
	Title          string
	Content        string
	Link           string
	SourceType     string
	SourceID       string
	AllowReply     bool
	ChannelTargets []ChannelTarget
}

// PublishResult reports what was written, so callers can log the fan-out
// without re-reading the inbox.
type PublishResult struct {
	Notification     NotificationDTO
	EnqueuedChannels []string
}

// Publish writes the inbox row and, for each target the user opted into,
// a pending row in the outbound ledger (notifications). The ledger worker owns
// delivery and retry, so this call never blocks on an external provider.
func (s Service) Publish(ctx context.Context, command PublishCommand) (*PublishResult, error) {
	if command.OrganizationID == "" || command.UserID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId and userId are required")
	}
	if !validCategories[command.Category] {
		return nil, apperrors.NewInvalidArgument("invalid notification category")
	}
	title := strings.TrimSpace(command.Title)
	if title == "" {
		return nil, apperrors.NewInvalidArgument("notification title is required")
	}

	id := strings.TrimSpace(command.NotificationID)
	if id == "" {
		generated, err := s.UUID.NewString()
		if err != nil {
			return nil, apperrors.WrapInternal(err, "generate notification id")
		}
		id = generated
	}
	now := s.Clock.Now()
	var sourceID *string
	if command.SourceID != "" {
		value := command.SourceID
		sourceID = &value
	}
	notification := &domain.Notification{
		ID:             id,
		OrganizationID: command.OrganizationID,
		UserID:         command.UserID,
		Category:       command.Category,
		EventType:      command.EventType,
		Title:          title,
		Content:        command.Content,
		Link:           command.Link,
		SourceType:     command.SourceType,
		SourceID:       sourceID,
		AllowReply:     command.AllowReply,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	targets := mergeChannelTargets(command.ChannelTargets, s.resolveContacts(ctx, command.OrganizationID, command.UserID))

	enqueued := make([]string, 0, len(targets))
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		scoped, ok := tx.(appuow.UserNotificationsUoW)
		if !ok {
			return apperrors.NewInternalf("unit of work %T does not expose UserNotifications", tx)
		}
		repo := scoped.UserNotifications()
		if createErr := repo.Create(ctx, notification); createErr != nil {
			return createErr
		}
		enqueued = s.enqueueChannels(ctx, tx, repo, notification, targets, now)
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := &PublishResult{
		Notification:     toNotificationDTO(notification),
		EnqueuedChannels: enqueued,
	}
	return result, nil
}

func (s Service) resolveContacts(ctx context.Context, organizationID, userID string) Contacts {
	if s.Contacts == nil {
		return Contacts{}
	}
	contacts, err := s.Contacts.ResolveContacts(ctx, organizationID, userID)
	if err != nil {
		slog.WarnContext(ctx, "failed to resolve notification contacts; external channels skipped",
			"organization_id", organizationID, "user_id", userID, "error", err)
		return Contacts{}
	}
	return contacts
}

// mergeChannelTargets keeps explicit caller targets authoritative and appends
// contacts-derived targets for channels the caller did not specify.
func mergeChannelTargets(explicit []ChannelTarget, contacts Contacts) []ChannelTarget {
	merged := make([]ChannelTarget, 0, len(explicit)+2+len(contacts.Bindings))
	seen := make(map[string]bool, len(explicit))
	add := func(channel, recipient string) {
		channel = strings.TrimSpace(channel)
		recipient = strings.TrimSpace(recipient)
		if channel == "" || recipient == "" || seen[channel] {
			return
		}
		seen[channel] = true
		merged = append(merged, ChannelTarget{Channel: channel, Recipient: recipient})
	}
	for _, target := range explicit {
		add(target.Channel, target.Recipient)
	}
	add(string(domain.ChannelEmail), contacts.Email)
	add(string(domain.ChannelSMS), contacts.Phone)
	for channel, recipient := range contacts.Bindings {
		add(channel, recipient)
	}
	return merged
}

// enqueueChannels resolves the provider for every opted-in target and adds a
// pending ledger row inside the publish transaction, so an inbox row never
// commits without its fan-out. Missing preference or provider is not an error:
// the inbox row already exists and delivery is best effort per channel.
func (s Service) enqueueChannels(ctx context.Context, tx appuow.UnitOfWork, repo domain.Repository,
	notification *domain.Notification, targets []ChannelTarget, now time.Time) []string {
	if len(targets) == 0 {
		return nil
	}

	preferences, err := repo.ListPreferences(ctx, notification.OrganizationID, notification.UserID)
	if err != nil {
		slog.WarnContext(ctx, "failed to load notification preferences; skipping external channels",
			"organization_id", notification.OrganizationID, "user_id", notification.UserID, "error", err)
		return nil
	}
	enabled := make(map[string]bool, len(preferences))
	for _, preference := range preferences {
		if preference.Category == notification.Category {
			enabled[preference.Channel] = preference.Enabled
		}
	}

	var providers []*domainnotificationproviders.NotificationProvider
	enqueued := make([]string, 0, len(targets))
	for _, target := range targets {
		channel := strings.TrimSpace(target.Channel)
		recipient := strings.TrimSpace(target.Recipient)
		if channel == "" || channel == string(domain.ChannelInApp) || recipient == "" {
			continue
		}
		if !enabled[channel] {
			continue
		}
		if providers == nil {
			providers = listEnabledProviders(ctx, tx, notification.OrganizationID)
		}
		provider := pickProviderForChannel(providers, channel)
		if provider == nil {
			slog.InfoContext(ctx, "no notification provider configured for channel; inbox only",
				"organization_id", notification.OrganizationID, "channel", channel)
			continue
		}

		deliveryID, idErr := s.UUID.NewString()
		if idErr != nil {
			slog.WarnContext(ctx, "failed to generate delivery id", "channel", channel, "error", idErr)
			continue
		}
		delivery := &domainnotifications.Notification{
			ID:                 deliveryID,
			OrganizationID:     notification.OrganizationID,
			ProviderID:         provider.ID,
			UserNotificationID: notification.ID,
			Channel:            channel,
			ProviderKind:       provider.ProviderType,
			Recipient:          recipient,
			Subject:            notification.Title,
			Content:            renderChannelContent(notification),
			Status:             "pending",
			Variables: map[string]any{
				"event_type": notification.EventType,
				"link":       notification.Link,
				"user_id":    notification.UserID,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		if linkErr := tx.Notifications().Create(ctx, delivery); linkErr != nil {
			slog.WarnContext(ctx, "failed to enqueue channel delivery",
				"notification_id", notification.ID, "channel", channel, "error", linkErr)
			continue
		}
		enqueued = append(enqueued, channel)
	}
	return enqueued
}

// listEnabledProviders reads the tenant's enabled notification providers. The
// list is small (a handful per tenant) and only read when a channel is actually
// opted in.
func listEnabledProviders(ctx context.Context, tx appuow.UnitOfWork, organizationID string) []*domainnotificationproviders.NotificationProvider {
	enabledOnly := true
	providers, _, err := tx.NotificationProviders().List(ctx, domainnotificationproviders.ListQuery{
		OrganizationID: organizationID,
		Page:           1,
		PageSize:       200,
		IsEnabled:      &enabledOnly,
	})
	if err != nil {
		slog.WarnContext(ctx, "failed to list notification providers", "organization_id", organizationID, "error", err)
		return nil
	}
	return providers
}

// pickProviderForChannel maps a logical channel to the notification provider
// type that implements it. WeCom has a legacy group-robot type ("wecom"); the
// per-user app-message type is "wecom_app" and must not be confused with it.
func pickProviderForChannel(providers []*domainnotificationproviders.NotificationProvider, channel string) *domainnotificationproviders.NotificationProvider {
	for _, provider := range providers {
		if provider.ProviderType == channel {
			return provider
		}
	}
	return nil
}

func renderChannelContent(notification *domain.Notification) string {
	if notification.Link == "" {
		return notification.Content
	}
	return notification.Content + "\n\n" + notification.Link
}
