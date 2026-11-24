package applications

import (
	"context"
	"encoding/json"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/platform/config"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PatchApplicationCommand struct {
	ID             string
	OrganizationID string
	// ActorID is the operator user id (empty for system actions).
	ActorID                   string
	Slug                      *string
	DisplayName               *string
	Category                  *string
	AppType                   *string
	Description               *string
	HomepageURL               *string
	Domain                    *string
	Status                    *string
	Logo                      *string
	LogoutUrl                 *string
	LogoutRedirectUrl         *string
	Certificate               *string
	CustomHeader              *string
	EnableSamlCompress        *bool
	ExpireInHours             *int
	RefreshTokenExpireInHours *int
	TokenFormat               *string
	TokenSigningMethod        *string
	ClientID                  *string
	ClientSecret              *string
	TokenEndpointAuthMethod   *string
	// JWKS 是内嵌公钥集合；nil 表示“不修改”，非 nil 时整体替换现有 active 集合。
	JWKS []byte
	// JWKSURI 非 nil 时替换远端 JWKS；空串表示删除。
	JWKSURI *string
	// ClientCertificates 非 nil 时整体替换 mTLS 证书集合；nil 表示“不修改”。
	ClientCertificates *[]ClientCertificateInput
	RedirectURIs       *[]string
	GrantTypes         *[]string
	CustomScopes       *[]ApplicationCustomScopeCommand
	CustomCss          *string
	SigninUrl          *string
	Revision           int64
	// Phase 2: Extended fields
	IsShared                          *bool
	Title                             *string
	Favicon                           *string
	MenuOrder                         *int
	MenuMode                          *string
	EnableSignup                      *bool
	EnableExclusiveSignin             *bool
	EnableGuestSignin                 *bool
	EnableAutoSignin                  *bool
	EnableLinkWithEmail               *bool
	CookieExpireInHours               *int
	DisableSignin                     *bool
	EnableSigninSession               *bool
	Tags                              *[]string
	SignupUrl                         *string
	ForgetUrl                         *string
	AffiliationUrl                    *string
	TokenFields                       *string
	ForcedRedirectOrigin              *string
	CodeExpire                        *int
	SamlNameIdFormat                  *string
	SamlNameIdContent                 *string
	SamlSignRequest                   *bool
	SamlMetadataUrl                   *string
	EnableSamlC14n10                  *bool
	EnableSamlPostBinding             *bool
	DisableSamlAttributes             *bool
	EnableSamlAssertionSignature      *bool
	UseEmailAsSamlNameId              *bool
	ConsentMode                       *string
	DpopRequired                      *bool
	PkceRequired                      *bool
	IpWhitelist                       *string
	IpBlacklist                       *string
	FailedSigninLimit                 *int
	FailedSigninFrozenTime            *int
	CodeResendTimeout                 *int
	CorsAllowedOrigins                *string
	EnableBetaOidc                    *bool
	DefaultGroup                      *string
	OrgChoiceMode                     *string
	FormCss                           *string
	FormCssMobile                     *string
	FormSideHtml                      *string
	FormBackgroundUrl                 *string
	FormBackgroundUrlMobile           *string
	FormOffset                        *string
	FormPosition                      *string
	HeaderHtml                        *string
	FooterHtml                        *string
	SignupHtml                        *string
	SigninHtml                        *string
	TermsOfUse                        *string
	PrivacyPolicy                     *string
	BackchannelLogoutUri              *string
	FrontchannelLogoutUri             *string
	FrontchannelLogoutSessionRequired *bool
	OtherDomains                      *[]string
	UpstreamHost                      *string
	SslMode                           *string
	SslCert                           *string
	ClientCert                        *string
	SamlHashAlgorithm                 *string
	SamlC14nPrefix                    *string
	PageHtml                          *string
	// Nested configuration edited in the console. Transport must not import the
	// domain layer, so these arrive as raw JSON and are decoded here. nil means
	// "not submitted"; an explicit empty array means "clear".
	IsPortal        *bool           `json:"isPortal"`
	IsVisible       *bool           `json:"isVisible"`
	PromptUrl       *string         `json:"promptUrl"`
	SamlReplyUrl    *string         `json:"samlReplyUrl"`
	SamlMetadata    *string         `json:"samlMetadata"`
	ProviderItems   json.RawMessage `json:"providerItems"`
	SigninMethods   json.RawMessage `json:"signinMethods"`
	SigninItems     json.RawMessage `json:"signinItems"`
	SignupItems     json.RawMessage `json:"signupItems"`
	SamlAttributes  json.RawMessage `json:"samlAttributes"`
	TokenAttributes json.RawMessage `json:"tokenAttributes"`
}

