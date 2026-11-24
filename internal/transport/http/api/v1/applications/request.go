package applications

import (
	"encoding/json"
	"strings"

	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
)

type ListApplicationsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Category       string
	Status         string
}

type GetApplicationRequest struct {
	ID             string
	OrganizationID string
}

type CreateApplicationRequest struct {
	// OrganizationID is overwritten by the authenticated tenant scope (only a
	// platform administrator may request another tenant); it is kept in the
	// body for wire compatibility.
	OrganizationID          string                          `json:"organizationId"`
	Slug                    string                          `json:"slug"`
	DisplayName             string                          `json:"displayName"`
	Category                string                          `json:"category,omitempty"`
	AppType                 string                          `json:"appType,omitempty"`
	Description             string                          `json:"description,omitempty"`
	HomepageURL             string                          `json:"homepageUrl,omitempty"`
	Domain                  string                          `json:"domain,omitempty"`
	Status                  string                          `json:"status"`
	ClientID                string                          `json:"clientId,omitempty"`
	ClientSecret            string                          `json:"clientSecret,omitempty"`
	TokenEndpointAuthMethod string                          `json:"tokenEndpointAuthMethod,omitempty"`
	TokenSigningMethod      string                          `json:"tokenSigningMethod,omitempty"`
	RedirectURIs            []string                        `json:"redirectUris,omitempty"`
	GrantTypes              []string                        `json:"grantTypes,omitempty"`
	CustomScopes            []ApplicationCustomScopeRequest `json:"customScopes,omitempty"`
	// JWKS 是 private_key_jwt 的内嵌公钥集合。nil 表示未提供；提供时必须
	// 是 {"keys":[...]} 且不得含私钥参数（422）。
	JWKS json.RawMessage `json:"jwks,omitempty"`
	// JWKSURI 指向远端 JWKS（HTTPS）；与内嵌 JWKS 二选一或并存。
	JWKSURI string `json:"jwksUri,omitempty"`
	// ClientCertificates 是 mTLS 认证的初始证书集合，与 application 同事务
	// 写入；认证方式为 mTLS 时至少一张，其他方式必须为空。
	ClientCertificates []ApplicationClientCertificateItemRequest `json:"clientCertificates,omitempty"`
	// Phase 2: Extended fields
	IsShared                     *bool    `json:"isShared,omitempty"`
	Title                        string   `json:"title,omitempty"`
	Favicon                      string   `json:"favicon,omitempty"`
	MenuOrder                    int      `json:"menuOrder,omitempty"`
	MenuMode                     string   `json:"menuMode,omitempty"`
	EnableSignup                 *bool    `json:"enableSignup,omitempty"`
	EnableExclusiveSignin        *bool    `json:"enableExclusiveSignin,omitempty"`
	EnableGuestSignin            *bool    `json:"enableGuestSignin,omitempty"`
	EnableAutoSignin             *bool    `json:"enableAutoSignin,omitempty"`
	EnableLinkWithEmail          *bool    `json:"enableLinkWithEmail,omitempty"`
	CookieExpireInHours          int      `json:"cookieExpireInHours,omitempty"`
	SignupUrl                    string   `json:"signupUrl,omitempty"`
	ForgetUrl                    string   `json:"forgetUrl,omitempty"`
	AffiliationUrl               string   `json:"affiliationUrl,omitempty"`
	TokenFields                  []string `json:"tokenFields,omitempty"`
	ForcedRedirectOrigin         string   `json:"forcedRedirectOrigin,omitempty"`
	CodeExpire                   int      `json:"codeExpire,omitempty"`
	SamlNameIdFormat             string   `json:"samlNameIdFormat,omitempty"`
	SamlNameIdContent            string   `json:"samlNameIdContent,omitempty"`
	SamlSignRequest              *bool    `json:"samlSignRequest,omitempty"`
	SamlMetadataUrl              string   `json:"samlMetadataUrl,omitempty"`
	EnableSamlC14n10             *bool    `json:"enableSamlC14n10,omitempty"`
	EnableSamlPostBinding        *bool    `json:"enableSamlPostBinding,omitempty"`
	DisableSamlAttributes        *bool    `json:"disableSamlAttributes,omitempty"`
	EnableSamlAssertionSignature *bool    `json:"enableSamlAssertionSignature,omitempty"`
	UseEmailAsSamlNameId         *bool    `json:"useEmailAsSamlNameId,omitempty"`
	ConsentMode                  string   `json:"consentMode,omitempty"`
	DpopRequired                 *bool    `json:"dpopRequired,omitempty"`
	PkceRequired                 *bool    `json:"pkceRequired,omitempty"`
	IpWhitelist                  string   `json:"ipWhitelist,omitempty"`
	IpBlacklist                  string   `json:"ipBlacklist,omitempty"`
	FailedSigninLimit            int      `json:"failedSigninLimit,omitempty"`
	FailedSigninFrozenTime       int      `json:"failedSigninFrozenTime,omitempty"`
	CodeResendTimeout            int      `json:"codeResendTimeout,omitempty"`
	CorsAllowedOrigins           string   `json:"corsAllowedOrigins,omitempty"`
	EnableBetaOidc               *bool    `json:"enableBetaOidc,omitempty"`
	DefaultGroup                 string   `json:"defaultGroup,omitempty"`
	OrgChoiceMode                string   `json:"orgChoiceMode,omitempty"`
	FormCss                      string   `json:"formCss,omitempty"`
	FormCssMobile                string   `json:"formCssMobile,omitempty"`
	FormSideHtml                 string   `json:"formSideHtml,omitempty"`
	FormBackgroundUrl            string   `json:"formBackgroundUrl,omitempty"`
	FormBackgroundUrlMobile      string   `json:"formBackgroundUrlMobile,omitempty"`
	FormOffset                   string   `json:"formOffset,omitempty"`
	FormPosition                 string   `json:"formPosition,omitempty"`
	HeaderHtml                   string   `json:"headerHtml,omitempty"`
	FooterHtml                   string   `json:"footerHtml,omitempty"`
	SignupHtml                   string   `json:"signupHtml,omitempty"`
	SigninHtml                   string   `json:"signinHtml,omitempty"`
	TermsOfUse                   string   `json:"termsOfUse,omitempty"`
	PrivacyPolicy                string   `json:"privacyPolicy,omitempty"`
	BackchannelLogoutUri         string   `json:"backchannelLogoutUri,omitempty"`
	OtherDomains                 []string `json:"otherDomains,omitempty"`
	UpstreamHost                 string   `json:"upstreamHost,omitempty"`
	SslMode                      string   `json:"sslMode,omitempty"`
	SslCert                      string   `json:"sslCert,omitempty"`
	ClientCert                   string   `json:"clientCert,omitempty"`
	SamlHashAlgorithm            string   `json:"samlHashAlgorithm,omitempty"`
	SamlC14nPrefix               string   `json:"samlC14nPrefix,omitempty"`
	PageHtml                     string   `json:"pageHtml,omitempty"`
}

