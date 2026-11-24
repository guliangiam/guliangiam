package applications

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ExportVersion is the current version of the export format.
// ExportVersion 1.2 removes the duplicated sign-in boolean flags; signinMethods is
// the only source. 1.1 exported the effective token endpoint auth method so legacy
// rows with an empty stored method do not import as public clients.
const ExportVersion = "1.2"

// ExportVersionLegacy is the pre-1.1 format: still accepted on import, but an
// empty tokenEndpointAuthMethod in it means "unknown", not "public".
const ExportVersionLegacy = "1.0"

// ExportVersionSigninMethods is the 1.1 format (still accepted on import).
const ExportVersionSigninMethods = "1.1"

// ExportPayload represents the JSON structure for an exported application configuration.
type ExportPayload struct {
	Version    string               `json:"version"`
	ExportedAt time.Time            `json:"exportedAt"`
	Export     ApplicationExportDTO `json:"export"`
}

// ApplicationExportDTO contains the exportable fields of an application.
// Sensitive fields (clientSecret, signingKey) are excluded.
type ApplicationExportDTO struct {
	// Identity
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	Category    string `json:"category"`
	AppType     string `json:"appType"`
	Description string `json:"description"`
	HomepageURL string `json:"homepageUrl"`
	Domain      string `json:"domain"`
	Logo        string `json:"logo"`
	Favicon     string `json:"favicon"`
	Title       string `json:"title"`
	IsPortal    bool   `json:"isPortal"`
	IsVisible   bool   `json:"isVisible"`
	IsShared    bool   `json:"isShared"`
	MenuOrder   int    `json:"menuOrder"`
	MenuMode    string `json:"menuMode"`
	Status      string `json:"status"`

	// OAuth/OIDC
	ClientID                  string                       `json:"clientId"`
	TokenEndpointAuthMethod   string                       `json:"tokenEndpointAuthMethod"`
	RedirectURIs              []string                     `json:"redirectUris"`
	GrantTypes                []string                     `json:"grantTypes"`
	TokenFormat               string                       `json:"tokenFormat"`
	ExpireInHours             int                          `json:"expireInHours"`
	RefreshTokenExpireInHours int                          `json:"refreshTokenExpireInHours"`
	TokenSigningMethod        string                       `json:"tokenSigningMethod"`
	CustomScopes              []ApplicationCustomScopeItem `json:"customScopes"`

	// Feature toggles
	EnableSamlCompress    bool `json:"enableSamlCompress"`
	EnableSignup          bool `json:"enableSignup"`
	EnableExclusiveSignin bool `json:"enableExclusiveSignin"`
	EnableGuestSignin     bool `json:"enableGuestSignin"`
	EnableAutoSignin      bool `json:"enableAutoSignin"`
	EnableLinkWithEmail   bool `json:"enableLinkWithEmail"`

	// Session
	CookieExpireInHours int `json:"cookieExpireInHours"`

	// URLs
	SigninUrl                         string `json:"signinUrl"`
	LogoutUrl                         string `json:"logoutUrl"`
	LogoutRedirectUrl                 string `json:"logoutRedirectUrl"`
	SignupUrl                         string `json:"signupUrl"`
	ForgetUrl                         string `json:"forgetUrl"`
	AffiliationUrl                    string `json:"affiliationUrl"`
	PromptUrl                         string `json:"promptUrl"`
	BackchannelLogoutUri              string `json:"backchannelLogoutUri"`
	FrontchannelLogoutUri             string `json:"frontchannelLogoutUri"`
	FrontchannelLogoutSessionRequired bool   `json:"frontchannelLogoutSessionRequired"`

	// SAML
	SamlReplyUrl                 string                 `json:"samlReplyUrl"`
	SamlMetadata                 string                 `json:"samlMetadata"`
	SamlMetadataUrl              string                 `json:"samlMetadataUrl"`
	SamlNameIdFormat             string                 `json:"samlNameIdFormat"`
	SamlNameIdContent            string                 `json:"samlNameIdContent"`
	SamlSignRequest              bool                   `json:"samlSignRequest"`
	SamlAttributes               []domain.SamlAttribute `json:"samlAttributes"`
	EnableSamlC14n10             bool                   `json:"enableSamlC14n10"`
	EnableSamlPostBinding        bool                   `json:"enableSamlPostBinding"`
	DisableSamlAttributes        bool                   `json:"disableSamlAttributes"`
	EnableSamlAssertionSignature bool                   `json:"enableSamlAssertionSignature"`
	UseEmailAsSamlNameId         bool                   `json:"useEmailAsSamlNameId"`

	// Security
	ConsentMode            string `json:"consentMode"`
	DpopRequired           bool   `json:"dpopRequired"`
	PkceRequired           bool   `json:"pkceRequired"`
	IpWhitelist            string `json:"ipWhitelist"`
	IpBlacklist            string `json:"ipBlacklist"`
	FailedSigninLimit      int    `json:"failedSigninLimit"`
	FailedSigninFrozenTime int    `json:"failedSigninFrozenTime"`
	CodeResendTimeout      int    `json:"codeResendTimeout"`
	CorsAllowedOrigins     string `json:"corsAllowedOrigins"`
	DefaultGroup           string `json:"defaultGroup"`
	OrgChoiceMode          string `json:"orgChoiceMode"`

	// Token customization
	TokenFields          string                  `json:"tokenFields"`
	TokenAttributes      []domain.TokenAttribute `json:"tokenAttributes"`
	ForcedRedirectOrigin string                  `json:"forcedRedirectOrigin"`
	CodeExpire           int                     `json:"codeExpire"`

	// UI Customization
	FormCss                 string `json:"formCss"`
	FormCssMobile           string `json:"formCssMobile"`
	FormSideHtml            string `json:"formSideHtml"`
	FormBackgroundUrl       string `json:"formBackgroundUrl"`
	FormBackgroundUrlMobile string `json:"formBackgroundUrlMobile"`
	FormOffset              string `json:"formOffset"`
	FormPosition            string `json:"formPosition"`
	CustomCss               string `json:"customCss"`
	CustomHeader            string `json:"customHeader"`
	HeaderHtml              string `json:"headerHtml"`
	FooterHtml              string `json:"footerHtml"`
	SignupHtml              string `json:"signupHtml"`
	SigninHtml              string `json:"signinHtml"`
	TermsOfUse              string `json:"termsOfUse"`
	PrivacyPolicy           string `json:"privacyPolicy"`

	// SSL/TLS
	SslMode           string `json:"sslMode"`
	SslCert           string `json:"sslCert"`
	ClientCert        string `json:"clientCert"`
	SamlHashAlgorithm string `json:"samlHashAlgorithm"`
	SamlC14nPrefix    string `json:"samlC14nPrefix"`
	PageHtml          string `json:"pageHtml"`

	// Other
	UpstreamHost   string   `json:"upstreamHost"`
	OtherDomains   []string `json:"otherDomains"`
	EnableBetaOidc bool     `json:"enableBetaOidc"`

	// Collections
	SignupItems   []domain.SignupItem       `json:"signupItems"`
	SigninMethods []string                  `json:"signinMethods"`
	SigninItems   []domain.SigninItemConfig `json:"signinItems"`
	ProviderItems []domain.ProviderItem     `json:"providerItems"`
	Tags          []string                  `json:"tags"`
}

