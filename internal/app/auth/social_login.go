package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// SocialUserInfo contains the normalized user information returned by an IdP.
// This mirrors idp.UserInfo to avoid importing the infra layer from the app layer.
type SocialUserInfo struct {
	ID          string
	Username    string
	DisplayName string
	Email       string
	Phone       string
	AvatarURL   string
	// AuthnContext is the SAML AuthnContextClassRef asserted by the IdP. Other
	// adapters leave it empty; the social login path maps it onto the session
	// authentication level (and keeps the raw value as acr for audit).
	AuthnContext string
	// AccessToken is the upstream token, kept only to call the provider's
	// custom logout endpoint. It is sealed before it reaches the session.
	AccessToken string
}

// SocialLoginCommand handles the OAuth callback for social login.
type SocialLoginCommand struct {
	ProviderID        string // ID of the provider record in the database
	Code              string // OAuth authorization code
	State             string // CSRF state parameter
	CodeVerifier      string // PKCE code_verifier issued by the console flow
	OrganizationSlug  string // Optional: pre-select organization
	PreviousSessionID string
	ApplicationID     string // Application requesting login
	IPAddress         string
	UserAgent         string
	// HandleLoggedIn parameters
	ResponseType string
	RedirectURI  string
	Nonce        string
	Scope        []string
}

// SocialAuthorizeCommand generates the redirect URL for a social provider.
type SocialAuthorizeCommand struct {
	ProviderID       string
	OrganizationSlug string
	ApplicationID    string
	ResponseType     string
	RedirectURI      string
	State            string
	Nonce            string
	Scope            []string
}

// SocialLoginResult contains the result of a social login operation.
type SocialLoginResult struct {
	LoginResult
	IsNewUser bool // true if a new user was auto-registered
	// registeredUser 供 ADR-029 post_registration 在事务提交后触发；
	// 不导出、不参与序列化。
	registeredUser          *userdomain.User
	registeredApplicationID string
}

// SocialProviderConfig carries the provider columns the IDP adapters need.
// Host-based adapters (Okta/Auth0/Casdoor/Gitea/Nextcloud) derive their
// endpoints from IssuerURL/Host, and explicit auth/token/userinfo URLs win
// when the admin configured them. The SAML fields carry the SP entity ID,
// IdP SSO URL, signing certificate and metadata so the SP adapter can enforce
// audience/recipient/destination instead of trusting unconfigured defaults.
type SocialProviderConfig struct {
	ProviderID     string
	OrganizationID string
	Category       string
	ProviderType   string
	ClientID       string
	ClientSecret   string
	RedirectURI    string
	IssuerURL      string
	AuthURL        string
	TokenURL       string
	UserInfoURL    string
	Host           string
	Scopes         []string
	EnablePkce     bool
	EnableProxy    bool
	HTTPHeaders    map[string]string
	CodeVerifier   string
	// Nonce and the PKCE challenge are per-authorization runtime values for
	// generic OIDC. They are generated server-side and never come from the
	// provider row.
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
	// State is the OAuth/SAML state echoed by the provider. The SAML adapter
	// derives the expected AuthnRequest ID from it.
	State string
	// SAML-specific provider columns.
	IdPEntityID  string
	SsoURL       string
	SAMLCert     string
	SAMLMetadata string
}

func socialProviderConfig(provider *providerdomain.Provider, redirectURI string) SocialProviderConfig {
	return SocialProviderConfig{
		ProviderID:     provider.ID,
		OrganizationID: provider.OrganizationID,
		Category:       string(provider.Category),
		ProviderType:   string(provider.ProviderType),
		ClientID:       provider.ClientID,
		ClientSecret:   provider.ClientSecret,
		RedirectURI:    redirectURI,
		IssuerURL:      provider.IssuerUrl,
		AuthURL:        provider.AuthUrl,
		TokenURL:       provider.TokenUrl,
		UserInfoURL:    provider.UserInfoUrl,
		Host:           provider.Host,
		Scopes:         append([]string{}, provider.Scopes...),
		EnablePkce:     provider.EnablePkce,
		EnableProxy:    provider.EnableProxy,
		HTTPHeaders:    provider.HttpHeaders,
		IdPEntityID:    provider.IdPEntityId,
		SsoURL:         provider.SsoUrl,
		SAMLCert:       provider.Cert,
		SAMLMetadata:   provider.Metadata,
	}
}

