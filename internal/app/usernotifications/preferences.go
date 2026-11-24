package usernotifications

import (
	"context"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var categoryOrder = []string{
	string(domain.CategorySystem),
	string(domain.CategorySecurity),
	string(domain.CategoryBusiness),
	string(domain.CategoryFeedback),
	string(domain.CategoryAnnouncement),
}

var channelOrder = []string{
	string(domain.ChannelInApp),
	string(domain.ChannelEmail),
	string(domain.ChannelSMS),
	string(domain.ChannelWeComApp),
	string(domain.ChannelWeChatOfficial),
}

type PreferencesCommand struct {
	OrganizationID string
	UserID         string
	Preferences    []PreferenceDTO
}

// ListPreferences returns the full category × channel matrix. Stored overrides
// win; otherwise in-app is enabled and external channels are disabled by default.
func (s Service) ListPreferences(ctx context.Context, organizationID, userID string) ([]PreferenceDTO, error) {
	if err := requireScope(organizationID, userID); err != nil {
		return nil, err
	}
	stored, err := s.listStoredPreferences(ctx, organizationID, userID)
	if err != nil {
		return nil, err
	}
	overrides := make(map[string]bool, len(stored))
	for _, item := range stored {
		overrides[item.Category+"|"+item.Channel] = item.Enabled
	}
	result := make([]PreferenceDTO, 0, len(categoryOrder)*len(channelOrder))
	for _, category := range categoryOrder {
		for _, channel := range channelOrder {
			enabled, ok := overrides[category+"|"+channel]
			if !ok {
				enabled = channel == string(domain.ChannelInApp)
			}
			result = append(result, PreferenceDTO{Category: category, Channel: channel, Enabled: enabled})
		}
	}
	return result, nil
}

func (s Service) UpdatePreferences(ctx context.Context, command PreferencesCommand) ([]PreferenceDTO, error) {
	if err := requireScope(command.OrganizationID, command.UserID); err != nil {
		return nil, err
	}
	if len(command.Preferences) == 0 {
		return nil, apperrors.NewInvalidArgument("preferences are required")
	}
	now := s.Clock.Now()
	items := make([]*domain.Preference, 0, len(command.Preferences))
	seen := make(map[string]bool, len(command.Preferences))
	for _, pref := range command.Preferences {
		if !validCategories[pref.Category] {
			return nil, apperrors.NewInvalidArgument("invalid notification category")
		}
		if !validChannels[pref.Channel] {
			return nil, apperrors.NewInvalidArgument("invalid notification channel")
		}
		key := pref.Category + "|" + pref.Channel
		if seen[key] {
			return nil, apperrors.NewInvalidArgument("duplicate notification preference")
		}
		seen[key] = true
		id, err := s.UUID.NewString()
		if err != nil {
			return nil, apperrors.WrapInternal(err, "generate preference id")
		}
		items = append(items, &domain.Preference{
			ID:             id,
			OrganizationID: command.OrganizationID,
			UserID:         command.UserID,
			Category:       pref.Category,
			Channel:        pref.Channel,
			Enabled:        pref.Enabled,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}
	if err := withInbox(ctx, s.UOW, func(ctx context.Context, repo domain.Repository) error {
		return repo.UpsertPreferences(ctx, command.OrganizationID, command.UserID, items)
	}); err != nil {
		return nil, err
	}
	return s.ListPreferences(ctx, command.OrganizationID, command.UserID)
}

func (s Service) listStoredPreferences(ctx context.Context, organizationID, userID string) ([]*domain.Preference, error) {
	var stored []*domain.Preference
	err := withInbox(ctx, s.UOW, func(ctx context.Context, repo domain.Repository) error {
		var listErr error
		stored, listErr = repo.ListPreferences(ctx, organizationID, userID)
		return listErr
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

type ChannelBindingCommand struct {
	OrganizationID string
	UserID         string
	Channel        string
	ExternalID     string
}

func (s Service) ListChannelBindings(ctx context.Context, organizationID, userID string) ([]ChannelBindingDTO, error) {
	if err := requireScope(organizationID, userID); err != nil {
		return nil, err
	}
	var items []*domain.ChannelBinding
	err := withInbox(ctx, s.UOW, func(ctx context.Context, repo domain.Repository) error {
		var listErr error
		items, listErr = repo.ListChannelBindings(ctx, organizationID, userID)
		return listErr
	})
	if err != nil {
		return nil, err
	}
	result := make([]ChannelBindingDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toChannelBindingDTO(item))
	}
	return result, nil
}

func (s Service) UpsertChannelBinding(ctx context.Context, command ChannelBindingCommand) (*ChannelBindingDTO, error) {
	if err := requireScope(command.OrganizationID, command.UserID); err != nil {
		return nil, err
	}
	if command.Channel != string(domain.ChannelWeComApp) && command.Channel != string(domain.ChannelWeChatOfficial) {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "unsupported channel binding", map[string]any{
			"field":   "channel",
			"value":   command.Channel,
			"allowed": []string{string(domain.ChannelWeComApp), string(domain.ChannelWeChatOfficial)},
		})
	}
	externalID := strings.TrimSpace(command.ExternalID)
	if externalID == "" {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "externalId is required", map[string]any{
			"field":    "externalId",
			"expected": "non-empty",
		})
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate channel binding id")
	}
	now := s.Clock.Now()
	// 渠道绑定是已登录用户对本人收件箱的自助操作，且只允许企业微信应用/公众号
	// 两类渠道（见上方校验）：无法在绑定瞬间向外部 ID 发验证码，因此不采信
	// 客户端传入的 verified 标志，改为服务端按「本人声明」落库，避免任意客户端
	// 伪造校验状态。联系人解析只读取这里写入的绑定。
	binding := &domain.ChannelBinding{
		ID:             id,
		OrganizationID: command.OrganizationID,
		UserID:         command.UserID,
		Channel:        command.Channel,
		ExternalID:     externalID,
		Verified:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := withInbox(ctx, s.UOW, func(ctx context.Context, repo domain.Repository) error {
		return repo.UpsertChannelBinding(ctx, binding)
	}); err != nil {
		return nil, err
	}
	result := toChannelBindingDTO(binding)
	return &result, nil
}

func (s Service) DeleteChannelBinding(ctx context.Context, command IDCommand) error {
	if err := requireScope(command.OrganizationID, command.UserID); err != nil {
		return err
	}
	if command.ID == "" {
		return apperrors.NewInvalidArgument("channel binding id is required")
	}
	return withInbox(ctx, s.UOW, func(ctx context.Context, repo domain.Repository) error {
		return repo.DeleteChannelBinding(ctx, command.ID, command.OrganizationID, command.UserID)
	})
}