// ImportPayload represents the JSON structure for importing an application.
type ImportPayload struct {
	OrganizationID string `json:"organizationId"`
	// ActorID is the operator user id (empty for system actions); it only feeds
	// the audit event and is not part of the export document.
	ActorID string               `json:"-"`
	Version string               `json:"version"`
	Export  ApplicationExportDTO `json:"export"`
}

// ExportApplication exports an application configuration as a JSON payload.
// Sensitive fields (clientSecret, signingKey, certificate) are excluded.
func (s *Service) ExportApplication(ctx context.Context, organizationID, id string) (*ExportPayload, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}

	app, err := s.UOW.Applications().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		// 保留 NotFound 语义：跨租户 id 与不存在都必须返回 404，而不是包装成 500。
		if apperrors.FromError(err).Code == apperrors.CodeNotFound {
			return nil, err
		}
		return nil, apperrors.WrapInternal(err, "get application for export")
	}

	return &ExportPayload{
		Version:    ExportVersion,
		ExportedAt: s.Clock.Now(),
		Export:     toExportDTO(app),
	}, nil
}

// ImportApplication creates a new application from an exported configuration.
// It generates a new clientID and clientSecret, and validates the slug is unique.
func (s *Service) ImportApplication(ctx context.Context, payload ImportPayload) (*domain.Application, error) {
	if payload.OrganizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if payload.Export.Slug == "" {
		return nil, apperrors.NewInvalidArgument("export.slug is required")
	}
	if version := strings.TrimSpace(payload.Version); version != "" && version != ExportVersion && version != ExportVersionSigninMethods && version != ExportVersionLegacy {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"unsupported export version",
			map[string]any{"field": "version", "value": version, "supported": []string{ExportVersionLegacy, ExportVersionSigninMethods, ExportVersion}})
	}

	// Import re-hydrates an export document, but it lands in the same columns
	// Create validates, so it enforces the console's ceilings before the first
	// repository call instead of failing at the INSERT. Only those ceilings:
	// columns declared as TEXT (slug, homepageUrl, …) stay unbounded on purpose.
	// The redirect list is normalized first because the normalized values are
	// the ones the create path would have stored.
	redirectURIs := normalizeStringList(payload.Export.RedirectURIs)
	if err := validateApplicationTextLength("displayName", payload.Export.DisplayName); err != nil {
		return nil, err
	}
	if err := validateApplicationTextLength("description", payload.Export.Description); err != nil {
		return nil, err
	}
	if err := validateRedirectURILimits("redirectUris", redirectURIs); err != nil {
		return nil, err
	}
	// Blank normalizes to RS256, but an out-of-enum method would be stored and
	// could never sign a token, so it is rejected here as on create/patch.
	if err := validateTokenSigningMethod(payload.Export.TokenSigningMethod); err != nil {
		return nil, err
	}
	// Import lands directly in the repository without the create path, so the
	// public-client grant rule has to be applied here as well; otherwise an
	// export carrying none + client_credentials becomes an unusable client.
	if err := validatePublicClientGrantCombination(payload.Export.TokenEndpointAuthMethod, "", payload.Export.GrantTypes); err != nil {
		return nil, err
	}
	// The export DTO carries no client material, so private_key_jwt and both
	// mTLS methods would import as clients the token endpoint can never
	// authenticate; out-of-enum values (client_secret_jwt) are rejected too.
	authMethod, err := normalizeImportTokenEndpointAuthMethod(payload.Export.TokenEndpointAuthMethod)
	if err != nil {
		return nil, err
	}
	// SAML 与反向代理字段同样落在 Create/Patch 校验过的列上，导入必须复用同一批
	// 校验函数（长度/格式/取值），否则手工编辑的导出文档能写入 PATCH 会拒绝的值。
	// 这里只做单字段校验；跨字段组合约束（R1–R4）需要持久值，不属于导入范围。
	if err := validateSamlMetadata(payload.Export.SamlMetadata); err != nil {
		return nil, err
	}
	if err := validateSamlMetadataURL(payload.Export.SamlMetadataUrl); err != nil {
		return nil, err
	}
	if err := validateSslMode(payload.Export.SslMode); err != nil {
		return nil, err
	}
	if err := validateSslCert(payload.Export.SslCert); err != nil {
		return nil, err
	}
	if err := validateUpstreamHost(payload.Export.UpstreamHost); err != nil {
		return nil, err
	}
	if err := validateOtherDomains(payload.Export.OtherDomains); err != nil {
		return nil, err
	}

	// Check slug uniqueness within the organization
	existing, _, err := s.UOW.Applications().List(ctx, domain.ListQuery{
		OrganizationID: payload.OrganizationID,
		Keyword:        payload.Export.Slug,
		PageSize:       1,
	})
	if err != nil {
		return nil, apperrors.WrapInternal(err, "check slug uniqueness")
	}
	for _, app := range existing {
		if app.Slug == payload.Export.Slug {
			return nil, apperrors.NewConflictf("application with slug %q already exists in this organization", payload.Export.Slug)
		}
	}

	// Build domain application from export DTO
	app := &domain.Application{
		OrganizationID:                    payload.OrganizationID,
		Slug:                              payload.Export.Slug,
		DisplayName:                       payload.Export.DisplayName,
		Category:                          payload.Export.Category,
		AppType:                           payload.Export.AppType,
		Description:                       payload.Export.Description,
		HomepageURL:                       payload.Export.HomepageURL,
		Domain:                            payload.Export.Domain,
		Logo:                              payload.Export.Logo,
		Favicon:                           payload.Export.Favicon,
		Title:                             payload.Export.Title,
		IsPortal:                          payload.Export.IsPortal,
		IsVisible:                         payload.Export.IsVisible,
		IsShared:                          payload.Export.IsShared,
		MenuOrder:                         payload.Export.MenuOrder,
		MenuMode:                          payload.Export.MenuMode,
		Status:                            payload.Export.Status,
		TokenEndpointAuthMethod:           authMethod,
		RedirectURIs:                      redirectURIs,
		GrantTypes:                        payload.Export.GrantTypes,
		TokenFormat:                       payload.Export.TokenFormat,
		ExpireInHours:                     payload.Export.ExpireInHours,
		RefreshTokenExpireInHours:         payload.Export.RefreshTokenExpireInHours,
		TokenSigningMethod:                normalizeTokenSigningMethod(payload.Export.TokenSigningMethod),
		EnableSamlCompress:                payload.Export.EnableSamlCompress,
		EnableSignup:                      payload.Export.EnableSignup,
		EnableExclusiveSignin:             payload.Export.EnableExclusiveSignin,
		EnableGuestSignin:                 payload.Export.EnableGuestSignin,
		EnableAutoSignin:                  payload.Export.EnableAutoSignin,
		EnableLinkWithEmail:               payload.Export.EnableLinkWithEmail,
		CookieExpireInHours:               payload.Export.CookieExpireInHours,
		SigninUrl:                         payload.Export.SigninUrl,
		LogoutUrl:                         payload.Export.LogoutUrl,
		LogoutRedirectUrl:                 payload.Export.LogoutRedirectUrl,
		SignupUrl:                         payload.Export.SignupUrl,
		ForgetUrl:                         payload.Export.ForgetUrl,
		AffiliationUrl:                    payload.Export.AffiliationUrl,
		PromptUrl:                         payload.Export.PromptUrl,
		BackchannelLogoutUri:              payload.Export.BackchannelLogoutUri,
		FrontchannelLogoutUri:             payload.Export.FrontchannelLogoutUri,
		FrontchannelLogoutSessionRequired: payload.Export.FrontchannelLogoutSessionRequired,
		SamlReplyUrl:                      payload.Export.SamlReplyUrl,
		SamlMetadata:                      payload.Export.SamlMetadata,
		SamlMetadataUrl:                   payload.Export.SamlMetadataUrl,
		SamlNameIdFormat:                  payload.Export.SamlNameIdFormat,
		SamlNameIdContent:                 payload.Export.SamlNameIdContent,
		SamlSignRequest:                   payload.Export.SamlSignRequest,
		SamlAttributes:                    payload.Export.SamlAttributes,
		EnableSamlC14n10:                  payload.Export.EnableSamlC14n10,
		EnableSamlPostBinding:             payload.Export.EnableSamlPostBinding,
		DisableSamlAttributes:             payload.Export.DisableSamlAttributes,
		EnableSamlAssertionSignature:      payload.Export.EnableSamlAssertionSignature,
		UseEmailAsSamlNameId:              payload.Export.UseEmailAsSamlNameId,
		ConsentMode:                       payload.Export.ConsentMode,
		DpopRequired:                      payload.Export.DpopRequired,
		PkceRequired:                      payload.Export.PkceRequired,
		IpWhitelist:                       payload.Export.IpWhitelist,
		IpBlacklist:                       payload.Export.IpBlacklist,
		FailedSigninLimit:                 payload.Export.FailedSigninLimit,
		FailedSigninFrozenTime:            payload.Export.FailedSigninFrozenTime,
		CodeResendTimeout:                 payload.Export.CodeResendTimeout,
		CorsAllowedOrigins:                payload.Export.CorsAllowedOrigins,
		DefaultGroup:                      payload.Export.DefaultGroup,
		OrgChoiceMode:                     payload.Export.OrgChoiceMode,
		TokenFields:                       payload.Export.TokenFields,
		TokenAttributes:                   payload.Export.TokenAttributes,
		ForcedRedirectOrigin:              payload.Export.ForcedRedirectOrigin,
		CodeExpire:                        payload.Export.CodeExpire,
		FormCss:                           payload.Export.FormCss,
		FormCssMobile:                     payload.Export.FormCssMobile,
		FormSideHtml:                      payload.Export.FormSideHtml,
		FormBackgroundUrl:                 payload.Export.FormBackgroundUrl,
		FormBackgroundUrlMobile:           payload.Export.FormBackgroundUrlMobile,
		FormOffset:                        payload.Export.FormOffset,
		FormPosition:                      payload.Export.FormPosition,
		CustomCss:                         payload.Export.CustomCss,
		CustomHeader:                      payload.Export.CustomHeader,
		HeaderHtml:                        payload.Export.HeaderHtml,
		FooterHtml:                        payload.Export.FooterHtml,
		SignupHtml:                        payload.Export.SignupHtml,
		SigninHtml:                        payload.Export.SigninHtml,
		TermsOfUse:                        payload.Export.TermsOfUse,
		PrivacyPolicy:                     payload.Export.PrivacyPolicy,
		SslMode:                           payload.Export.SslMode,
		SslCert:                           payload.Export.SslCert,
		ClientCert:                        payload.Export.ClientCert,
		SamlHashAlgorithm:                 payload.Export.SamlHashAlgorithm,
		SamlC14nPrefix:                    payload.Export.SamlC14nPrefix,
		PageHtml:                          payload.Export.PageHtml,
		UpstreamHost:                      payload.Export.UpstreamHost,
		OtherDomains:                      payload.Export.OtherDomains,
		EnableBetaOidc:                    payload.Export.EnableBetaOidc,
		SignupItems:                       payload.Export.SignupItems,
		SigninMethods:                     payload.Export.SigninMethods,
		SigninItems:                       payload.Export.SigninItems,
		ProviderItems:                     payload.Export.ProviderItems,
		Tags:                              payload.Export.Tags,
	}

	// Generate new ID, ClientID
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate application ID")
	}
	clientID, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate client ID")
	}
	app.ID = id
	app.ClientID = clientID

	// Create the application and its audit event in the same transaction so an
	// import never leaves an application without a matching audit record.
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Applications().Create(ctx, app); err != nil {
			return err
		}
		return s.embedAuditEvent(
			tx,
			ctx,
			app.OrganizationID,
			payload.ActorID,
			"application.created",
			app.ID,
			"application.created",
			// Only the imported application's identity and origin: the export
			// document may carry UI/HTML content and is never copied into audit.
			map[string]any{
				"slug":          app.Slug,
				"displayName":   app.DisplayName,
				"source":        "import",
				"exportVersion": payload.Version,
			},
		)
	})
	if err != nil {
		return nil, apperrors.WrapInternal(err, "create imported application")
	}

	return app, nil
}

