package applications

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/platform/config"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// secretPrefixDisplayLength 是脱敏展示前缀长度：gl_cs_ + 6 个随机字符。
const secretPrefixDisplayLength = len(clientSecretPrefix) + 6

// RotateClientSecretCommand 是管理面轮换 client secret 的输入。
type RotateClientSecretCommand struct {
	ID             string
	OrganizationID string
	ActorID        string
}

// RotateClientSecretResult 是轮换结果。ClientSecretPlain 只在此响应中出现一次。
type RotateClientSecretResult struct {
	ClientSecretPlain  string
	ClientSecretPrefix string
	RotatedAt          time.Time
	GraceUntil         time.Time
}

// CreateClientSecretMaterial 生成一次性明文 secret，并返回可持久化的行。
// 明文不落库、不进日志；哈希与脱敏前缀是唯一持久化内容。
func (s *Service) buildClientSecretRow(applicationID string, material ClientSecretMaterial, now time.Time) (*domain.ClientSecret, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}
	return &domain.ClientSecret{
		ID:            id,
		ApplicationID: applicationID,
		SecretPrefix:  secretDisplayPrefix(material.Plain),
		SecretHash:    material.Hash,
		Status:        domain.ClientSecretStatusActive,
	}, nil
}

// secretDisplayPrefix 取 "gl_cs_" + 前 6 个随机字符。短于展示宽度的输入
// （历史自定义 secret）只展示前一半字符：脱敏前缀绝不能等于完整明文，
// 否则读接口会把凭据原样泄露给任何能看到应用详情的人。
func secretDisplayPrefix(plaintext string) string {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return ""
	}
	if len(plaintext) <= secretPrefixDisplayLength {
		return plaintext[:len(plaintext)/2]
	}
	return plaintext[:secretPrefixDisplayLength]
}

// RotateClientSecret 轮换某应用的 active secret：
//   - 旧 active 行降级为 grace，grace_until = now + 配置窗口（默认 24h，可配置）；
//   - 新 secret 以 active 写入；
//   - 明文只在本次响应返回一次。
//
// 轮换仅对 client_secret_basic / client_secret_post 有意义；JWT 与 mTLS 方式
// 返回 422，避免产生永不使用的凭据。
func (s *Service) RotateClientSecret(ctx context.Context, command RotateClientSecretCommand) (*RotateClientSecretResult, error) {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(command.ID)
	if id == "" {
		return nil, apperrors.NewInvalidArgument("application id is required")
	}

	plaintext, err := generateOAuthClientSecret()
	if err != nil {
		return nil, err
	}
	hash, err := platformcrypto.HashClientSecret(plaintext)
	if err != nil {
		return nil, err
	}
	material := ClientSecretMaterial{Plain: plaintext, Hash: hash}

	now := s.Clock.Now()
	graceUntil := now.Add(config.ClientSecretRotationGrace())

	var row *domain.ClientSecret
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		application, err := tx.Applications().GetByIDForOrganization(ctx, id, command.OrganizationID)
		if err != nil {
			return err
		}
		if !application.RequiresClientSecret() {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"client secret rotation is only available for client_secret_basic and client_secret_post",
				map[string]any{metaKeyField: "tokenEndpointAuthMethod"})
		}

		row, err = s.buildClientSecretRow(application.ID, material, now)
		if err != nil {
			return err
		}
		if err := tx.Applications().RotateClientSecret(ctx, application.OrganizationID, application.ID, row, now, graceUntil); err != nil {
			return err
		}

		return s.embedAuditEvent(
			tx,
			ctx,
			application.OrganizationID,
			command.ActorID,
			"application.client_secret.rotated",
			application.ID,
			"application.client_secret.rotated",
			// 只放可公开字段：新 secret 的掩码前缀与 grace 截止时间。
			map[string]any{
				"clientSecretPrefix": row.SecretPrefix,
				"graceUntil":         graceUntil.UTC().Format(time.RFC3339),
			},
		)
	})
	if err != nil {
		return nil, err
	}

	return &RotateClientSecretResult{
		ClientSecretPlain:  plaintext,
		ClientSecretPrefix: row.SecretPrefix,
		RotatedAt:          now,
		GraceUntil:         graceUntil,
	}, nil
}

// clientSecretState 汇总某应用的 secret 展示状态（只读）。
// set 表示存在 active 或 grace 窗口内的行；masked 取最新可用行的前缀。
// 历史兼容：只有 legacy 列有哈希时仍报告“已设置”，但无前缀可展示。
func (s *Service) clientSecretState(ctx context.Context, repo domain.Repository, application *domain.Application, now time.Time) (bool, string, error) {
	secrets, err := repo.ListClientSecrets(ctx, application.OrganizationID, application.ID)
	if err != nil {
		return false, "", err
	}
	for _, secret := range secrets {
		if secret.UsableAt(now) {
			return true, secret.Masked(), nil
		}
	}
	if strings.TrimSpace(application.ClientSecretHash) != "" {
		return true, "", nil
	}
	return false, "", nil
}
