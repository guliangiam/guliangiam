package auth

import (
	"context"
	"log/slog"
	"strings"

	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	providerlogoutevent "github.com/guliangiam/guliangiam/internal/platform/providerlogout"
)

// socialSessionDeviceInfo stores the provider identity used by this session
// and, when a KEK is available, the sealed upstream access token that the
// custom logout endpoint needs. Without a KEK the token is not retained.
func (s *Service) socialSessionDeviceInfo(ctx context.Context, provider *providerdomain.Provider, userInfo *SocialUserInfo) map[string]any {
	info := map[string]any{
		paramLoginMethod:                   loginMethodSocial,
		"providerType":                     string(provider.ProviderType),
		sessiondomain.DeviceInfoProviderID: provider.ID,
	}

	if token := strings.TrimSpace(userInfo.AccessToken); token != "" {
		if cipher := s.providerTokenCipher(); cipher != nil {
			sealed, err := cipher.EncryptField(token)
			if err != nil {
				slog.WarnContext(ctx, "failed to seal provider access token for logout", "provider_id", provider.ID, "error", err)
			} else {
				info[sessiondomain.DeviceInfoProviderAccessToken] = sealed
			}
		}
	}
	return info
}

func (s *Service) providerTokenCipher() ProviderTokenCipher {
	if s.ProviderTokenCipher != nil {
		return s.ProviderTokenCipher
	}
	if platformcrypto.FieldEncryptionConfigured() {
		return platformcrypto.GlobalFieldEncryptor()
	}
	return nil
}

// enqueueProviderLogout records the upstream logout as an Outbox event. The
// request path performs no external call (S-12): the outbox worker delivers the
// POST under the shared retry/backoff policy. This is best-effort — the local
// session is already revoked, so a failed enqueue must never fail the logout.
func (s *Service) enqueueProviderLogout(ctx context.Context, session *sessiondomain.Session) {
	if session == nil || session.DeviceInfo == nil || s.Outbox == nil {
		return
	}
	providerID, _ := session.DeviceInfo[sessiondomain.DeviceInfoProviderID].(string)
	sealed, _ := session.DeviceInfo[sessiondomain.DeviceInfoProviderAccessToken].(string)
	if strings.TrimSpace(providerID) == "" || strings.TrimSpace(sealed) == "" {
		return
	}
	providerType, _ := session.DeviceInfo["providerType"].(string)

	event, err := providerlogoutevent.NewEvent(providerlogoutevent.Request{
		OrganizationID:    session.OrganizationID,
		SessionID:         session.ID,
		ProviderID:        providerID,
		ProviderType:      providerType,
		SealedAccessToken: sealed,
		RequestedAt:       s.Clock.Now().UTC(),
	})
	if err != nil {
		slog.WarnContext(ctx, "provider logout event build failed", "provider_id", providerID, "error", err)
		return
	}
	if enqueueErr := s.Outbox.Enqueue(ctx, event); enqueueErr != nil {
		slog.WarnContext(ctx, "provider logout event enqueue failed", "provider_id", providerID, "error", enqueueErr)
	}
}