// exchangeSocialCode is a function variable that exchanges an OAuth code for user info.
// It is wired to the actual idp implementation in the bootstrap layer.
var exchangeSocialCode = defaultExchangeSocialCode

// getSocialRedirectURL is a function variable that generates the redirect URL for a social provider.
// It is wired to the actual idp implementation in the bootstrap layer.
var getSocialRedirectURL = defaultGetSocialRedirectURL

func defaultExchangeSocialCode(_ context.Context, _ SocialProviderConfig, _ string) (*SocialUserInfo, error) {
	return nil, apperrors.NewUnimplemented("social login not configured: exchangeSocialCode not wired")
}

func defaultGetSocialRedirectURL(_ context.Context, _ SocialProviderConfig, _ string) (string, error) {
	return "", apperrors.NewUnimplemented("social login not configured: getSocialRedirectURL not wired")
}

// LoginWithProvider handles the OAuth callback for social login.
// Flow: exchange code → get user info → match/create user → create session → handleLoggedIn.
func (s *Service) LoginWithProvider(ctx context.Context, command SocialLoginCommand) (*SocialLoginResult, error) {
	var result *SocialLoginResult
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		loginResult, err := s.loginWithProviderInTx(ctx, tx, command)
		if err != nil {
			return err
		}
		result = loginResult
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.registeredUser != nil {
		s.dispatchPostRegistration(ctx, result.registeredUser.OrganizationID, result.registeredUser.ID,
			result.registeredUser.Username, result.registeredApplicationID, "Social Auto Register")
	}
	return result, nil
}