type PatchApplicationRequest struct {
	Slug        *string `json:"slug,omitempty"`
	DisplayName *string `json:"displayName,omitempty"`
	Category    *string `json:"category,omitempty"`
	AppType     *string `json:"appType,omitempty"`
	Description *string `json:"description,omitempty"`
	HomepageURL *string `json:"homepageUrl,omitempty"`
	Domain      *string `json:"domain,omitempty"`
	Status      *string `json:"status,omitempty"`
	// Legacy / Phase 0 fields that FE sends but were missing from the Request struct.
	Logo                      *string `json:"logo,omitempty"`
	LogoutUrl                 *string `json:"logoutUrl,omitempty"`
	LogoutRedirectUrl         *string `json:"logoutRedirectUrl,omitempty"`
	Certificate               *string `json:"certificate,omitempty"`
	CustomHeader              *string `json:"customHeader,omitempty"`
	EnableSamlCompress        *bool   `json:"enableSamlCompress,omitempty"`
	ExpireInHours             *int    `json:"expireInHours,omitempty"`
	RefreshTokenExpireInHours *int    `json:"refreshTokenExpireInHours,omitempty"`
	TokenFormat               *string `json:"tokenFormat,omitempty"`
	TokenSigningMethod        *string `json:"tokenSigningMethod,omitempty"`

	ClientID                *string                         `json:"clientId,omitempty"`
	ClientSecret            *string                         `json:"clientSecret,omitempty"`
	TokenEndpointAuthMethod *string                         `json:"tokenEndpointAuthMethod,omitempty"`
	RedirectURIs            []string                        `json:"redirectUris,omitempty"`
	GrantTypes              []string                        `json:"grantTypes,omitempty"`
	CustomScopes            []ApplicationCustomScopeRequest `json:"customScopes,omitempty"`
	// JWKS 非 nil 时整体替换现有 active 公钥集合；nil 表示不修改。
	JWKS json.RawMessage `json:"jwks,omitempty"`
	// JWKSURI 非 nil 时替换远端 JWKS；指向空串表示删除该 URI。
	JWKSURI *string `json:"jwksUri,omitempty"`
	// ClientCertificates 非 nil 时整体替换 mTLS 证书集合；nil 表示不修改。
	ClientCertificates *[]ApplicationClientCertificateItemRequest `json:"clientCertificates,omitempty"`
	CustomCss          *string                                    `json:"customCss,omitempty"`
	SigninUrl          *string                                    `json:"signinUrl,omitempty"`
	Revision           int64                                      `json:"revision,omitempty"`
	// Phase 2: Extended fields (all pointers for partial update)
	IsShared                     *bool    `json:"isShared,omitempty"`
	Title                        *string  `json:"title,omitempty"`
	Favicon                      *string  `json:"favicon,omitempty"`
	MenuOrder                    *int     `json:"menuOrder,omitempty"`
	MenuMode                     *string  `json:"menuMode,omitempty"`
	EnableSignup                 *bool    `json:"enableSignup,omitempty"`
	EnableExclusiveSignin        *bool    `json:"enableExclusiveSignin,omitempty"`
	EnableGuestSignin            *bool    `json:"enableGuestSignin,omitempty"`
	EnableAutoSignin             *bool    `json:"enableAutoSignin,omitempty"`
	EnableLinkWithEmail          *bool    `json:"enableLinkWithEmail,omitempty"`
	CookieExpireInHours          *int     `json:"cookieExpireInHours,omitempty"`
	DisableSignin                *bool    `json:"disableSignin,omitempty"`
	EnableSigninSession          *bool    `json:"enableSigninSession,omitempty"`
	Tags                         []string `json:"tags,omitempty"`
	SignupUrl                    *string  `json:"signupUrl,omitempty"`
	ForgetUrl                    *string  `json:"forgetUrl,omitempty"`
	AffiliationUrl               *string  `json:"affiliationUrl,omitempty"`
	TokenFields                  []string `json:"tokenFields,omitempty"`
	ForcedRedirectOrigin         *string  `json:"forcedRedirectOrigin,omitempty"`
	CodeExpire                   *int     `json:"codeExpire,omitempty"`
	SamlNameIdFormat             *string  `json:"samlNameIdFormat,omitempty"`
	SamlNameIdContent            *string  `json:"samlNameIdContent,omitempty"`
	SamlSignRequest              *bool    `json:"samlSignRequest,omitempty"`
	SamlMetadataUrl              *string  `json:"samlMetadataUrl,omitempty"`
	EnableSamlC14n10             *bool    `json:"enableSamlC14n10,omitempty"`
	EnableSamlPostBinding        *bool    `json:"enableSamlPostBinding,omitempty"`
	DisableSamlAttributes        *bool    `json:"disableSamlAttributes,omitempty"`
	EnableSamlAssertionSignature *bool    `json:"enableSamlAssertionSignature,omitempty"`
	UseEmailAsSamlNameId         *bool    `json:"useEmailAsSamlNameId,omitempty"`
	ConsentMode                  *string  `json:"consentMode,omitempty"`
	DpopRequired                 *bool    `json:"dpopRequired,omitempty"`
	PkceRequired                 *bool    `json:"pkceRequired,omitempty"`
	IpWhitelist                  *string  `json:"ipWhitelist,omitempty"`
	IpBlacklist                  *string  `json:"ipBlacklist,omitempty"`
	FailedSigninLimit            *int     `json:"failedSigninLimit,omitempty"`
	FailedSigninFrozenTime       *int     `json:"failedSigninFrozenTime,omitempty"`
	CodeResendTimeout            *int     `json:"codeResendTimeout,omitempty"`
	CorsAllowedOrigins           *string  `json:"corsAllowedOrigins,omitempty"`
	EnableBetaOidc               *bool    `json:"enableBetaOidc,omitempty"`
	DefaultGroup                 *string  `json:"defaultGroup,omitempty"`
	OrgChoiceMode                *string  `json:"orgChoiceMode,omitempty"`
	FormCss                      *string  `json:"formCss,omitempty"`
	FormCssMobile                *string  `json:"formCssMobile,omitempty"`
	FormSideHtml                 *string  `json:"formSideHtml,omitempty"`
	FormBackgroundUrl            *string  `json:"formBackgroundUrl,omitempty"`
	FormBackgroundUrlMobile      *string  `json:"formBackgroundUrlMobile,omitempty"`
	FormOffset                   *string  `json:"formOffset,omitempty"`
	FormPosition                 *string  `json:"formPosition,omitempty"`
	HeaderHtml                   *string  `json:"headerHtml,omitempty"`
	FooterHtml                   *string  `json:"footerHtml,omitempty"`
	SignupHtml                   *string  `json:"signupHtml,omitempty"`
	SigninHtml                   *string  `json:"signinHtml,omitempty"`
	TermsOfUse                   *string  `json:"termsOfUse,omitempty"`
	PrivacyPolicy                *string  `json:"privacyPolicy,omitempty"`
	BackchannelLogoutUri         *string  `json:"backchannelLogoutUri,omitempty"`
	OtherDomains                 []string `json:"otherDomains,omitempty"`
	UpstreamHost                 *string  `json:"upstreamHost,omitempty"`
	SslMode                      *string  `json:"sslMode,omitempty"`
	SslCert                      *string  `json:"sslCert,omitempty"`
	ClientCert                   *string  `json:"clientCert,omitempty"`
	SamlHashAlgorithm            *string  `json:"samlHashAlgorithm,omitempty"`
	SamlC14nPrefix               *string  `json:"samlC14nPrefix,omitempty"`
	PageHtml                     *string  `json:"pageHtml,omitempty"`
	// Passthrough: FE sends these for change-detection snapshots; PATCH accepts
	// them to satisfy DisallowUnknownFields, but they are mapped to domain
	// fields that are typically managed via dedicated endpoints.
	IsPortal                          *bool   `json:"isPortal,omitempty"`
	IsVisible                         *bool   `json:"isVisible,omitempty"`
	PromptUrl                         *string `json:"promptUrl,omitempty"`
	SamlReplyUrl                      *string `json:"samlReplyUrl,omitempty"`
	SamlMetadata                      *string `json:"samlMetadata,omitempty"`
	FrontchannelLogoutUri             *string `json:"frontchannelLogoutUri,omitempty"`
	FrontchannelLogoutSessionRequired *bool   `json:"frontchannelLogoutSessionRequired,omitempty"`
	// Nested configuration slices edited in the console. They are forwarded as
	// raw JSON because transport must not import the domain layer; the
	// application layer decodes them into domain types. nil means "not
	// submitted", an explicit empty array means "clear".
	ProviderItems   json.RawMessage `json:"providerItems,omitempty"`
	SigninMethods   json.RawMessage `json:"signinMethods,omitempty"`
	SigninItems     json.RawMessage `json:"signinItems,omitempty"`
	SignupItems     json.RawMessage `json:"signupItems,omitempty"`
	SamlAttributes  json.RawMessage `json:"samlAttributes,omitempty"`
	TokenAttributes json.RawMessage `json:"tokenAttributes,omitempty"`
}

