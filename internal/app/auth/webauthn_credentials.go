package auth

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// WebAuthnCredentialSummary 是自服务凭据列表的展示投影（不暴露公钥与内部挑战）。
type WebAuthnCredentialSummary struct {
	ID           string    `json:"id"`
	CredentialID string    `json:"credentialId"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	AAGUID       string    `json:"aaguid,omitempty"`
	SignCount    int64     `json:"signCount"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// ListWebAuthnCredentials 返回会话本人名下的 Passkey 凭据。
// 身份只取可信会话（SessionUserID/SessionOrganizationID），请求体/查询参数不能指定他人。
func (s *Service) ListWebAuthnCredentials(ctx context.Context, sessionUserID, sessionOrganizationID string) ([]WebAuthnCredentialSummary, error) {
	if strings.TrimSpace(sessionUserID) == "" {
		return nil, apperrors.NewUnauthorized("a session is required to list WebAuthn credentials")
	}
	user, err := s.sessionUserForOrganization(ctx, sessionUserID, sessionOrganizationID)
	if err != nil {
		return nil, err
	}
	credentials, err := s.UOW.WebAuthnCredentials().ListByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	summaries := make([]WebAuthnCredentialSummary, 0, len(credentials))
	for _, credential := range credentials {
		summaries = append(summaries, toWebAuthnCredentialSummary(credential))
	}
	return summaries, nil
}

// RevokeWebAuthnCredential 停用（status=revoked）会话本人名下的一条 Passkey。
// 仅能命中会话用户自己的凭据：凭据行 ID 属他用户或不存在统一 404（防枚举）。
func (s *Service) RevokeWebAuthnCredential(ctx context.Context, sessionUserID, sessionOrganizationID, credentialRowID string) error {
	if strings.TrimSpace(sessionUserID) == "" {
		return apperrors.NewUnauthorized("a session is required to revoke WebAuthn credentials")
	}
	if strings.TrimSpace(credentialRowID) == "" {
		return apperrors.NewInvalidArgument("credentialId is required")
	}
	user, err := s.sessionUserForOrganization(ctx, sessionUserID, sessionOrganizationID)
	if err != nil {
		return err
	}
	credentials, err := s.UOW.WebAuthnCredentials().ListByUserID(ctx, user.ID)
	if err != nil {
		return err
	}
	for _, credential := range credentials {
		if credential.ID != credentialRowID {
			continue
		}
		return s.UOW.WebAuthnCredentials().UpdateStatus(ctx, credential.ID, webauthndomain.CredentialStatusRevoked)
	}
	return apperrors.NewNotFound("webauthn credential not found")
}

func (s *Service) sessionUserForOrganization(ctx context.Context, sessionUserID, sessionOrganizationID string) (*userdomain.User, error) {
	sessionUserID = strings.TrimSpace(sessionUserID)
	if strings.TrimSpace(sessionOrganizationID) == "" {
		return nil, apperrors.NewForbidden("authenticated session has no organization scope")
	}
	user, err := s.UOW.Users().GetByIDForOrganization(ctx, sessionUserID, sessionOrganizationID)
	if err != nil {
		return nil, apperrors.NewForbidden("authenticated session user does not belong to this organization")
	}
	return user, nil
}

func toWebAuthnCredentialSummary(credential *webauthndomain.Credential) WebAuthnCredentialSummary {
	if credential == nil {
		return WebAuthnCredentialSummary{}
	}
	return WebAuthnCredentialSummary{
		ID:           credential.ID,
		CredentialID: base64.RawURLEncoding.EncodeToString(credential.CredentialID),
		Name:         credential.Name,
		Status:       string(credential.Status),
		AAGUID:       credential.AAGUID,
		SignCount:    credential.SignCount,
		CreatedAt:    credential.CreatedAt,
		UpdatedAt:    credential.UpdatedAt,
	}
}