// loginWithProviderInTx is the transaction-scoped body of LoginWithProvider.
// The SAML browser ACS and the legacy JSON social-login endpoint both enter
// here, so SAML state validation has exactly one enforcement point.
func (s *Service) loginWithProviderInTx(ctx context.Context, tx appuow.UnitOfWork, command SocialLoginCommand) (*SocialLoginResult, error) {
	if strings.TrimSpace(command.ProviderID) == "" {
		return nil, apperrors.NewInvalidArgument("providerId is required")
	}
	if strings.TrimSpace(command.Code) == "" {
		return nil, apperrors.NewInvalidArgument("authorization code is required")
	}

	now := s.Clock.Now()

	// 1. Load the provider configuration from the database.
	provider, err := tx.Providers().GetByID(ctx, command.ProviderID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "load provider")
	}
	if err := ensureProviderUsable(provider); err != nil {
		return nil, err
	}
	// SAML responses must carry a server-issued, one-time state. The guard is
	// a no-op for every non-SAML provider, preserving the existing OAuth/OIDC
	// state handling and exchange flow.
	if err := s.consumeSAMLProviderState(ctx, tx, provider, command); err != nil {
		return nil, err
	}
	// 2. Exchange the authorization code for user info via the IDP adapter.
	redirectURI := firstNonEmpty(provider.CallbackUrl, provider.ProviderURL)
	providerConfig, err := s.socialProviderConfigForLogin(provider, redirectURI, command, now)
	if err != nil {
		return nil, err
	}
	providerConfig.State = strings.TrimSpace(command.State)
	userInfo, err := exchangeSocialCode(ctx, providerConfig, command.Code)
	if err != nil {
		// SAML failures are audited with request/issuer/reason only; the
		// assertion body and attributes are never persisted.
		if strings.EqualFold(string(provider.Category), "saml") {
			if auditErr := s.auditSAMLRejection(ctx, tx, command, provider, err); auditErr != nil {
				return nil, auditErr
			}
		}
		authErr := apperrors.WrapUnauthorized(err, "failed to authenticate with provider")
		if meta := samlFailureMeta(err); len(meta) > 0 {
			authErr.Meta = meta
		}
		return nil, authErr
	}

	if strings.TrimSpace(userInfo.ID) == "" {
		return nil, apperrors.NewInternal("provider returned empty user ID")
	}
	if err := validateProviderEmail(provider, userInfo.Email); err != nil {
		return nil, err
	}

	// 3. Resolve organization.
	organization, err := s.resolveSocialOrganization(ctx, tx, command, provider.OrganizationID)
	if err != nil {
		return nil, err
	}

	// 4. Resolve application.
	application, err := resolveApplicationReference(ctx, tx.Applications(), organization, command.ApplicationID)
	if err != nil {
		return nil, err
	}
	if application.OrganizationID != organization.ID {
		return nil, apperrors.NewInvalidArgument("application does not belong to organization")
	}
	providerItem := providerItemForApplication(application, provider)

	// 5. Try to find existing user by provider identity.
	var user *userdomain.User
	isNewUser := false

	user, err = tx.Users().GetByProviderIdentity(ctx, provider.ID, userInfo.ID)
	if err != nil {
		if !isNotFoundError(err) {
			return nil, err
		}
		// Not found — apply the provider item's binding rules.
		user, err = s.matchExistingSocialUser(ctx, tx, organization.ID, userInfo, providerItem)
		if err != nil {
			return nil, err
		}
	}

	// 6. If no matching user, auto-register if the organization allows it.
	if user == nil {
		if organization.DisableSignup {
			return nil, apperrors.NewForbidden("organization does not allow sign-up")
		}
		if organization.DisableSignin {
			return nil, apperrors.NewForbidden("organization has sign-in disabled")
		}

		user, err = s.autoRegisterSocialUser(ctx, tx, organization, provider.ID, string(provider.ProviderType), userInfo, now, providerItem)
		if err != nil {
			return nil, apperrors.WrapInternal(err, "auto-register social user")
		}
		isNewUser = true
	} else {
		// Update existing identity if needed.
		if err := s.updateSocialIdentity(ctx, tx, user, provider.ID, userInfo, now); err != nil {
			slog.WarnContext(ctx, "failed to update social identity", "error", err, "user_id", user.ID)
		}
	}

	// 7. Check if user is allowed to login.
	if !isUserAllowedToLogin(user.Status, user.IsForbidden) {
		return nil, apperrors.NewForbidden("user cannot sign in")
	}

	// ADR-029 pre_login (social channel): fail-closed policy gate before
	// the session is created. Provider code/assertion 不进入 action input。
	if gateErr := s.enforceActionsPreLogin(ctx, loginMethodSocial, organization.ID, user.ID, user.Username,
		application.ID, command.IPAddress, command.UserAgent); gateErr != nil {
		return nil, gateErr
	}

	// 8. Create session.
	sessionID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate session id")
	}

	effectiveTTL := sessionTTL
	if !application.EnableSigninSession {
		effectiveTTL = shortSessionTTL
	}

	sid, err := newSessionSid()
	if err != nil {
		return nil, err
	}
	authnContext := strings.TrimSpace(userInfo.AuthnContext)
	authLevel := socialAuthenticationLevel(authnContext)
	deviceInfo := s.socialSessionDeviceInfo(ctx, provider, userInfo)
	if authnContext != "" {
		deviceInfo["acr"] = authnContext
	}
	deviceInfo["amr"] = authLevel
	session := &sessiondomain.Session{
		ID:                  sessionID,
		Sid:                 sid,
		UserID:              user.ID,
		Username:            user.Username,
		OrganizationID:      organization.ID,
		OrganizationSlug:    organization.Slug,
		ApplicationID:       application.ID,
		ApplicationSlug:     application.Slug,
		SessionState:        statusActive,
		DeviceInfo:          deviceInfo,
		IPAddress:           strings.TrimSpace(command.IPAddress),
		UserAgent:           strings.TrimSpace(command.UserAgent),
		ExpiresAt:           now.Add(effectiveTTL),
		CreatedAt:           now,
		LastSeenAt:          now,
		UpdatedAt:           now,
		Revision:            1,
		AuthenticationLevel: authLevel,
	}
	if err := tx.Sessions().Create(ctx, session); err != nil {
		return nil, err
	}

	if err := s.embedAuditOutboxPlain(
		tx,
		ctx,
		"",
		"",
		"",
		session.OrganizationID,
		session.UserID,
		"session.created",
		"session",
		session.ID,
		"session",
		"session.created",
		map[string]any{},
	); err != nil {
		return nil, err
	}
	// S9: revoke old session carried by request (session fixation prevention)
	if strings.TrimSpace(command.PreviousSessionID) != "" {
		// Not best-effort: a failed revoke rolls the whole login back instead of
		// leaving the old session alive next to the freshly issued one.
		if err := revokePreviousSessionForRotation(ctx, tx.Sessions(), command.PreviousSessionID, now); err != nil {
			return nil, err
		}
		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"",
			"",
			"",
			"session.revoked",
			"session",
			command.PreviousSessionID,
			"session",
			"session.revoked",
			map[string]any{},
		); err != nil {
			return nil, err
		}
	}

	// 登录方式无关的会话策略与 signin 统计（排他登录 / 并发上限 / LastSignin*）。
	if err := s.applyLoginSessionPolicies(ctx, tx, user, application, organization.ID, session, now); err != nil {
		return nil, err
	}
	// 9. Generate login response (reuse handleLoggedIn for code/token/SAML flows).
	loginCommand := LoginCommand{
		OrganizationSlug: organization.Slug,
		Username:         user.Username,
		ApplicationID:    application.ID,
		ResponseType:     command.ResponseType,
		RedirectURI:      command.RedirectURI,
		State:            command.State,
		Nonce:            command.Nonce,
		Scope:            command.Scope,
		IPAddress:        command.IPAddress,
		UserAgent:        command.UserAgent,
	}

	loginResponse, err := s.handleLoggedIn(ctx, tx, loginCommand, organization, application, user, sid, now)
	if err != nil {
		return nil, err
	}

	socialResult := &SocialLoginResult{
		LoginResult: LoginResult{SessionID: sessionID, Response: *loginResponse},
		IsNewUser:   isNewUser,
	}
	if isNewUser {
		socialResult.registeredUser = user
		socialResult.registeredApplicationID = application.ID
	}
	return socialResult, nil
}

