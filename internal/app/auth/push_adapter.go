package auth

import (
	"context"
	"fmt"
	"strings"

	domainnotif "github.com/guliangiam/guliangiam/internal/domain/notifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PushNotifier interface {
	Send(ctx context.Context, providerType string, config pushProviderConfig, receiver, title, body string) error
}

type defaultPushNotifier struct {
	factory domainnotif.NotificationFactory
}

func NewDefaultPushNotifier(factory domainnotif.NotificationFactory) PushNotifier {
	return &defaultPushNotifier{factory: factory}
}

func (n *defaultPushNotifier) Send(ctx context.Context, providerType string, config pushProviderConfig, receiver, title, body string) error {
	if n.factory == nil {
		return apperrors.NewUnavailable("notification factory is not configured")
	}

	metadata := make(map[string]interface{})
	switch strings.ToLower(providerType) {
	case "pushbullet":
		metadata["accessToken"] = config.clientSecret
	case "pushover":
		metadata["accessToken"] = config.clientSecret
	case "slack":
		webhookURL := firstNonEmpty(config.clientSecret, config.metadata)
		if webhookURL == "" {
			return apperrors.NewInvalidArgument("slack webhook url is required")
		}
		metadata["webhook_url"] = webhookURL
	case "dingtalk":
		var webhookURL string
		switch {
		case strings.HasPrefix(config.clientID, "http"):
			webhookURL = config.clientID
		case strings.HasPrefix(config.metadata, "http"):
			webhookURL = config.metadata
		case config.clientID != "":
			webhookURL = fmt.Sprintf("https://oapi.dingtalk.com/robot/send?access_token=%s", config.clientID)
		}
		if webhookURL == "" {
			return apperrors.NewInvalidArgument("dingtalk webhook url or access token is required")
		}
		metadata["webhook_url"] = webhookURL
	case "lark":
		webhookURL := firstNonEmpty(config.clientSecret, config.metadata)
		if webhookURL == "" {
			return apperrors.NewInvalidArgument("lark webhook url is required")
		}
		metadata["webhook_url"] = webhookURL
	case "wecom":
		webhookURL := firstNonEmpty(config.clientSecret, config.metadata)
		if webhookURL == "" {
			return apperrors.NewInvalidArgument("wecom webhook url is required")
		}
		metadata["webhook_url"] = webhookURL
	case "bark":
		deviceKey := firstNonEmpty(config.clientSecret, config.clientID)
		if deviceKey == "" {
			return apperrors.NewInvalidArgument("bark device key is required")
		}
		metadata["deviceKey"] = deviceKey
	case "telegram":
		botToken := firstNonEmpty(config.clientSecret, config.clientID)
		if botToken == "" {
			return apperrors.NewInvalidArgument("telegram bot token is required")
		}
		metadata["botToken"] = botToken
	default:
		return apperrors.NewUnimplementedf("push notification provider %q is not yet implemented", providerType)
	}

	notif := &domainnotif.SendNotification{
		Type:      strings.ToLower(providerType),
		Recipient: receiver,
		Subject:   title,
		Content:   body,
		Metadata:  metadata,
	}

	return n.factory.SendNotification(ctx, strings.ToLower(providerType), notif)
}
