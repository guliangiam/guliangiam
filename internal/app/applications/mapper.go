package applications

import domain "github.com/guliangiam/guliangiam/internal/domain/applications"

func toListItem(application *domain.Application) ApplicationListItem {
	providerIDs := make([]string, 0, len(application.ProviderBindings))
	for _, binding := range application.ProviderBindings {
		providerIDs = append(providerIDs, binding.ProviderID)
	}

	return ApplicationListItem{
		ID:             application.ID,
		OrganizationID: application.OrganizationID,
		Slug:           application.Slug,
		Name:           application.Slug,
		DisplayName:    application.DisplayName,
		Category:       application.Category,
		AppType:        application.AppType,
		Domain:         application.Domain,
		Status:         application.Status,
		Tags:           append([]string(nil), application.Tags...),
		Logo:           application.Logo,
		HomepageURL:    application.HomepageURL,
		IsVisible:      application.IsVisible,
		Certificate:    application.Certificate,
		ProviderIDs:    providerIDs,
		CreatedAt:      application.CreatedAt,
		UpdatedAt:      application.UpdatedAt,
	}
}

// toDetail 映射详情。clientSecret 为可选参数：仅管理面 GetApplication 传入
// 其 secret 只读投影；其他读路径不查询 secret 表，保持 clientSecretSet=false。
func toDetail(application *domain.Application, clientSecret ...ClientSecretState) *ApplicationDetail {
	customScopes := make([]ApplicationCustomScopeItem, 0, len(application.CustomScopes))
	for _, item := range application.CustomScopes {
		customScopes = append(customScopes, ApplicationCustomScopeItem{
			Scope:       item.Scope,
			DisplayName: item.DisplayName,
			Description: item.Description,
		})
	}

	detail := &ApplicationDetail{
		ApplicationListItem:       toListItem(application),
		ClientJWKS:                emptyClientJWKSView(),
		Description:               application.Description,
		ClientID:                  application.ClientID,
		TokenEndpointAuthMethod:   application.EffectiveTokenEndpointAuthMethod(),
		ClientSecretSet:           len(clientSecret) > 0 && clientSecret[0].Set,
		ClientSecretMasked:        "",
		RedirectURIs:              append([]string(nil), application.RedirectURIs...),
		GrantTypes:                append([]string(nil), application.GrantTypes...),
		DisableSignin:             application.DisableSignin,
		EnableSigninSession:       application.EnableSigninSession,
		EnablePassword:            application.PasswordEnabled(),
		EnablePasswordLogin:       application.PasswordLoginEnabled(),
		EnableCodeSignin:          application.CodeSigninEnabled(),
		EnableSamlCompress:        application.EnableSamlCompress,
		SignupItems:               append([]domain.SignupItem(nil), application.SignupItems...),
		TokenFormat:               application.TokenFormat,
		ExpireInHours:             application.ExpireInHours,
		RefreshTokenExpireInHours: application.RefreshTokenExpireInHours,
		TokenSigningMethod:        application.TokenSigningMethod,
		ProviderItems:             append([]domain.ProviderItem(nil), application.ProviderItems...),
		SamlReplyUrl:              application.SamlReplyUrl,
		Certificate:               application.Certificate,
		CustomCss:                 application.CustomCss,
		CustomHeader:              application.CustomHeader,
		SigninUrl:                 application.SigninUrl,
		LogoutUrl:                 application.LogoutUrl,
		LogoutRedirectUrl:         application.LogoutRedirectUrl,
		IsPortal:                  application.IsPortal,
		PromptUrl:                 application.PromptUrl,
		SamlMetadata:              application.SamlMetadata,
		CustomScopes:              customScopes,
		Revision:                  application.Revision,
		// Phase 2 extended fields
		IsShared:                          application.IsShared,
		Title:                             application.Title,
		Favicon:                           application.Favicon,
		MenuOrder:                         application.MenuOrder,
		MenuMode:                          application.MenuMode,
		EnableSignup:                      application.EnableSignup,
		EnableExclusiveSignin:             application.EnableExclusiveSignin,
		EnableGuestSignin:                 application.EnableGuestSignin,
		EnableAutoSignin:                  application.EnableAutoSignin,
		EnableLinkWithEmail:               application.EnableLinkWithEmail,
		EnableWebAuthn:                    application.WebAuthnEnabled(),
		EnableDingtalk:                    application.DingTalkEnabled(),
		EnableWechat:                      application.WeChatEnabled(),
		EnableLark:                        application.LarkEnabled(),
		EnableCas:                         application.CASEnabled(),
		CookieExpireInHours:               application.CookieExpireInHours,
		SignupUrl:                         application.SignupUrl,
		ForgetUrl:                         application.ForgetUrl,
		AffiliationUrl:                    application.AffiliationUrl,
		TokenFields:                       application.TokenFields,
		TokenAttributes:                   append([]domain.TokenAttribute(nil), application.TokenAttributes...),
		ForcedRedirectOrigin:              application.ForcedRedirectOrigin,
		CodeExpire:                        application.CodeExpire,
		SamlNameIdFormat:                  application.SamlNameIdFormat,
		SamlNameIdContent:                 application.SamlNameIdContent,
		SamlSignRequest:                   application.SamlSignRequest,
		SamlMetadataUrl:                   application.SamlMetadataUrl,
		SamlAttributes:                    append([]domain.SamlAttribute(nil), application.SamlAttributes...),
		EnableSamlC14n10:                  application.EnableSamlC14n10,
		EnableSamlPostBinding:             application.EnableSamlPostBinding,
		DisableSamlAttributes:             application.DisableSamlAttributes,
		EnableSamlAssertionSignature:      application.EnableSamlAssertionSignature,
		UseEmailAsSamlNameId:              application.UseEmailAsSamlNameId,
		ConsentMode:                       application.ConsentMode,
		DpopRequired:                      application.DpopRequired,
		PkceRequired:                      application.PkceRequired,
		IpWhitelist:                       application.IpWhitelist,
		IpBlacklist:                       application.IpBlacklist,
		FailedSigninLimit:                 application.FailedSigninLimit,
		FailedSigninFrozenTime:            application.FailedSigninFrozenTime,
		CodeResendTimeout:                 application.CodeResendTimeout,
		CorsAllowedOrigins:                application.CorsAllowedOrigins,
		EnableBetaOidc:                    application.EnableBetaOidc,
		DefaultGroup:                      application.DefaultGroup,
		OrgChoiceMode:                     application.OrgChoiceMode,
		FormCss:                           application.FormCss,
		FormCssMobile:                     application.FormCssMobile,
		FormSideHtml:                      application.FormSideHtml,
		FormBackgroundUrl:                 application.FormBackgroundUrl,
		FormBackgroundUrlMobile:           application.FormBackgroundUrlMobile,
		FormOffset:                        application.FormOffset,
		FormPosition:                      application.FormPosition,
		HeaderHtml:                        application.HeaderHtml,
		FooterHtml:                        application.FooterHtml,
		SignupHtml:                        application.SignupHtml,
		SigninHtml:                        application.SigninHtml,
		TermsOfUse:                        application.TermsOfUse,
		PrivacyPolicy:                     application.PrivacyPolicy,
		BackchannelLogoutUri:              application.BackchannelLogoutUri,
		FrontchannelLogoutUri:             application.FrontchannelLogoutUri,
		FrontchannelLogoutSessionRequired: application.FrontchannelLogoutSessionRequired,
		OtherDomains:                      append([]string(nil), application.OtherDomains...),
		UpstreamHost:                      application.UpstreamHost,
		SslMode:                           application.SslMode,
		SslCert:                           application.SslCert,
		ClientCert:                        application.ClientCert,
		SamlHashAlgorithm:                 application.SamlHashAlgorithm,
		SamlC14nPrefix:                    application.SamlC14nPrefix,
		PageHtml:                          application.PageHtml,
		SigninMethods:                     append([]string(nil), application.SigninMethods...),
		SigninItems:                       append([]domain.SigninItemConfig(nil), application.SigninItems...),
	}
	if len(clientSecret) > 0 {
		detail.ClientSecretMasked = clientSecret[0].Masked
	}
	return detail
}