// GetSocialRedirectURL generates the redirect URL for a social login provider.
func (s *Service) GetSocialRedirectURL(ctx context.Context, command SocialAuthorizeCommand) (string, error) {
	if strings.TrimSpace(command.ProviderID) == "" {
		return "", apperrors.NewInvalidArgument("providerId is required")
	}

	provider, err := s.UOW.Providers().GetByID(ctx, command.ProviderID)
	if err != nil {
		return "", apperrors.WrapInternal(err, "load provider")
	}
	if err := ensureProviderUsable(provider); err != nil {
		return "", err
	}

	redirectURI := firstNonEmpty(provider.CallbackUrl, provider.ProviderURL)

	// SAML must never trust a caller-supplied state: the SP browser ACS only
	// accepts a state this server signed for this provider/organization, so the
	// Authorization command's state (if any) is ignored on the SAML branch.
	state := strings.TrimSpace(command.State)
	if isSAMLProvider(provider) {
		orgSlug, err := s.samlProviderOrganizationSlug(ctx, provider, command.OrganizationSlug)
		if err != nil {
			return "", err
		}
		state, err = s.signSAMLSPState(provider.ID, orgSlug)
		if err != nil {
			return "", err
		}
	} else if state == "" {
		state, err = generateSocialState()
		if err != nil {
			return "", apperrors.NewInternal("generate state")
		}
	}

	providerConfig := socialProviderConfig(provider, redirectURI)
	if isGenericOIDCProvider(provider) {
		if err := s.bindSocialOIDCState(provider, state, &providerConfig, s.Clock.Now()); err != nil {
			return "", err
		}
	}

	redirectURL, err := getSocialRedirectURL(ctx, providerConfig, state)
	if err != nil {
		return "", apperrors.WrapInternal(err, "get redirect URL")
	}

	return redirectURL, nil
}