// decodeApplicationConfigList turns a raw JSON array into a domain slice while
// preserving the "submitted but empty" case, which must clear the stored value.
func decodeApplicationConfigList[T any](raw json.RawMessage, field string) ([]T, error) {
	var items []T
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, apperrors.NewUnprocessableEntity(field + " must be an array of objects")
	}
	if items == nil {
		items = make([]T, 0)
	}
	return items, nil
}

// PatchApplication 保持既有签名，供只需要成功/失败语义的调用方使用。
func (s *Service) PatchApplication(ctx context.Context, command PatchApplicationCommand) error {
	_, err := s.patchApplication(ctx, command)
	return err
}

// PatchApplicationWithRevision 在保存成功后返回落库后的 revision。
// 前端用它立即同步乐观锁基线，避免异步 refetch 完成前再次保存触发假冲突。
func (s *Service) PatchApplicationWithRevision(ctx context.Context, command PatchApplicationCommand) (int64, error) {
	return s.patchApplication(ctx, command)
}

func (s *Service) patchApplication(ctx context.Context, command PatchApplicationCommand) (int64, error) {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return 0, err
	}
	if err := validatePatchApplicationCommand(command); err != nil {
		return 0, err
	}

	parsedJWKS, parseErr := parseOptionalClientJWKS(command.JWKS)
	if parseErr == nil && command.JWKSURI != nil {
		parseErr = validateClientJWKSURI(*command.JWKSURI)
	}
	if parseErr != nil {
		return 0, parseErr
	}

	var savedRevision int64
	// PATCH 事务内先按租户读取，再做写入；Update 另以实体 OrganizationID 作为谓词。
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		application, err := tx.Applications().GetByIDForOrganization(ctx, command.ID, command.OrganizationID)
		if err != nil {
			return err
		}
		// 只校验本次真正变更的 SAML 值，历史行保持可保存（见 validateApplicationSamlWrite）。
		if err := validateApplicationSamlWrite(application, command); err != nil {
			return err
		}
		// 反向代理字段同样只校验本次变更，空值一律放行。
		if err := validateApplicationReverseProxyWrite(application, command); err != nil {
			return err
		}
		// R3d：同组织内的 host 唯一性需要查库，沿用「变化才校验」的同一口径。
		if err := validateReverseProxyHostUniqueness(ctx, tx.Applications(), application, command); err != nil {
			return err
		}
		now := s.Clock.Now()
		usableSecrets, err := tx.Applications().ListUsableClientSecrets(ctx, application.OrganizationID, application.ID, now)
		if err != nil {
			return err
		}
		// 捕获原始配置的 OIDC 一致性状态 —— 用于 PATCH 验证"本次变更是否引入了新的不合法"
		origMethod := application.EffectiveTokenEndpointAuthMethod()
		origNeedsAuth := origMethod != domain.TokenEndpointAuthMethodNone
		origHasSecret := len(usableSecrets) > 0 || application.ClientSecretHash != ""
		origValidOIDC := !origNeedsAuth || origHasSecret

		updated := cloneApplication(application)

		if command.Revision > 0 && updated.Revision != command.Revision {
			return apperrors.NewConflict("application revision mismatch")
		}

		if command.Slug != nil {
			updated.Slug = normalizeOptionalString(*command.Slug)
		}
		if command.DisplayName != nil {
			updated.DisplayName = *command.DisplayName
		}
		if command.Category != nil {
			updated.Category = *command.Category
		}
		if command.AppType != nil {
			updated.AppType = *command.AppType
		}
		if command.Description != nil {
			updated.Description = *command.Description
		}
		if command.HomepageURL != nil {
			updated.HomepageURL = *command.HomepageURL
		}
		if command.Domain != nil {
			updated.Domain = *command.Domain
		}
		if command.Status != nil {
			updated.Status = *command.Status
		}
		// ClientID is IMMUTABLE per RFC 6749 — once issued, it must never change.
		// The field must NOT appear in PATCH requests at all. If the caller sends
		// it (even as empty string or identical value), reject with 422 — this is
		// the strongest possible defense against accidental or malicious mutation.
		if command.ClientID != nil {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "clientId is immutable and cannot be changed; omit it from PATCH", map[string]any{metaKeyField: "clientId"})
		}
		var pendingSecret *ClientSecretMaterial
		if command.ClientSecret != nil {
			secret := normalizeOptionalString(*command.ClientSecret)
			if secret != "" {
				hash, err := platformcrypto.HashClientSecret(secret)
				if err != nil {
					return err
				}
				pendingSecret = &ClientSecretMaterial{Plain: secret, Hash: hash}
			}
			// 空字符串 = 前端表单默认值（编辑时敏感字段不回显）→ 视为"不修改"。
			// 清除 secret 必须通过显式的 dedicated endpoint，P2 不支持。
			// legacy 列不再写入（TODO(deprecate)）；新值由 app_client_secrets 同事务落库。
		}
		if command.TokenEndpointAuthMethod != nil {
			updated.TokenEndpointAuthMethod = normalizeTokenEndpointAuthMethod(*command.TokenEndpointAuthMethod, derefString(command.ClientSecret))
		} else if command.ClientSecret != nil {
			updated.TokenEndpointAuthMethod = normalizeTokenEndpointAuthMethod(updated.TokenEndpointAuthMethod, derefString(command.ClientSecret))
		}
		if command.RedirectURIs != nil {
			updated.RedirectURIs = normalizeStringList(*command.RedirectURIs)
		}
		if command.GrantTypes != nil {
			updated.GrantTypes = normalizeStringList(*command.GrantTypes)
		}
		if command.CustomScopes != nil {
			updated.CustomScopes = toDomainCustomScopes(*command.CustomScopes)
		}
		if command.CustomCss != nil {
			updated.CustomCss = *command.CustomCss
		}
		if command.SigninUrl != nil {
			updated.SigninUrl = *command.SigninUrl
		}

		// Legacy / Phase 0 fields that were missing from PatchCommand.
		if command.Logo != nil {
			updated.Logo = *command.Logo
		}
		if command.LogoutUrl != nil {
			updated.LogoutUrl = *command.LogoutUrl
		}
		if command.LogoutRedirectUrl != nil {
			updated.LogoutRedirectUrl = *command.LogoutRedirectUrl
		}
		if command.Certificate != nil {
			updated.Certificate = *command.Certificate
		}
		if command.CustomHeader != nil {
			updated.CustomHeader = *command.CustomHeader
		}
		if command.EnableSamlCompress != nil {
			updated.EnableSamlCompress = *command.EnableSamlCompress
		}
		if command.ExpireInHours != nil {
			updated.ExpireInHours = *command.ExpireInHours
		}
		if command.RefreshTokenExpireInHours != nil {
			updated.RefreshTokenExpireInHours = *command.RefreshTokenExpireInHours
		}
		if command.TokenFormat != nil {
			updated.TokenFormat = *command.TokenFormat
		}
		if command.TokenSigningMethod != nil {
			updated.TokenSigningMethod = normalizeTokenSigningMethod(*command.TokenSigningMethod)
		}

		// Phase 2: Extended field patching
		if command.IsShared != nil {
			updated.IsShared = *command.IsShared
		}
		if command.Title != nil {
			updated.Title = *command.Title
		}
		if command.Favicon != nil {
			updated.Favicon = *command.Favicon
		}
		if command.MenuOrder != nil {
			updated.MenuOrder = *command.MenuOrder
		}
		if command.MenuMode != nil {
			updated.MenuMode = *command.MenuMode
		}
		if command.EnableSignup != nil {
			updated.EnableSignup = *command.EnableSignup
		}
		if command.EnableExclusiveSignin != nil {
			updated.EnableExclusiveSignin = *command.EnableExclusiveSignin
		}
		if command.EnableGuestSignin != nil {
			updated.EnableGuestSignin = *command.EnableGuestSignin
		}
		if command.EnableAutoSignin != nil {
			updated.EnableAutoSignin = *command.EnableAutoSignin
		}
		if command.EnableLinkWithEmail != nil {
			updated.EnableLinkWithEmail = *command.EnableLinkWithEmail
		}
		if command.DisableSignin != nil {
			updated.DisableSignin = *command.DisableSignin
		}
		if command.EnableSigninSession != nil {
			updated.EnableSigninSession = *command.EnableSigninSession
		}
		if command.Tags != nil {
			updated.Tags = append([]string(nil), *command.Tags...)
		}
		if command.CookieExpireInHours != nil {
			updated.CookieExpireInHours = *command.CookieExpireInHours
		}
		if command.SignupUrl != nil {
			updated.SignupUrl = *command.SignupUrl
		}
		if command.ForgetUrl != nil {
			updated.ForgetUrl = *command.ForgetUrl
		}
		if command.AffiliationUrl != nil {
			updated.AffiliationUrl = *command.AffiliationUrl
		}
		if command.TokenFields != nil {
			updated.TokenFields = *command.TokenFields
		}
		if command.ForcedRedirectOrigin != nil {
			updated.ForcedRedirectOrigin = *command.ForcedRedirectOrigin
		}
		if command.CodeExpire != nil {
			updated.CodeExpire = *command.CodeExpire
		}
		if command.SamlNameIdFormat != nil {
			updated.SamlNameIdFormat = *command.SamlNameIdFormat
		}
		if command.SamlNameIdContent != nil {
			updated.SamlNameIdContent = *command.SamlNameIdContent
		}
		if command.SamlSignRequest != nil {
			updated.SamlSignRequest = *command.SamlSignRequest
		}
		if command.SamlMetadataUrl != nil {
			updated.SamlMetadataUrl = *command.SamlMetadataUrl
		}
		if command.EnableSamlC14n10 != nil {
			updated.EnableSamlC14n10 = *command.EnableSamlC14n10
		}
		if command.EnableSamlPostBinding != nil {
			updated.EnableSamlPostBinding = *command.EnableSamlPostBinding
		}
		if command.DisableSamlAttributes != nil {
			updated.DisableSamlAttributes = *command.DisableSamlAttributes
		}
		if command.EnableSamlAssertionSignature != nil {
			updated.EnableSamlAssertionSignature = *command.EnableSamlAssertionSignature
		}
		if command.UseEmailAsSamlNameId != nil {
			updated.UseEmailAsSamlNameId = *command.UseEmailAsSamlNameId
		}
		if command.ConsentMode != nil {
			updated.ConsentMode = *command.ConsentMode
		}
		if command.DpopRequired != nil {
			updated.DpopRequired = *command.DpopRequired
		}
		if command.PkceRequired != nil {
			updated.PkceRequired = *command.PkceRequired
		}
		if command.IpWhitelist != nil {
			updated.IpWhitelist = *command.IpWhitelist
		}
		if command.IpBlacklist != nil {
			updated.IpBlacklist = *command.IpBlacklist
		}
		if command.FailedSigninLimit != nil {
			updated.FailedSigninLimit = *command.FailedSigninLimit
		}
		if command.FailedSigninFrozenTime != nil {
			updated.FailedSigninFrozenTime = *command.FailedSigninFrozenTime
		}
		if command.CodeResendTimeout != nil {
			updated.CodeResendTimeout = *command.CodeResendTimeout
		}
		if command.CorsAllowedOrigins != nil {
			updated.CorsAllowedOrigins = *command.CorsAllowedOrigins
		}
		if command.EnableBetaOidc != nil {
			updated.EnableBetaOidc = *command.EnableBetaOidc
		}
		if command.DefaultGroup != nil {
			updated.DefaultGroup = *command.DefaultGroup
		}
		if command.OrgChoiceMode != nil {
			updated.OrgChoiceMode = *command.OrgChoiceMode
		}
		if command.FormCss != nil {
			updated.FormCss = *command.FormCss
		}
		if command.FormCssMobile != nil {
			updated.FormCssMobile = *command.FormCssMobile
		}
		if command.FormSideHtml != nil {
			updated.FormSideHtml = *command.FormSideHtml
		}
		if command.FormBackgroundUrl != nil {
			updated.FormBackgroundUrl = *command.FormBackgroundUrl
		}
		if command.FormBackgroundUrlMobile != nil {
			updated.FormBackgroundUrlMobile = *command.FormBackgroundUrlMobile
		}
		if command.FormOffset != nil {
			updated.FormOffset = *command.FormOffset
		}
		if command.FormPosition != nil {
			updated.FormPosition = *command.FormPosition
		}
		if command.HeaderHtml != nil {
			updated.HeaderHtml = *command.HeaderHtml
		}
		if command.FooterHtml != nil {
			updated.FooterHtml = *command.FooterHtml
		}
		if command.SignupHtml != nil {
			updated.SignupHtml = *command.SignupHtml
		}
		if command.SigninHtml != nil {
			updated.SigninHtml = *command.SigninHtml
		}
		if command.TermsOfUse != nil {
			updated.TermsOfUse = *command.TermsOfUse
		}
		if command.PrivacyPolicy != nil {
			updated.PrivacyPolicy = *command.PrivacyPolicy
		}
		if command.BackchannelLogoutUri != nil {
			updated.BackchannelLogoutUri = *command.BackchannelLogoutUri
		}
		if command.FrontchannelLogoutUri != nil {
			updated.FrontchannelLogoutUri = *command.FrontchannelLogoutUri
		}
		if command.FrontchannelLogoutSessionRequired != nil {
			updated.FrontchannelLogoutSessionRequired = *command.FrontchannelLogoutSessionRequired
		}
		if command.OtherDomains != nil {
			updated.OtherDomains = normalizeStringList(*command.OtherDomains)
		}
		if command.UpstreamHost != nil {
			updated.UpstreamHost = strings.TrimSpace(*command.UpstreamHost)
		}
		if command.SslMode != nil {
			updated.SslMode = strings.TrimSpace(*command.SslMode)
		}
		if command.SslCert != nil {
			updated.SslCert = strings.TrimSpace(*command.SslCert)
		}
		if command.ClientCert != nil {
			updated.ClientCert = *command.ClientCert
		}
		if command.SamlHashAlgorithm != nil {
			updated.SamlHashAlgorithm = *command.SamlHashAlgorithm
		}
		if command.SamlC14nPrefix != nil {
			updated.SamlC14nPrefix = *command.SamlC14nPrefix
		}
		if command.PageHtml != nil {
			updated.PageHtml = *command.PageHtml
		}

		if command.Slug != nil {
			existing, err := tx.Applications().GetByOrganizationAndSlug(ctx, updated.OrganizationID, updated.Slug)
			if err == nil && existing.ID != updated.ID {
				return apperrors.NewConflict("application slug already exists in organization")
			}
			if err != nil {
				appErr, ok := err.(*apperrors.Error)
				if !ok || appErr.Code != apperrors.CodeNotFound {
					return err
				}
			}
		}

		// allowMissingSecretOnCreate: PATCH 路径的"配置可退化性"判断。
		//
		// 后端原则：只有当本次 PATCH **把合法配置变成了不合法配置** 时才拦截。
		// 如果原始就不合法（历史遗留），或者新配置没有比旧配置更差 —— 放行，
		// 让用户能先编辑其他字段（比如 displayName / redirectUri），再回来修 OIDC 配置。
		//
		// worsened := 原始合法 && 新配置不合法
		allowMissingSecretOnCreate := true
		finalNeedsAuth := updated.TokenEndpointAuthMethod != domain.TokenEndpointAuthMethodNone
		newSecretProvided := pendingSecret != nil
		origSecretStillUsable := len(usableSecrets) > 0 || application.ClientSecretHash != ""
		finalHasSecret := newSecretProvided || origSecretStillUsable
		finalValidOIDC := !finalNeedsAuth || finalHasSecret
		worsened := origValidOIDC && !finalValidOIDC
		if worsened {
			allowMissingSecretOnCreate = false
		}

		if err := validateOAuthClientSettings(
			updated.ClientID,
			derefString(command.ClientSecret),
			updated.TokenEndpointAuthMethod,
			updated.RedirectURIs,
			updated.GrantTypes,
			allowMissingSecretOnCreate,
		); err != nil {
			return err
		}

		finalMethod := updated.EffectiveTokenEndpointAuthMethod()
		// none 的授权类型组合只在本次 PATCH 触碰 OIDC 配置时强制：历史遗留的
		// 不可用组合仍允许编辑其他字段，但任何方法/授权类型的修改都必须落到
		// token endpoint 真正能兑换的组合上。
		if command.TokenEndpointAuthMethod != nil || command.GrantTypes != nil {
			if err := validatePublicClientGrantCombination(finalMethod, derefString(command.ClientSecret), updated.GrantTypes); err != nil {
				return err
			}
		}

		// private_key_jwt 的可用性取决于 app_client_jwks，而不是 secret。
		// 只在本次 PATCH 之后仍为 private_key_jwt 时要求至少一把 active 公钥；
		// 未携带 jwks 字段时沿用已登记集合。
		if finalMethod == domain.TokenEndpointAuthMethodPrivateKeyJWT {
			finalHasJWKS := len(parsedJWKS) > 0
			if command.JWKS == nil {
				existing, err := tx.Applications().ListActiveClientJWKs(ctx, updated.OrganizationID, updated.ID)
				if err != nil {
					return err
				}
				finalHasJWKS = len(existing) > 0
			}
			if !finalHasJWKS {
				return jwksRequiredError(finalMethod)
			}
		}

		// mTLS 的可用性取决于 app_client_certificates：切换方法是唯一会把应用
		// 从“可用”变成“不可用”的路径，因此必须同一事务内提交证书集合，或在
		// 已登记集合里找到与目标方式匹配的证书。
		methodChanged := finalMethod != origMethod
		if command.ClientCertificates != nil {
			if !isMTLSMethod(finalMethod) {
				return apperrors.NewUnprocessableEntity(
					"clientCertificates are only accepted for tls_client_auth or self_signed_tls_client_auth",
				)
			}
			if err := s.replaceClientCertificatesTx(ctx, tx, updated, finalMethod, *command.ClientCertificates, command.ActorID, now); err != nil {
				return err
			}
		} else if methodChanged && isMTLSMethod(finalMethod) {
			existing, err := tx.Applications().ListActiveClientCertificates(ctx, updated.OrganizationID, updated.ID)
			if err != nil {
				return err
			}
			usable := false
			for _, certificate := range existing {
				if certificate.AuthMethod == finalMethod {
					usable = true
					break
				}
			}
			if !usable {
				return clientCertificatesRequiredError(finalMethod)
			}
		}

		// 公钥整体替换与 application 更新同事务：失败即回滚，不留下半套密钥。
		if command.JWKSURI != nil {
			uriRepo, ok := tx.Applications().(domain.ClientJWKSURIRepository)
			if !ok {
				return apperrors.NewInternal("client jwks_uri repository is not configured")
			}
			if trimmed := strings.TrimSpace(*command.JWKSURI); trimmed == "" {
				if err := uriRepo.DeleteClientJWKSURI(ctx, updated.OrganizationID, updated.ID); err != nil {
					return err
				}
			} else if err := uriRepo.SetClientJWKSURI(ctx, updated.OrganizationID, updated.ID, trimmed, now); err != nil {
				return err
			}
		}
		if command.JWKS != nil {
			rows, err := s.buildClientJWKRows(updated.ID, updated.OrganizationID, parsedJWKS, now)
			if err != nil {
				return err
			}
			if err := tx.Applications().ReplaceClientJWKs(ctx, updated.OrganizationID, updated.ID, rows, now); err != nil {
				return err
			}
			if err := s.embedAuditEvent(
				tx,
				ctx,
				updated.OrganizationID,
				command.ActorID,
				"application.client_jwks.updated",
				updated.ID,
				"application.client_jwks.updated",
				// 只记录数量与 kid 列表，JWK 公钥内容不进入审计/outbox。
				map[string]any{
					"kidCount": len(parsedJWKS),
					"kids":     parsedClientJWKKids(parsedJWKS),
				},
			); err != nil {
				return err
			}
		}

		// 新表是唯一写路径：PATCH 提供的新 secret 以轮换语义落库，旧行进入宽限期。
		if pendingSecret != nil {
			row, err := s.buildClientSecretRow(updated.ID, *pendingSecret, now)
			if err != nil {
				return err
			}
			if err := tx.Applications().RotateClientSecret(ctx, updated.OrganizationID, updated.ID, row, now, now.Add(config.ClientSecretRotationGrace())); err != nil {
				return err
			}
		}

		// 切换到非 secret 方式（none / JWT / mTLS）时旧 secret 保留但立即停用，
		// 不物理删除，便于回滚与审计。
		if !updated.RequiresClientSecret() && origMethod != updated.EffectiveTokenEndpointAuthMethod() {
			if err := tx.Applications().RetireClientSecrets(ctx, updated.OrganizationID, updated.ID, now); err != nil {
				return err
			}
		}

		// SAML/portal/prompt flags are edited in the console (Basic, SAML and UI
		// tabs); apply them here so the PATCH actually persists what the form shows.
		if command.IsPortal != nil {
			updated.IsPortal = *command.IsPortal
		}
		if command.IsVisible != nil {
			updated.IsVisible = *command.IsVisible
		}
		if command.PromptUrl != nil {
			updated.PromptUrl = *command.PromptUrl
		}
		if command.SamlReplyUrl != nil {
			updated.SamlReplyUrl = *command.SamlReplyUrl
		}
		if command.SamlMetadata != nil {
			updated.SamlMetadata = *command.SamlMetadata
		}

		// 嵌套配置（注册项/登录方式/登录项/提供商项/SAML 与 Token 属性）此前只被
		// 解码、从未写回，导致“保存成功但配置不生效”。nil 表示本次未提交，显式
		// 空数组表示清空，因此这里必须保留 nil/空 的区分。
		if command.ProviderItems != nil {
			items, err := decodeApplicationConfigList[domain.ProviderItem](command.ProviderItems, "providerItems")
			if err != nil {
				return err
			}
			updated.ProviderItems = items
		}
		if command.SigninItems != nil {
			items, err := decodeApplicationConfigList[domain.SigninItemConfig](command.SigninItems, "signinItems")
			if err != nil {
				return err
			}
			updated.SigninItems = items
		}
		if command.SignupItems != nil {
			items, err := decodeApplicationConfigList[domain.SignupItem](command.SignupItems, "signupItems")
			if err != nil {
				return err
			}
			updated.SignupItems = items
		}
		if command.SamlAttributes != nil {
			items, err := decodeApplicationConfigList[domain.SamlAttribute](command.SamlAttributes, "samlAttributes")
			if err != nil {
				return err
			}
			updated.SamlAttributes = items
		}
		if command.TokenAttributes != nil {
			items, err := decodeApplicationConfigList[domain.TokenAttribute](command.TokenAttributes, "tokenAttributes")
			if err != nil {
				return err
			}
			updated.TokenAttributes = items
		}
		if command.SigninMethods != nil {
			var methods []string
			if err := json.Unmarshal(command.SigninMethods, &methods); err != nil {
				return apperrors.NewUnprocessableEntity("signinMethods must be an array of strings")
			}
			names := make([]string, 0, len(methods))
			for _, name := range methods {
				if trimmed := strings.TrimSpace(name); trimmed != "" {
					names = append(names, trimmed)
				}
			}
			updated.SigninMethods = names
		}

		updated.UpdatedAt = s.Clock.Now()
		updated.Revision++

		if err := tx.Applications().Update(ctx, updated); err != nil {
			return err
		}
		// 仓储在 UPDATE ... RETURNING revision 后把真实 revision 写回实体。
		savedRevision = updated.Revision

		if err := s.embedAuditEvent(
			tx,
			ctx,
			updated.OrganizationID,
			command.ActorID,
			"application.updated",
			updated.ID,
			"application.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return savedRevision, nil
}