type ApplicationCustomScopeRequest struct {
	Scope       string `json:"scope"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
}

type ReplaceApplicationProviderBindingsRequest struct {
	Items []ReplaceApplicationProviderBindingItemRequest `json:"items"`
}

type ReplaceApplicationProviderBindingItemRequest struct {
	ProviderID string `json:"providerId"`
}

// ReplaceApplicationClientCertificatesRequest 是替换式全量写入。
// 每项必须同时提交证书本体与指纹：服务端重算后比对，只报指纹无法登记。
type ReplaceApplicationClientCertificatesRequest struct {
	Items []ApplicationClientCertificateItemRequest `json:"items"`
}

type ApplicationClientCertificateItemRequest struct {
	FingerprintSHA256 string `json:"fingerprintSha256"`
	Certificate       string `json:"certificate"`
	SelfSigned        bool   `json:"selfSigned"`
}

func (r ReplaceApplicationClientCertificatesRequest) toCommand(organizationID, id, actorID string) appapplications.ReplaceClientCertificatesCommand {
	return appapplications.ReplaceClientCertificatesCommand{
		ID:             id,
		OrganizationID: organizationID,
		ActorID:        actorID,
		Certificates:   toClientCertificateInputs(r.Items),
	}
}

func toClientCertificateInputs(items []ApplicationClientCertificateItemRequest) []appapplications.ClientCertificateInput {
	inputs := make([]appapplications.ClientCertificateInput, 0, len(items))
	for _, item := range items {
		inputs = append(inputs, appapplications.ClientCertificateInput{
			FingerprintSHA256: item.FingerprintSHA256,
			Certificate:       item.Certificate,
			SelfSigned:        item.SelfSigned,
		})
	}
	return inputs
}

func (r CreateApplicationRequest) toCommand() appapplications.CreateApplicationCommand {
	return appapplications.CreateApplicationCommand{
		OrganizationID:          r.OrganizationID,
		Slug:                    r.Slug,
		DisplayName:             r.DisplayName,
		Category:                r.Category,
		AppType:                 r.AppType,
		Description:             r.Description,
		HomepageURL:             r.HomepageURL,
		Domain:                  r.Domain,
		Status:                  r.Status,
		ClientID:                r.ClientID,
		ClientSecret:            r.ClientSecret,
		TokenEndpointAuthMethod: r.TokenEndpointAuthMethod,
		TokenSigningMethod:      r.TokenSigningMethod,
		RedirectURIs:            append([]string(nil), r.RedirectURIs...),
		GrantTypes:              append([]string(nil), r.GrantTypes...),
		CustomScopes:            toApplicationCustomScopeCommands(r.CustomScopes),
		JWKS:                    optionalJWKSPayload(r.JWKS),
		JWKSURI:                 r.JWKSURI,
		ClientCertificates:      toClientCertificateInputs(r.ClientCertificates),
	}
}

func (r PatchApplicationRequest) toCommand(id string) appapplications.PatchApplicationCommand {
	var customScopes *[]appapplications.ApplicationCustomScopeCommand
	var redirectURIs *[]string
	var grantTypes *[]string
	var tokenFields *string
	if r.CustomScopes != nil {
		items := toApplicationCustomScopeCommands(r.CustomScopes)
		customScopes = &items
	}
	if r.RedirectURIs != nil {
		items := append([]string(nil), r.RedirectURIs...)
		redirectURIs = &items
	}
	if r.GrantTypes != nil {
		items := append([]string(nil), r.GrantTypes...)
		grantTypes = &items
	}
	if r.TokenFields != nil {
		// FE sends string[]; domain stores Postgres text[] as pgx's curly-brace
		// serialized form (e.g. "{openid,email}"). Re-encode here so pgx round-trips cleanly.
		joined := "{" + strings.Join(r.TokenFields, ",") + "}"
		tokenFields = &joined
	}

	var otherDomains *[]string
	if r.OtherDomains != nil {
		items := append([]string(nil), r.OtherDomains...)
		otherDomains = &items
	}
	var tags *[]string
	if r.Tags != nil {
		items := append([]string(nil), r.Tags...)
		tags = &items
	}

	var clientCertificates *[]appapplications.ClientCertificateInput
	if r.ClientCertificates != nil {
		items := toClientCertificateInputs(*r.ClientCertificates)
		clientCertificates = &items
	}

	return appapplications.PatchApplicationCommand{
		ID:          id,
		Slug:        r.Slug,
		DisplayName: r.DisplayName,
		Category:    r.Category,
		AppType:     r.AppType,
		Description: r.Description,
		HomepageURL: r.HomepageURL,
		Domain:      r.Domain,
		Status:      r.Status,
		// Legacy / Phase 0 fields.
		Logo:                      r.Logo,
		LogoutUrl:                 r.LogoutUrl,
		LogoutRedirectUrl:         r.LogoutRedirectUrl,
		Certificate:               r.Certificate,
		CustomHeader:              r.CustomHeader,
		EnableSamlCompress:        r.EnableSamlCompress,
		ExpireInHours:             r.ExpireInHours,
		RefreshTokenExpireInHours: r.RefreshTokenExpireInHours,
		TokenFormat:               r.TokenFormat,
		TokenSigningMethod:        r.TokenSigningMethod,
		ClientID:                  r.ClientID,
		ClientSecret:              r.ClientSecret,
		TokenEndpointAuthMethod:   r.TokenEndpointAuthMethod,
		RedirectURIs:              redirectURIs,
		GrantTypes:                grantTypes,
		CustomScopes:              customScopes,
		JWKS:                      optionalJWKSPayload(r.JWKS),
		JWKSURI:                   r.JWKSURI,
		ClientCertificates:        clientCertificates,
		CustomCss:                 r.CustomCss,
		SigninUrl:                 r.SigninUrl,
		Revision:                  r.Revision,
		// Phase 2: Extended fields
		IsShared:                     r.IsShared,
		Title:                        r.Title,
		Favicon:                      r.Favicon,
		MenuOrder:                    r.MenuOrder,
		MenuMode:                     r.MenuMode,
		EnableSignup:                 r.EnableSignup,
		EnableExclusiveSignin:        r.EnableExclusiveSignin,
		EnableGuestSignin:            r.EnableGuestSignin,
		EnableAutoSignin:             r.EnableAutoSignin,
		EnableLinkWithEmail:          r.EnableLinkWithEmail,
		DisableSignin:                r.DisableSignin,
		EnableSigninSession:          r.EnableSigninSession,
		Tags:                         tags,
		CookieExpireInHours:          r.CookieExpireInHours,
		SignupUrl:                    r.SignupUrl,
		ForgetUrl:                    r.ForgetUrl,
		AffiliationUrl:               r.AffiliationUrl,
		TokenFields:                  tokenFields,
		ForcedRedirectOrigin:         r.ForcedRedirectOrigin,
		CodeExpire:                   r.CodeExpire,
		SamlNameIdFormat:             r.SamlNameIdFormat,
		SamlNameIdContent:            r.SamlNameIdContent,
		SamlSignRequest:              r.SamlSignRequest,
		SamlMetadataUrl:              r.SamlMetadataUrl,
		EnableSamlC14n10:             r.EnableSamlC14n10,
		EnableSamlPostBinding:        r.EnableSamlPostBinding,
		DisableSamlAttributes:        r.DisableSamlAttributes,
		EnableSamlAssertionSignature: r.EnableSamlAssertionSignature,
		UseEmailAsSamlNameId:         r.UseEmailAsSamlNameId,
		ConsentMode:                  r.ConsentMode,
		DpopRequired:                 r.DpopRequired,
		PkceRequired:                 r.PkceRequired,
		IpWhitelist:                  r.IpWhitelist,
		IpBlacklist:                  r.IpBlacklist,
		FailedSigninLimit:            r.FailedSigninLimit,
		FailedSigninFrozenTime:       r.FailedSigninFrozenTime,
		CodeResendTimeout:            r.CodeResendTimeout,
		CorsAllowedOrigins:           r.CorsAllowedOrigins,
		EnableBetaOidc:               r.EnableBetaOidc,
		DefaultGroup:                 r.DefaultGroup,
		OrgChoiceMode:                r.OrgChoiceMode,
		FormCss:                      r.FormCss,
		FormCssMobile:                r.FormCssMobile,
		FormSideHtml:                 r.FormSideHtml,
		FormBackgroundUrl:            r.FormBackgroundUrl,
		FormBackgroundUrlMobile:      r.FormBackgroundUrlMobile,
		FormOffset:                   r.FormOffset,
		FormPosition:                 r.FormPosition,
		HeaderHtml:                   r.HeaderHtml,
		FooterHtml:                   r.FooterHtml,
		SignupHtml:                   r.SignupHtml,
		SigninHtml:                   r.SigninHtml,
		TermsOfUse:                   r.TermsOfUse,
		PrivacyPolicy:                r.PrivacyPolicy,
		BackchannelLogoutUri:         r.BackchannelLogoutUri,
		OtherDomains:                 otherDomains,
		UpstreamHost:                 r.UpstreamHost,
		SslMode:                      r.SslMode,
		SslCert:                      r.SslCert,
		ClientCert:                   r.ClientCert,
		SamlHashAlgorithm:            r.SamlHashAlgorithm,
		SamlC14nPrefix:               r.SamlC14nPrefix,
		PageHtml:                     r.PageHtml,
		// Passthrough aggregation/read-only fields — decoded but typically managed
		// via dedicated endpoints; forwarding them keeps FE change-detection snapshots symmetric.
		IsPortal:                          r.IsPortal,
		IsVisible:                         r.IsVisible,
		PromptUrl:                         r.PromptUrl,
		SamlReplyUrl:                      r.SamlReplyUrl,
		SamlMetadata:                      r.SamlMetadata,
		FrontchannelLogoutUri:             r.FrontchannelLogoutUri,
		FrontchannelLogoutSessionRequired: r.FrontchannelLogoutSessionRequired,
		ProviderItems:                     r.ProviderItems,
		SigninMethods:                     r.SigninMethods,
		SigninItems:                       r.SigninItems,
		SignupItems:                       r.SignupItems,
		SamlAttributes:                    r.SamlAttributes,
		TokenAttributes:                   r.TokenAttributes,
	}
}

func toApplicationCustomScopeCommands(items []ApplicationCustomScopeRequest) []appapplications.ApplicationCustomScopeCommand {
	result := make([]appapplications.ApplicationCustomScopeCommand, 0, len(items))
	for _, item := range items {
		result = append(result, appapplications.ApplicationCustomScopeCommand{
			Scope:       item.Scope,
			DisplayName: item.DisplayName,
			Description: item.Description,
		})
	}
	return result
}

// optionalJWKSPayload preserves the nil/absent distinction: PATCH relies on nil
// meaning "do not modify", while an explicit empty keys array means "clear".
func optionalJWKSPayload(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	return append([]byte(nil), raw...)
}

func (r ReplaceApplicationProviderBindingsRequest) toCommands() []appapplications.ReplaceApplicationProviderBindingCommand {
	items := make([]appapplications.ReplaceApplicationProviderBindingCommand, 0, len(r.Items))
	for _, item := range r.Items {
		items = append(items, appapplications.ReplaceApplicationProviderBindingCommand{ProviderID: item.ProviderID})
	}
	return items
}

// ImportApplicationRequest is the request body for importing an application.
type ImportApplicationRequest struct {
	// OrganizationID follows the same trusted-scope rule as CreateApplicationRequest.
	OrganizationID string                               `json:"organizationId"`
	Version        string                               `json:"version"`
	Export         appapplications.ApplicationExportDTO `json:"export"`
}
