package applications

import (
	"strings"

	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func toApplication(row applicationRow) (domain.Application, error) {
	application := domain.Application{
		ID:                                row.ID,
		OrganizationID:                    row.OrganizationID,
		Slug:                              row.Slug,
		DisplayName:                       row.DisplayName,
		Category:                          row.Category,
		AppType:                           row.AppType,
		Description:                       derefString(row.Description),
		HomepageURL:                       derefString(row.HomepageURL),
		Domain:                            derefString(row.Domain),
		Status:                            row.Status,
		ClientID:                          derefString(row.ClientID),
		ClientSecretHash:                  derefString(row.ClientSecretHash),
		TokenEndpointAuthMethod:           row.TokenEndpointAuthMethod,
		DisableSignin:                     row.DisableSignin,
		EnableExclusiveSignin:             row.EnableExclusiveSignin,
		EnableSigninSession:               row.EnableSigninSession,
		Logo:                              row.Logo,
		EnableSamlCompress:                row.EnableSamlCompress,
		TokenFormat:                       row.TokenFormat,
		ExpireInHours:                     row.ExpireInHours,
		RefreshTokenExpireInHours:         row.RefreshTokenExpireInHours,
		TokenSigningMethod:                row.TokenSigningMethod,
		SamlReplyUrl:                      row.SamlReplyUrl,
		Certificate:                       row.Certificate,
		CustomCss:                         row.CustomCss,
		CustomHeader:                      row.CustomHeader,
		SigninUrl:                         row.SigninUrl,
		LogoutUrl:                         row.LogoutUrl,
		LogoutRedirectUrl:                 row.LogoutRedirectUrl,
		IsPortal:                          row.IsPortal,
		IsVisible:                         row.IsVisible,
		PromptUrl:                         row.PromptUrl,
		SamlMetadata:                      row.SamlMetadata,
		DeletedAt:                         row.DeletedAt,
		CreatedAt:                         row.CreatedAt,
		UpdatedAt:                         row.UpdatedAt,
		Revision:                          row.Revision,
		IsShared:                          row.IsShared,
		Title:                             row.Title,
		Favicon:                           row.Favicon,
		MenuOrder:                         row.MenuOrder,
		MenuMode:                          row.MenuMode,
		EnableSignup:                      row.EnableSignup,
		EnableGuestSignin:                 row.EnableGuestSignin,
		EnableAutoSignin:                  row.EnableAutoSignin,
		EnableLinkWithEmail:               row.EnableLinkWithEmail,
		CookieExpireInHours:               row.CookieExpireInHours,
		SignupUrl:                         row.SignupUrl,
		ForgetUrl:                         row.ForgetUrl,
		AffiliationUrl:                    row.AffiliationUrl,
		TokenFields:                       row.TokenFields,
		ForcedRedirectOrigin:              row.ForcedRedirectOrigin,
		CodeExpire:                        row.CodeExpire,
		SamlNameIdFormat:                  row.SamlNameIdFormat,
		SamlNameIdContent:                 row.SamlNameIdContent,
		SamlSignRequest:                   row.SamlSignRequest,
		SamlMetadataUrl:                   row.SamlMetadataUrl,
		EnableSamlC14n10:                  row.EnableSamlC14n10,
		EnableSamlPostBinding:             row.EnableSamlPostBinding,
		DisableSamlAttributes:             row.DisableSamlAttributes,
		EnableSamlAssertionSignature:      row.EnableSamlAssertionSignature,
		UseEmailAsSamlNameId:              row.UseEmailAsSamlNameId,
		ConsentMode:                       row.ConsentMode,
		DpopRequired:                      row.DpopRequired,
		PkceRequired:                      row.PkceRequired,
		IpWhitelist:                       row.IpWhitelist,
		IpBlacklist:                       row.IpBlacklist,
		FailedSigninLimit:                 row.FailedSigninLimit,
		FailedSigninFrozenTime:            row.FailedSigninFrozenTime,
		CodeResendTimeout:                 row.CodeResendTimeout,
		EnableBetaOidc:                    row.EnableBetaOidc,
		DefaultGroup:                      row.DefaultGroup,
		OrgChoiceMode:                     row.OrgChoiceMode,
		FormCss:                           row.FormCss,
		FormCssMobile:                     row.FormCssMobile,
		FormSideHtml:                      row.FormSideHtml,
		FormBackgroundUrl:                 row.FormBackgroundUrl,
		FormBackgroundUrlMobile:           row.FormBackgroundUrlMobile,
		FormOffset:                        row.FormOffset,
		FormPosition:                      row.FormPosition,
		HeaderHtml:                        row.HeaderHtml,
		FooterHtml:                        row.FooterHtml,
		SignupHtml:                        row.SignupHtml,
		SigninHtml:                        row.SigninHtml,
		TermsOfUse:                        row.TermsOfUse,
		PrivacyPolicy:                     row.PrivacyPolicy,
		BackchannelLogoutUri:              row.BackchannelLogoutUri,
		FrontchannelLogoutUri:             row.FrontchannelLogoutUri,
		FrontchannelLogoutSessionRequired: row.FrontchannelLogoutSessionRequired,
		UpstreamHost:                      row.UpstreamHost,
		SslMode:                           row.SslMode,
		SslCert:                           row.SslCert,
		ClientCert:                        row.ClientCert,
		SamlHashAlgorithm:                 row.SamlHashAlgorithm,
		SamlC14nPrefix:                    row.SamlC14nPrefix,
		PageHtml:                          row.PageHtml,
		CustomJs:                          row.CustomJs,
	}
	if err := mysqlshared.UnmarshalJSON(row.CustomScopes, &application.CustomScopes); err != nil {
		return domain.Application{}, apperrors.WrapInternalf(err, "unmarshal custom_scopes")
	}
	// MySQL DDL: TEXT[] → JSON TEXT → []byte, mapper 层转换
	if err := mysqlshared.UnmarshalJSON(row.RedirectURIs, &application.RedirectURIs); err != nil {
		return domain.Application{}, apperrors.WrapInternalf(err, "unmarshal redirect_uris")
	}
	if err := mysqlshared.UnmarshalJSON(row.GrantTypes, &application.GrantTypes); err != nil {
		return domain.Application{}, apperrors.WrapInternalf(err, "unmarshal grant_types")
	}
	if err := mysqlshared.UnmarshalJSON(row.Tags, &application.Tags); err != nil {
		return domain.Application{}, apperrors.WrapInternalf(err, "unmarshal tags")
	}
	if err := mysqlshared.UnmarshalJSON(row.OtherDomains, &application.OtherDomains); err != nil {
		return domain.Application{}, apperrors.WrapInternalf(err, "unmarshal other_domains")
	}
	if err := mysqlshared.UnmarshalJSON(row.SignupItems, &application.SignupItems); err != nil {
		return domain.Application{}, apperrors.WrapInternalf(err, "unmarshal signup_items")
	}
	if err := mysqlshared.UnmarshalJSON(row.ProviderItems, &application.ProviderItems); err != nil {
		return domain.Application{}, apperrors.WrapInternalf(err, "unmarshal provider_items")
	}
	if err := mysqlshared.UnmarshalJSON(row.TokenAttributes, &application.TokenAttributes); err != nil {
		return domain.Application{}, apperrors.WrapInternalf(err, "unmarshal token_attributes")
	}
	if err := mysqlshared.UnmarshalJSON(row.SamlAttributes, &application.SamlAttributes); err != nil {
		return domain.Application{}, apperrors.WrapInternalf(err, "unmarshal saml_attributes")
	}
	if err := mysqlshared.UnmarshalJSON(row.SigninMethods, &application.SigninMethods); err != nil {
		return domain.Application{}, apperrors.WrapInternalf(err, "unmarshal signin_methods")
	}
	if err := mysqlshared.UnmarshalJSON(row.SigninItems, &application.SigninItems); err != nil {
		return domain.Application{}, apperrors.WrapInternalf(err, "unmarshal signin_items")
	}
	var corsAllowedOrigins []string
	if err := mysqlshared.UnmarshalJSON(row.CorsAllowedOrigins, &corsAllowedOrigins); err != nil {
		return domain.Application{}, apperrors.WrapInternalf(err, "unmarshal cors_allowed_origins")
	}
	application.CorsAllowedOrigins = strings.Join(corsAllowedOrigins, ",")
	return application, nil
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
