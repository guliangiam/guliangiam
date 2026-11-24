package messages

import (
	"context"
	"net/mail"
	"regexp"
	"strings"

	domainnotificationproviders "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	domainnotifications "github.com/guliangiam/guliangiam/internal/domain/notifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Service 是 messages 传输层的端口接口。
// transport/messages handler 依赖此接口（间接调用 domain NotificationFactory），
// 符合 Rule 2 "transport 不直接 import domain/infra" 的约束。
type Service interface {
	SendEmail(ctx context.Context, to, subject, body string) error
	SendSms(ctx context.Context, phone, code string) error
	SendNotification(ctx context.Context, orgID, providerID, title, content string, targets []string) error
}

// ProviderLookup resolves a stored notification provider through a
// tenant-scoped read. It is the first stage of the providerID → providerType
// resolution: the factory registry is keyed by type, never by row ID.
type ProviderLookup interface {
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domainnotificationproviders.NotificationProvider, error)
}

// notificationService 是 Service 的默认实现，内部委托给 domain NotificationFactory。
type notificationService struct {
	factory   domainnotifications.NotificationFactory
	providers ProviderLookup
}

// NewService 从 domain NotificationFactory 构造 messages Service。
// transport/wire 层调用此函数，传入 infra/notification.Factory（已实现 domain 接口）。
// 不带 ProviderLookup 时，SendNotification 的 providerID 路径不可用（显式报错）。
func NewService(factory domainnotifications.NotificationFactory) Service {
	return &notificationService{factory: factory}
}

// NewServiceWithProviderLookup 构造支持 providerID 两级解析的 Service：
// providerID → GetByIDForOrganization（租户归属校验）→ 实体.ProviderType → 工厂。
func NewServiceWithProviderLookup(factory domainnotifications.NotificationFactory, providers ProviderLookup) Service {
	return &notificationService{factory: factory, providers: providers}
}

// phonePattern 是手机号的基本校验（E.164 风格：可选 +，6-15 位数字）。
var phonePattern = regexp.MustCompile(`^\+?[0-9]{6,15}$`)

func (s *notificationService) SendEmail(ctx context.Context, to, subject, body string) error {
	if err := validateEmailRecipient(to); err != nil {
		return err
	}
	return s.factory.SendNotification(ctx, "email", &domainnotifications.SendNotification{
		Type:      "email",
		Recipient: to,
		Subject:   subject,
		Content:   body,
	})
}

func (s *notificationService) SendSms(ctx context.Context, phone, code string) error {
	if err := validatePhoneRecipient(phone); err != nil {
		return err
	}
	return s.factory.SendNotification(ctx, "sms", &domainnotifications.SendNotification{
		Type:      "sms",
		Recipient: phone,
		Content:   code,
	})
}

func (s *notificationService) SendNotification(ctx context.Context, orgID, providerID, title, content string, targets []string) error {
	providerType := "notification"
	if providerID != "" {
		resolved, err := s.resolveProviderType(ctx, orgID, providerID)
		if err != nil {
			return err
		}
		providerType = resolved
	}
	for _, target := range targets {
		if err := s.factory.SendNotification(ctx, providerType, &domainnotifications.SendNotification{
			Type:           "notification",
			OrganizationID: orgID,
			Recipient:      target,
			Subject:        title,
			Content:        content,
		}); err != nil {
			return err
		}
	}
	return nil
}

// resolveProviderType maps a provider row ID to the factory's type key. The
// previous code passed the UUID straight to the factory, whose registry is
// keyed by type ("email"/"sms"/…), so every providerID call failed with 503.
// The tenant-scoped read also rejects a providerID belonging to another org.
func (s *notificationService) resolveProviderType(ctx context.Context, orgID, providerID string) (string, error) {
	if s.providers == nil {
		return "", apperrors.NewUnavailable("notification provider lookup is not configured")
	}
	provider, err := s.providers.GetByIDForOrganization(ctx, providerID, orgID)
	if err != nil {
		return "", err
	}
	if provider == nil || strings.TrimSpace(provider.ProviderType) == "" {
		return "", apperrors.NewInvalidArgument("notification provider has no providerType")
	}
	return provider.ProviderType, nil
}

func validateEmailRecipient(to string) error {
	if strings.TrimSpace(to) == "" {
		return apperrors.NewInvalidArgument("to is required")
	}
	if _, err := mail.ParseAddress(to); err != nil {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "to must be a valid email address", map[string]any{
			"field": "to", "value": to, "format": "email",
		})
	}
	return nil
}

func validatePhoneRecipient(phone string) error {
	if strings.TrimSpace(phone) == "" {
		return apperrors.NewInvalidArgument("phone is required")
	}
	if !phonePattern.MatchString(phone) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "phone must be a valid phone number (optional +, 6-15 digits)", map[string]any{
			"field": "phone", "value": phone, "format": "optional +, 6-15 digits",
		})
	}
	return nil
}