func cloneApplication(application *domain.Application) *domain.Application {
	if application == nil {
		return nil
	}
	cloned := *application
	cloned.RedirectURIs = append([]string(nil), application.RedirectURIs...)
	cloned.GrantTypes = append([]string(nil), application.GrantTypes...)
	cloned.CustomScopes = append([]domain.ApplicationCustomScope(nil), application.CustomScopes...)
	cloned.ProviderBindings = append([]domain.ApplicationProviderBinding(nil), application.ProviderBindings...)
	cloned.TokenAttributes = append([]domain.TokenAttribute(nil), application.TokenAttributes...)
	cloned.SamlAttributes = append([]domain.SamlAttribute(nil), application.SamlAttributes...)
	cloned.OtherDomains = append([]string(nil), application.OtherDomains...)
	cloned.SigninMethods = append([]string(nil), application.SigninMethods...)
	cloned.SigninItems = append([]domain.SigninItemConfig(nil), application.SigninItems...)
	cloned.SignupItems = append([]domain.SignupItem(nil), application.SignupItems...)
	cloned.ProviderItems = append([]domain.ProviderItem(nil), application.ProviderItems...)
	return &cloned
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func parsedClientJWKKids(keys []domain.ParsedClientJWK) []string {
	kids := make([]string, 0, len(keys))
	for _, key := range keys {
		kids = append(kids, key.Kid)
	}
	return kids
}