// auditSAMLRejection records a failed SAML assertion consumption in the same
// transaction that aborted the login. Only the AuthnRequest ID, issuer and
// rejection reason are persisted — never the assertion XML or attributes.
func (s *Service) auditSAMLRejection(
	ctx context.Context,
	tx appuow.UnitOfWork,
	command SocialLoginCommand,
	provider *providerdomain.Provider,
	failure error,
) error {
	meta := samlFailureMeta(failure)
	reason := "saml_response_rejected"
	if value := metaString(meta, "samlReason"); value != "" {
		reason = value
	}

	payload := map[string]any{
		"providerId":   provider.ID,
		"providerType": string(provider.ProviderType),
		"reason":       reason,
	}
	if issuer := metaString(meta, "samlIssuer"); issuer != "" {
		payload["issuer"] = issuer
	}
	requestID := metaString(meta, "samlRequestId")
	if requestID != "" {
		payload["requestId"] = requestID
	}

	return s.embedAuditEvent(
		tx, ctx,
		command.IPAddress, command.UserAgent, "auth", requestID, "",
		provider.OrganizationID, "",
		audit.EventAuthLoginFailed, "provider", provider.ID,
		"provider", audit.EventAuthLoginFailed,
		payload,
	)
}

// samlFailureMeta copies the allow-listed SAML rejection metadata attached by
// infra/saml. Unknown keys (and any assertion content) are discarded.
func samlFailureMeta(err error) map[string]any {
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr == nil || len(appErr.Meta) == 0 {
		return nil
	}
	meta := make(map[string]any)
	for _, key := range []string{"samlReason", "samlIssuer", "samlRequestId"} {
		if value, ok := appErr.Meta[key]; ok {
			meta[key] = value
		}
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

func metaString(meta map[string]any, key string) string {
	value, ok := meta[key].(string)
	if !ok {
		return ""
	}
	return value
}

// resolveSocialOrganization resolves the organization for a social login.
func (s *Service) resolveSocialOrganization(ctx context.Context, tx appuow.UnitOfWork, command SocialLoginCommand, providerOrgID string) (*orgdomain.Organization, error) {
	// Try the command's organization slug first.
	if command.OrganizationSlug != "" {
		org, err := tx.Organizations().GetBySlug(ctx, command.OrganizationSlug)
		if err != nil {
			return nil, apperrors.WrapInternal(err, "load organization")
		}
		return org, nil
	}

	// Fallback: use the provider's organization ID.
	if providerOrgID != "" {
		orgs, _, err := tx.Organizations().List(ctx, orgdomain.ListQuery{PageSize: 100})
		if err != nil {
			return nil, apperrors.WrapInternal(err, "list organizations")
		}
		for _, org := range orgs {
			if org.ID == providerOrgID {
				return org, nil
			}
		}
	}

	return nil, apperrors.NewInvalidArgument("cannot determine organization for social login")
}

// matchUserByEmail tries to find a user by email in the organization.
func (s *Service) matchUserByEmail(ctx context.Context, tx appuow.UnitOfWork, organizationID, email string) (*userdomain.User, error) {
	if email == "" {
		return nil, apperrors.NewInvalidArgument("empty email")
	}

	users, _, err := tx.Users().List(ctx, userdomain.ListQuery{
		OrganizationID: organizationID,
		PageSize:       100,
		SearchField:    contactEmail,
		SearchValue:    email,
	})
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		if u.Profile.Email == email {
			return u, nil
		}
	}

	return nil, apperrors.NewNotFound("user not found by email")
}

// autoRegisterSocialUser creates a new user from social login info and links the identity.
func (s *Service) autoRegisterSocialUser(ctx context.Context, tx appuow.UnitOfWork, org *orgdomain.Organization, providerID, providerType string, userInfo *SocialUserInfo, now time.Time, providerItem *applicationdomain.ProviderItem) (*userdomain.User, error) {
	userID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate user id")
	}

	// Generate a unique username based on provider + external ID.
	username := generateSocialUsername(providerType, userInfo)

	user := &userdomain.User{
		ID:             userID,
		OrganizationID: org.ID,
		Username:       username,
		UserType:       userTypeNormal,
		Status:         statusActive,
		IsAdmin:        false,
		IsForbidden:    false,
		Profile: userdomain.UserProfile{
			DisplayName: firstNonEmpty(userInfo.DisplayName, userInfo.Username, username),
			Email:       userInfo.Email,
			Phone:       userInfo.Phone,
			AvatarURL:   userInfo.AvatarURL,
		},
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
		Identities: []userdomain.UserIdentity{
			{
				ProviderID:      providerID,
				IdentityType:    loginMethodSocial,
				Subject:         userInfo.ID,
				ProviderAccount: firstNonEmpty(userInfo.Username, userInfo.DisplayName, userInfo.Email),
				ProfileSnapshot: map[string]any{
					contactEmail:     userInfo.Email,
					fieldDisplayName: userInfo.DisplayName,
					fieldAvatarURL:   userInfo.AvatarURL,
				},
				LinkedAt: now,
			},
		},
	}

	if err := tx.Users().Create(ctx, user); err != nil {
		return nil, apperrors.WrapInternal(err, "create user")
	}

	if providerItem != nil {
		if err := s.assignSignupGroup(ctx, tx, user.ID, org.ID, providerItem.SignupGroup); err != nil {
			return nil, apperrors.WrapInternal(err, "assign provider signup group")
		}
	}

	return user, nil
}

