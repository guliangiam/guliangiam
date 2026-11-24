package idp

import (
	"context"
	"fmt"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TelegramProvider implements IdP authentication via Telegram.
type TelegramProvider struct {
	BaseOAuthProvider
}

// NewTelegramProvider creates a new Telegram OAuth2 provider.
//
// 2026-11-14 OPEN-API-75 P4 校准：登录走 Login Widget（https://oauth.telegram.org/auth），
// 校验方式为 bot token HMAC（https://core.telegram.org/api/widgets）；
// 无 code exchange，getMe 仅用于回验 bot 凭据。
func NewTelegramProvider(config *OAuthConfig) *TelegramProvider {
	return &TelegramProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://oauth.telegram.org/auth",
			"",
			fmt.Sprintf("https://api.telegram.org/bot%s/getme", config.ClientSecret),
		),
	}
}

// ExchangeCode is not supported for Telegram authentication; it uses a different flow.
func (p *TelegramProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	return nil, apperrors.NewUnimplemented("telegram provider does not support code exchange; validate the widget data directly")
}
