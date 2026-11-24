package bootstrap

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/guliangiam/guliangiam/internal/app/notifications"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/infra/messaging"
	"github.com/guliangiam/guliangiam/internal/infra/notification"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// registerNotificationProviders wires email (via senderFactory), SMS, and 10+ webhook/token
// based notifiers. Returns (factory, worker) — caller must start worker.
func registerNotificationProviders(senderFactory *messaging.SenderFactory, emailProvider, smsProvider string, uow appuow.UnitOfWork) (*notification.Factory, *notifications.Worker) {
	notificationFactory := notification.NewFactory()
	// Register email notifier if an email sender is available.
	if emailSender, err := senderFactory.GetEmailSender(emailProvider); err == nil {
		notificationFactory.Register("email", notification.NewEmailNotifier(emailSender, envOrEmpty("NOTIFICATION_EMAIL_FROM"), "GuliangIAM Notification"))
	}
	// Register SMS notifier if an SMS sender is available.
	if smsSender, err := senderFactory.GetSmsSender(smsProvider); err == nil {
		notificationFactory.Register("sms", notification.NewSmsNotifier(smsSender))
	}
	// Register webhook-based notifiers (Recipient is the webhook URL).
	notificationFactory.Register("dingtalk", notification.NewDingTalkNotifier())
	notificationFactory.Register("lark", notification.NewLarkNotifier())
	notificationFactory.Register("wecom", notification.NewWeComNotifier())
	notificationFactory.Register("slack", notification.NewSlackNotifier())
	notificationFactory.Register("discord", notification.NewDiscordNotifier())
	notificationFactory.Register("rocketchat", notification.NewRocketChatNotifier())
	notificationFactory.Register("teams", notification.NewTeamsNotifier())
	notificationFactory.Register("bark", notification.NewBarkNotifier())
	// Register token-based notifiers if configured.
	if v := envOrEmpty("TELEGRAM_BOT_TOKEN"); v != "" {
		notificationFactory.Register("telegram", notification.NewTelegramNotifier(v))
	}
	if v := envOrEmpty("PUSHOVER_APP_TOKEN"); v != "" {
		notificationFactory.Register("pushover", notification.NewPushoverNotifier(v))
	}
	if v := envOrEmpty("LINE_NOTIFY_ACCESS_TOKEN"); v != "" {
		notificationFactory.Register("line", notification.NewLineNotifier(v))
	}
	if v := envOrEmpty("MATRIX_HOME_SERVER"); v != "" {
		notificationFactory.Register("matrix", notification.NewMatrixNotifier(v, envOrEmpty("MATRIX_ACCESS_TOKEN")))
	}
	if v := envOrEmpty("PUSHBULLET_ACCESS_TOKEN"); v != "" {
		notificationFactory.Register("pushbullet", notification.NewPushbulletNotifier(v))
	}
	// Notifiers with conditional configuration (require environment variables).
	if v := envOrEmpty("VIBER_AUTH_TOKEN"); v != "" {
		notificationFactory.Register("viber", notification.NewViberNotifierWithConfig(v, envOrEmpty("VIBER_BOT_NAME")))
	}
	if v := envOrEmpty("WEBPUSH_VAPID_PRIVATE_KEY"); v != "" {
		notificationFactory.Register("webpush", notification.NewWebPushNotifierWithConfig(envOrEmpty("WEBPUSH_VAPID_PUBLIC_KEY"), v, envOrEmpty("WEBPUSH_SUBSCRIBER_EMAIL")))
	}
	if v := envOrEmpty("TWITTER_OAUTH2_TOKEN"); v != "" {
		notificationFactory.Register("twitter", notification.NewTwitterNotifierWithConfig(v, envOrEmpty("TWITTER_API_KEY"), envOrEmpty("TWITTER_API_SECRET")))
	}
	if v := envOrEmpty("REDDIT_ACCESS_TOKEN"); v != "" {
		notificationFactory.Register("reddit", notification.NewRedditNotifierWithConfig(v, envOrEmpty("REDDIT_CLIENT_ID"), envOrEmpty("REDDIT_CLIENT_SECRET"), envOrEmpty("REDDIT_USERNAME")))
	}

	// Per-record notifiers: their endpoint/credentials live on the provider
	// row, which the worker forwards through the notification metadata.
	notificationFactory.RegisterBuilder("customhttp", func(n *notification.Notification) (notification.Provider, error) {
		endpoint := notificationMetadataString(n.Metadata, "webhook_url", "endpoint", "customApi", "host")
		if endpoint == "" {
			return nil, apperrors.NewUnavailable("customhttp notification provider requires a webhookUrl")
		}
		return notification.NewCustomHTTPNotifier(endpoint, notificationMetadataHeaders(n.Metadata)), nil
	})
	notificationFactory.RegisterBuilder("googlechat", func(n *notification.Notification) (notification.Provider, error) {
		endpoint := notificationMetadataString(n.Metadata, "webhook_url", "endpoint", "customApi", "host")
		if endpoint == "" {
			return nil, apperrors.NewUnavailable("googlechat notification provider requires a webhookUrl")
		}
		return notification.NewGoogleChatNotifier(endpoint), nil
	})
	notificationFactory.RegisterBuilder("cucloud", func(n *notification.Notification) (notification.Provider, error) {
		appID := notificationMetadataString(n.Metadata, "app_id", "appId", "admin")
		apiKey := notificationMetadataString(n.Metadata, "api_key", "apiKey", "access_token", "password")
		if appID == "" || apiKey == "" {
			return nil, apperrors.NewUnavailable("cucloud notification provider requires appId and apiKey")
		}
		return notification.NewCUCloudNotifier(appID, apiKey), nil
	})
	notificationFactory.RegisterBuilder("ntfy", func(n *notification.Notification) (notification.Provider, error) {
		server := notificationMetadataString(n.Metadata, "webhook_url", "endpoint", "customApi", "host")
		if server == "" {
			return nil, apperrors.NewUnavailable("ntfy notification provider requires a server URL (webhookUrl)")
		}
		// An empty topic falls back to the recipient inside the notifier.
		topic := notificationMetadataString(n.Metadata, "topic")
		token := notificationMetadataString(n.Metadata, "access_token", "api_key")
		return notification.NewNtfyNotifier(server, topic, token), nil
	})
	notificationFactory.RegisterBuilder("serverchan", func(n *notification.Notification) (notification.Provider, error) {
		sendKey := notificationMetadataString(n.Metadata, "access_token", "api_key", "send_key", "sendKey")
		if sendKey == "" {
			return nil, apperrors.NewUnavailable("serverchan notification provider requires a sendKey (accessToken)")
		}
		return notification.NewServerChanNotifier(sendKey), nil
	})
	notificationFactory.RegisterBuilder("pagerduty", func(n *notification.Notification) (notification.Provider, error) {
		routingKey := notificationMetadataString(n.Metadata, "routing_key", "routingKey", "access_token", "api_key")
		if routingKey == "" {
			return nil, apperrors.NewUnavailable("pagerduty notification provider requires a routingKey (accessToken)")
		}
		severity := notificationMetadataString(n.Metadata, "severity")
		source := notificationMetadataString(n.Metadata, "source")
		return notification.NewPagerDutyNotifierWithConfig(routingKey, severity, source), nil
	})
	notificationFactory.RegisterBuilder("opsgenie", func(n *notification.Notification) (notification.Provider, error) {
		apiKey := notificationMetadataString(n.Metadata, "api_key", "apiKey", "access_token")
		if apiKey == "" {
			return nil, apperrors.NewUnavailable("opsgenie notification provider requires an apiKey (accessToken)")
		}
		apiURL := notificationMetadataString(n.Metadata, "api_url", "apiUrl", "webhook_url", "endpoint")
		if apiURL == "" && strings.EqualFold(notificationMetadataString(n.Metadata, "region"), "eu") {
			apiURL = notification.OpsGenieEUAlertsURL
		}
		return notification.NewOpsGenieNotifierWithConfig(apiKey, apiURL), nil
	})
	notificationFactory.RegisterBuilder("gotify", func(n *notification.Notification) (notification.Provider, error) {
		baseURL := notificationMetadataString(n.Metadata, "webhook_url", "endpoint", "customApi", "host")
		if baseURL == "" {
			return nil, apperrors.NewUnavailable("gotify notification provider requires a server URL (webhookUrl)")
		}
		token := notificationMetadataString(n.Metadata, "access_token", "api_key", "token")
		priority := notificationMetadataInt(n.Metadata, "priority", 5)
		return notification.NewGotifyNotifierWithConfig(baseURL, token, priority), nil
	})

	notificationFactory.RegisterBuilder("wecom_app", func(n *notification.Notification) (notification.Provider, error) {
		corpID := notificationMetadataString(n.Metadata, "corp_id", "corpId", "admin")
		secret := notificationMetadataString(n.Metadata, "secret", "corp_secret", "corpSecret", "access_token", "password")
		agentID := notificationMetadataString(n.Metadata, "agent_id", "agentId")
		notifier, err := notification.NewWeComAppNotifier(corpID, secret, agentID)
		if err != nil {
			return nil, apperrors.WrapUnavailablef(err, "wecom_app notification provider config")
		}
		return notifier, nil
	})
	notificationFactory.RegisterBuilder("wechat_official", func(n *notification.Notification) (notification.Provider, error) {
		appID := notificationMetadataString(n.Metadata, "app_id", "appId", "admin")
		secret := notificationMetadataString(n.Metadata, "app_secret", "appSecret", "secret", "access_token", "password")
		templateID := notificationMetadataString(n.Metadata, "template_id", "templateId", "template")
		link := notificationMetadataString(n.Metadata, "url", "link")
		notifier, err := notification.NewWeChatOfficialNotifier(appID, secret, templateID, link)
		if err != nil {
			return nil, apperrors.WrapUnavailablef(err, "wechat_official notification provider config")
		}
		return notifier, nil
	})

	// email / sms 优先使用提供商记录上的配置（多租户各自一套 SMTP/短信账号）。
	// 部署级环境变量仍作为兜底：记录缺少必填项时回落到启动时注册的共享实例，
	// 两者都没有则在投递台账留下可读的 unavailable 原因。
	notificationFactory.RegisterBuilder("email", func(n *notification.Notification) (notification.Provider, error) {
		host := notificationMetadataString(n.Metadata, "host", "smtp_host", "smtpHost")
		if host == "" {
			return notificationFactory.GetProvider("email")
		}
		from := notificationMetadataString(n.Metadata, "from", "from_address", "fromAddress", "sender")
		if from == "" {
			from = envOrEmpty("NOTIFICATION_EMAIL_FROM")
		}
		if from == "" {
			return nil, apperrors.NewUnavailable("email notification provider requires a from address (config.from)")
		}
		port := notificationMetadataInt(n.Metadata, "port", 587)
		username := notificationMetadataString(n.Metadata, "username", "user")
		password := notificationMetadataString(n.Metadata, "password", "access_token", "api_key")
		subject := notificationMetadataString(n.Metadata, "subject")
		if subject == "" {
			subject = "GuliangIAM Notification"
		}
		sender := messaging.NewSMTPSender(host, port, username, password, from)
		return notification.NewEmailNotifier(sender, from, subject), nil
	})
	notificationFactory.RegisterBuilder("sms", func(n *notification.Notification) (notification.Provider, error) {
		accessKeyID := notificationMetadataString(n.Metadata, "access_key_id", "accessKeyId")
		accessKeySecret := notificationMetadataString(n.Metadata, "access_key_secret", "accessKeySecret", "access_token", "api_key")
		signName := notificationMetadataString(n.Metadata, "sign_name", "signName")
		templateCode := notificationMetadataString(n.Metadata, "template_code", "templateCode")
		if accessKeyID == "" || accessKeySecret == "" || signName == "" || templateCode == "" {
			return notificationFactory.GetProvider("sms")
		}
		return notification.NewSmsNotifier(messaging.NewAlibabaSmsSender(accessKeyID, accessKeySecret, signName, templateCode)), nil
	})

	// Start the notification delivery worker after all providers are registered.
	notificationWorker := notifications.NewWorker(uow, notificationFactory, nil)
	// DEF-35：notifications 为 FORCE RLS 表，Worker 必须经系统作用域事务读写。
	notificationWorker.SetSystemTxRunner(systemTxRunner(uow))
	return notificationFactory, notificationWorker
}

// notificationMetadataString returns the first non-empty string value under any
// of the candidate metadata keys.
func notificationMetadataString(metadata map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

// notificationMetadataInt reads a numeric metadata value (JSON decodes all
// numbers as float64; provider config may also store a string). It falls back
// to def when the key is absent or not a number.
func notificationMetadataInt(metadata map[string]any, key string, def int) int {
	value, ok := metadata[key]
	if !ok {
		return def
	}
	switch number := value.(type) {
	case float64:
		return int(number)
	case int:
		return number
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(number)); err == nil {
			return parsed
		}
	}
	return def
}

// notificationMetadataHeaders accepts either a decoded JSON object or the JSON
// string stored in the provider's customHeader column.
func notificationMetadataHeaders(metadata map[string]any) map[string]string {
	headers := map[string]string{}
	raw, ok := metadata["headers"]
	if !ok {
		raw = metadata["customHeader"]
	}
	switch value := raw.(type) {
	case map[string]any:
		for key, item := range value {
			if text, ok := item.(string); ok {
				headers[key] = text
			}
		}
	case map[string]string:
		for key, text := range value {
			headers[key] = text
		}
	case string:
		_ = json.Unmarshal([]byte(value), &headers)
	}
	return headers
}