// updateSocialIdentity updates the identity record if the external info has changed.
func (s *Service) updateSocialIdentity(ctx context.Context, tx appuow.UnitOfWork, user *userdomain.User, providerID string, userInfo *SocialUserInfo, now time.Time) error {
	for i := range user.Identities {
		identity := &user.Identities[i]
		if identity.ProviderID == providerID && identity.Subject == userInfo.ID {
			// Update profile snapshot and provider account.
			user.Identities[i].ProfileSnapshot = map[string]any{
				contactEmail:     userInfo.Email,
				fieldDisplayName: userInfo.DisplayName,
				fieldAvatarURL:   userInfo.AvatarURL,
			}
			user.Identities[i].ProviderAccount = firstNonEmpty(userInfo.Username, userInfo.DisplayName, userInfo.Email)

			return tx.Users().ReplaceIdentities(ctx, user.ID, user.Identities)
		}
	}

	// Identity not found for this provider — link it.
	identityID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return err
	}

	newIdentity := userdomain.UserIdentity{
		ID:              identityID,
		UserID:          user.ID,
		ProviderID:      providerID,
		IdentityType:    loginMethodSocial,
		Subject:         userInfo.ID,
		ProviderAccount: firstNonEmpty(userInfo.Username, userInfo.DisplayName, userInfo.Email),
		ProfileSnapshot: map[string]any{
			contactEmail:     userInfo.Email,
			fieldDisplayName: userInfo.DisplayName,
			fieldAvatarURL:   userInfo.AvatarURL,
		},
		LinkedAt: now,
	}

	return tx.Users().CreateIdentity(ctx, &newIdentity)
}

// generateSocialUsername creates a unique username from provider info.
func generateSocialUsername(providerType string, userInfo *SocialUserInfo) string {
	base := firstNonEmpty(userInfo.Username, userInfo.DisplayName, userInfo.Email)
	if base == "" {
		return fmt.Sprintf("user_%s_%s", providerType[:minLen(8, len(providerType))], userInfo.ID[:minLen(8, len(userInfo.ID))])
	}

	// Sanitize: lowercase, replace non-alphanumeric with underscore.
	sanitized := strings.ToLower(base)
	sanitized = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, sanitized)

	// Truncate to reasonable length and append provider suffix for uniqueness.
	if len(sanitized) > 30 {
		sanitized = sanitized[:30]
	}

	return fmt.Sprintf("%s_%s", sanitized, userInfo.ID[:minLen(6, len(userInfo.ID))])
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// generateSocialState generates a random state string for CSRF protection.
func generateSocialState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SetSocialLoginAdapters wires the social login adapter functions.
// This should be called from the bootstrap layer to connect the app layer
// to the actual IDP implementations without creating a direct import dependency.
func SetSocialLoginAdapters(exchangeFn ExchangeSocialCodeFunc, redirectFn GetSocialRedirectURLFunc) {
	if exchangeFn != nil {
		exchangeSocialCode = exchangeFn
	}
	if redirectFn != nil {
		getSocialRedirectURL = redirectFn
	}
}

// ExchangeSocialCodeFunc is the signature for exchanging an OAuth code for user info.
type ExchangeSocialCodeFunc func(ctx context.Context, provider SocialProviderConfig, code string) (*SocialUserInfo, error)

// GetSocialRedirectURLFunc is the signature for generating a social login redirect URL.
type GetSocialRedirectURLFunc func(ctx context.Context, provider SocialProviderConfig, state string) (string, error)
