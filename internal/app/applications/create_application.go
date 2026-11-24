package applications

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) CreateApplication(ctx context.Context, command CreateApplicationCommand) (*CreateApplicationResult, error) {
	if err := validateCreateApplicationCommand(command); err != nil {
		return nil, err
	}

	parsedJWKS, err := parseOptionalClientJWKS(command.JWKS)
	if err != nil {
		return nil, err
	}
	jwksURI := strings.TrimSpace(command.JWKSURI)
	if err := validateClientJWKSURI(jwksURI); err != nil {
		return nil, err
	}

	application, clientSecret, err := s.NewApplicationWithSecret(command)
	if err != nil {
		return nil, err
	}
	effectiveMethod := application.EffectiveTokenEndpointAuthMethod()
	if effectiveMethod == domain.TokenEndpointAuthMethodPrivateKeyJWT && len(parsedJWKS) == 0 && jwksURI == "" {
		return nil, jwksRequiredError(domain.TokenEndpointAuthMethodPrivateKeyJWT)
	}
	if isMTLSMethod(effectiveMethod) {
		if len(command.ClientCertificates) == 0 {
			return nil, clientCertificatesRequiredError(effectiveMethod)
		}
	} else if len(command.ClientCertificates) > 0 {
		return nil, apperrors.NewUnprocessableEntity(
			"clientCertificates are only accepted for tls_client_auth or self_signed_tls_client_auth",
		)
	}

	// 新表是唯一写路径：secret 行与 application 同事务创建，legacy 列保持空值。
	var secretPrefix string
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		_, err := tx.Applications().GetByOrganizationAndSlug(ctx, application.OrganizationID, application.Slug)
		if err == nil {
			return apperrors.NewAlreadyExists("application slug already exists in organization")
		}
		if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeNotFound {
			return err
		}

		// Check for soft-deleted application with the same org+slug.
		// The DB unique constraint on (organization_id, slug) does not include
		// deleted_at, so a soft-deleted record would cause a constraint
		// violation on INSERT.
		hasDeleted, err := tx.Applications().HasDeletedWithOrgAndSlug(ctx, application.OrganizationID, application.Slug)
		if err != nil {
			return err
		}
		if hasDeleted {
			return apperrors.NewAlreadyExists("application slug already exists in organization (soft-deleted); permanently delete the existing application or use a different slug")
		}

		if err := tx.Applications().Create(ctx, application); err != nil {
			return err
		}

		// 新表是唯一写路径：secret 行与 application 同事务创建，legacy 列保持空值。
		if clientSecret != nil {
			row, err := s.buildClientSecretRow(application.ID, *clientSecret, s.Clock.Now())
			if err != nil {
				return err
			}
			if err := tx.Applications().CreateClientSecret(ctx, application.OrganizationID, row); err != nil {
				return err
			}
			secretPrefix = row.SecretPrefix
		}

		if jwksURI != "" {
			uriRepo, ok := tx.Applications().(domain.ClientJWKSURIRepository)
			if !ok {
				return apperrors.NewInternal("client jwks_uri repository is not configured")
			}
			if err := uriRepo.SetClientJWKSURI(ctx, application.OrganizationID, application.ID, jwksURI, s.Clock.Now()); err != nil {
				return err
			}
		}
		// 公钥与 application 同事务写入；失败即整体回滚，不会留下“应用存在但
		// 没有可用验签密钥”的半成品。
		if len(parsedJWKS) > 0 {
			now := s.Clock.Now()
			rows, err := s.buildClientJWKRows(application.ID, application.OrganizationID, parsedJWKS, now)
			if err != nil {
				return err
			}
			if err := tx.Applications().ReplaceClientJWKs(ctx, application.OrganizationID, application.ID, rows, now); err != nil {
				return err
			}
		}

		// mTLS 证书与 application 同事务写入，避免“认证方式已选 mTLS，但证书
		// 还不存在”的不可用状态。
		if isMTLSMethod(effectiveMethod) {
			if err := s.replaceClientCertificatesTx(ctx, tx, application, effectiveMethod, command.ClientCertificates, command.ActorID, s.Clock.Now()); err != nil {
				return err
			}
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			application.OrganizationID,
			command.ActorID,
			"application.created",
			application.ID,
			"application.created",
			// 只放可公开的非秘密字段：认证方式、client_id、secret 掩码前缀、
			// 公钥数量。明文 secret、argon2id 哈希与 JWK 内容绝不进入审计/outbox。
			map[string]any{
				"tokenEndpointAuthMethod": string(application.TokenEndpointAuthMethod),
				"clientId":                application.ClientID,
				"clientSecretPrefix":      secretPrefix,
				"jwksKidCount":            len(parsedJWKS),
			},
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := &CreateApplicationResult{Application: application, ClientSecretPrefix: secretPrefix}
	if clientSecret != nil {
		result.ClientSecretPlain = clientSecret.Plain
	}
	return result, nil
}

func parseOptionalClientJWKS(raw []byte) ([]domain.ParsedClientJWK, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	return domain.ParseClientJWKS(raw)
}

func (s *Service) buildClientJWKRows(applicationID, organizationID string, keys []domain.ParsedClientJWK, now time.Time) ([]*domain.ClientJWK, error) {
	rows := make([]*domain.ClientJWK, 0, len(keys))
	for _, key := range keys {
		id, err := s.UUID.NewString()
		if err != nil {
			return nil, apperrors.WrapInternal(err, "generate client jwk id")
		}
		rows = append(rows, &domain.ClientJWK{
			ID:             id,
			ApplicationID:  applicationID,
			OrganizationID: organizationID,
			Kid:            key.Kid,
			Alg:            key.Alg,
			JWK:            key.JWK,
			Status:         domain.ClientJWKStatusActive,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		})
	}
	return rows, nil
}

// jwksRequiredError 是 422：认证方式要求公钥，但本次请求（或合并后的现有配置）
// 没有任何可用公钥。文案只指向可修正的字段，不回显请求内容。
func jwksRequiredError(method string) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"jwks is required when tokenEndpointAuthMethod is private_key_jwt",
		map[string]any{
			"field":    "jwks",
			"value":    method,
			"expected": "non_empty_jwks",
		},
	)
}