func toProviderBindingItems(bindings []domain.ApplicationProviderBinding) []ApplicationProviderBindingItem {
	items := make([]ApplicationProviderBindingItem, 0, len(bindings))
	for _, binding := range bindings {
		items = append(items, ApplicationProviderBindingItem{ProviderID: binding.ProviderID})
	}
	return items
}

// toPublicDetail maps the application aggregate onto the minimal anonymous
// discovery projection. It intentionally reads no SAML, callback, transport or
// token-policy field, so those values cannot reach an unauthenticated caller.
func toPublicDetail(application *domain.Application) *ApplicationPublicDetail {
	return &ApplicationPublicDetail{
		ID:                      application.ID,
		OrganizationID:          application.OrganizationID,
		Slug:                    application.Slug,
		Name:                    application.Slug,
		DisplayName:             application.DisplayName,
		Description:             application.Description,
		HomepageURL:             application.HomepageURL,
		ClientID:                application.ClientID,
		Logo:                    application.Logo,
		Title:                   application.Title,
		Favicon:                 application.Favicon,
		IsShared:                application.IsShared,
		EnableAutoSignin:        application.EnableAutoSignin,
		EnableSignup:            application.EnableSignup,
		ForcedRedirectOrigin:    application.ForcedRedirectOrigin,
		SignupURL:               application.SignupUrl,
		ForgetURL:               application.ForgetUrl,
		TermsOfUse:              application.TermsOfUse,
		PrivacyPolicy:           application.PrivacyPolicy,
		OrgChoiceMode:           application.OrgChoiceMode,
		SigninHTML:              application.SigninHtml,
		SignupHTML:              application.SignupHtml,
		HeaderHTML:              application.HeaderHtml,
		FooterHTML:              application.FooterHtml,
		CustomCSS:               application.CustomCss,
		CustomHeader:            application.CustomHeader,
		FormCSS:                 application.FormCss,
		FormCSSMobile:           application.FormCssMobile,
		FormSideHTML:            application.FormSideHtml,
		FormBackgroundURL:       application.FormBackgroundUrl,
		FormBackgroundURLMobile: application.FormBackgroundUrlMobile,
		FormOffset:              application.FormOffset,
		SigninMethods:           append([]string(nil), application.SigninMethods...),
		SigninItems:             append([]domain.SigninItemConfig(nil), application.SigninItems...),
		SignupItems:             append([]domain.SignupItem(nil), application.SignupItems...),
	}
}