func toExportDTO(app *domain.Application) ApplicationExportDTO {
	return ApplicationExportDTO{
		Slug:                              app.Slug,
		DisplayName:                       app.DisplayName,
		Category:                          app.Category,
		AppType:                           app.AppType,
		Description:                       app.Description,
		HomepageURL:                       app.HomepageURL,
		Domain:                            app.Domain,
		Logo:                              app.Logo,
		Favicon:                           app.Favicon,
		Title:                             app.Title,
		IsPortal:                          app.IsPortal,
		IsVisible:                         app.IsVisible,
		IsShared:                          app.IsShared,
		MenuOrder:                         app.MenuOrder,
		MenuMode:                          app.MenuMode,
		Status:                            app.Status,
		ClientID:                          app.ClientID,
		TokenEndpointAuthMethod:           app.EffectiveTokenEndpointAuthMethod(),
		RedirectURIs:                      app.RedirectURIs,
		GrantTypes:                        app.GrantTypes,
		TokenFormat:                       app.TokenFormat,
		ExpireInHours:                     app.ExpireInHours,
		RefreshTokenExpireInHours:         app.RefreshTokenExpireInHours,
		TokenSigningMethod:                app.TokenSigningMethod,
		CustomScopes:                      toCustomScopeItems(app.CustomScopes),
		EnableSamlCompress:                app.EnableSamlCompress,
		EnableSignup:                      app.EnableSignup,
		EnableExclusiveSignin:             app.EnableExclusiveSignin,
		EnableGuestSignin:                 app.EnableGuestSignin,
		EnableAutoSignin:                  app.EnableAutoSignin,
		EnableLinkWithEmail:               app.EnableLinkWithEmail,
		CookieExpireInHours:               app.CookieExpireInHours,
		SigninUrl:                         app.SigninUrl,
		LogoutUrl:                         app.LogoutUrl,
		LogoutRedirectUrl:                 app.LogoutRedirectUrl,
		SignupUrl:                         app.SignupUrl,
		ForgetUrl:                         app.ForgetUrl,
		AffiliationUrl:                    app.AffiliationUrl,
		PromptUrl:                         app.PromptUrl,
		BackchannelLogoutUri:              app.BackchannelLogoutUri,
		FrontchannelLogoutUri:             app.FrontchannelLogoutUri,
		FrontchannelLogoutSessionRequired: app.FrontchannelLogoutSessionRequired,
		SamlReplyUrl:                      app.SamlReplyUrl,
		SamlMetadata:                      app.SamlMetadata,
		SamlMetadataUrl:                   app.SamlMetadataUrl,
		SamlNameIdFormat:                  app.SamlNameIdFormat,
		SamlNameIdContent:                 app.SamlNameIdContent,
		SamlSignRequest:                   app.SamlSignRequest,
		SamlAttributes:                    app.SamlAttributes,
		EnableSamlC14n10:                  app.EnableSamlC14n10,
		EnableSamlPostBinding:             app.EnableSamlPostBinding,
		DisableSamlAttributes:             app.DisableSamlAttributes,
		EnableSamlAssertionSignature:      app.EnableSamlAssertionSignature,
		UseEmailAsSamlNameId:              app.UseEmailAsSamlNameId,
		ConsentMode:                       app.ConsentMode,
		DpopRequired:                      app.DpopRequired,
		PkceRequired:                      app.PkceRequired,
		IpWhitelist:                       app.IpWhitelist,
		IpBlacklist:                       app.IpBlacklist,
		FailedSigninLimit:                 app.FailedSigninLimit,
		FailedSigninFrozenTime:            app.FailedSigninFrozenTime,
		CodeResendTimeout:                 app.CodeResendTimeout,
		CorsAllowedOrigins:                app.CorsAllowedOrigins,
		DefaultGroup:                      app.DefaultGroup,
		OrgChoiceMode:                     app.OrgChoiceMode,
		TokenFields:                       app.TokenFields,
		TokenAttributes:                   app.TokenAttributes,
		ForcedRedirectOrigin:              app.ForcedRedirectOrigin,
		CodeExpire:                        app.CodeExpire,
		FormCss:                           app.FormCss,
		FormCssMobile:                     app.FormCssMobile,
		FormSideHtml:                      app.FormSideHtml,
		FormBackgroundUrl:                 app.FormBackgroundUrl,
		FormBackgroundUrlMobile:           app.FormBackgroundUrlMobile,
		FormOffset:                        app.FormOffset,
		FormPosition:                      app.FormPosition,
		CustomCss:                         app.CustomCss,
		CustomHeader:                      app.CustomHeader,
		HeaderHtml:                        app.HeaderHtml,
		FooterHtml:                        app.FooterHtml,
		SignupHtml:                        app.SignupHtml,
		SigninHtml:                        app.SigninHtml,
		TermsOfUse:                        app.TermsOfUse,
		PrivacyPolicy:                     app.PrivacyPolicy,
		SslMode:                           app.SslMode,
		SslCert:                           app.SslCert,
		ClientCert:                        app.ClientCert,
		SamlHashAlgorithm:                 app.SamlHashAlgorithm,
		SamlC14nPrefix:                    app.SamlC14nPrefix,
		PageHtml:                          app.PageHtml,
		UpstreamHost:                      app.UpstreamHost,
		OtherDomains:                      app.OtherDomains,
		EnableBetaOidc:                    app.EnableBetaOidc,
		SignupItems:                       app.SignupItems,
		SigninMethods:                     app.SigninMethods,
		SigninItems:                       app.SigninItems,
		ProviderItems:                     app.ProviderItems,
		Tags:                              app.Tags,
	}
}

func toCustomScopeItems(scopes []domain.ApplicationCustomScope) []ApplicationCustomScopeItem {
	items := make([]ApplicationCustomScopeItem, 0, len(scopes))
	for _, s := range scopes {
		items = append(items, ApplicationCustomScopeItem{
			Scope:       s.Scope,
			DisplayName: s.DisplayName,
			Description: s.Description,
		})
	}
	return items
}
