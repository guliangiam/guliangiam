package auth

import (
	"context"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
)

// TokenVerifier 是本包对 JWT 验证能力的本地接口依赖。
// 由 bootstrap/wire_transport.go 注入 infra/jwt.Issuer 具体实现。
// 定义在 transport 包内而非 domain，避免 transport→domain 穿透。
type TokenVerifier interface {
	ValidateToken(tokenString string) (map[string]interface{}, error)
}

type Handler struct {
	Service   *appauth.Service
	JWTIssuer TokenVerifier
	// ImpersonationResolver overrides Service for tests/tools that need an
	// opaque-token resolver without a full app service.
	ImpersonationResolver ImpersonationSessionResolver
}

func (h Handler) GetAccount(ctx context.Context, request GetAccountRequest) (*AccountResponse, error) {
	response, err := h.Service.GetAccount(ctx, request.toQuery())
	if err != nil {
		return nil, err
	}

	typed := AccountResponse(*response)
	return &typed, nil
}

func (h Handler) Login(ctx context.Context, command appauth.LoginCommand) (*LoginResponse, string, error) {
	result, err := h.Service.Login(ctx, command)
	if err != nil {
		return nil, "", err
	}

	typed := LoginResponse(result.Response)
	typed.RememberMeToken = result.RememberMeToken
	typed.RememberMeExpiresAt = result.RememberMeExpiresAt
	return &typed, result.SessionID, nil
}

func (h Handler) Authorize(ctx context.Context, command appauth.AuthorizeCommand) (*AuthorizeResponse, error) {
	response, err := h.Service.Authorize(ctx, command)
	if err != nil {
		return nil, err
	}
	typed := AuthorizeResponse(*response)
	return &typed, nil
}

func (h Handler) ExchangeAuthorizationCode(ctx context.Context, command appauth.ExchangeAuthorizationCodeCommand) (*ExchangeAuthorizationCodeResponse, error) {
	response, err := h.Service.ExchangeAuthorizationCode(ctx, command)
	if err != nil {
		return nil, err
	}
	typed := ExchangeAuthorizationCodeResponse(*response)
	return &typed, nil
}

func (h Handler) ExchangeRefreshToken(ctx context.Context, command appauth.ExchangeRefreshTokenCommand) (*ExchangeAuthorizationCodeResponse, error) {
	response, err := h.Service.ExchangeRefreshToken(ctx, command)
	if err != nil {
		return nil, err
	}
	typed := ExchangeAuthorizationCodeResponse(*response)
	return &typed, nil
}

func (h Handler) ExchangeClientCredentials(ctx context.Context, command appauth.ExchangeClientCredentialsCommand) (*ExchangeAuthorizationCodeResponse, error) {
	response, err := h.Service.ExchangeClientCredentials(ctx, command)
	if err != nil {
		return nil, err
	}
	typed := ExchangeAuthorizationCodeResponse(*response)
	return &typed, nil
}

func (h Handler) GetLoginChallenge(ctx context.Context, command appauth.LoginChallengeCommand) (*LoginChallengeResponse, error) {
	response, err := h.Service.GetLoginChallenge(ctx, command)
	if err != nil {
		return nil, err
	}
	typed := LoginChallengeResponse(*response)
	return &typed, nil
}

func (h Handler) ApproveLoginChallenge(ctx context.Context, command appauth.LoginChallengeCommand) (*LoginChallengeResponse, error) {
	response, err := h.Service.ApproveLoginChallenge(ctx, command)
	if err != nil {
		return nil, err
	}
	typed := LoginChallengeResponse(*response)
	return &typed, nil
}

func (h Handler) RejectLoginChallenge(ctx context.Context, command appauth.LoginChallengeCommand) (*LoginChallengeResponse, error) {
	response, err := h.Service.RejectLoginChallenge(ctx, command)
	if err != nil {
		return nil, err
	}
	typed := LoginChallengeResponse(*response)
	return &typed, nil
}

func (h Handler) BuildFrontChannelLogout(ctx context.Context, sessionID, issuerURL string) (*appauth.FrontChannelLogoutResult, error) {
	return h.Service.BuildFrontChannelLogout(ctx, sessionID, issuerURL)
}

func (h Handler) Logout(ctx context.Context, command appauth.LogoutCommand) error {
	return h.Service.Logout(ctx, command)
}

func (h Handler) GrantConsent(ctx context.Context, command appauth.GrantConsentCommand) (*ConsentResponse, error) {
	response, err := h.Service.GrantConsent(ctx, command)
	if err != nil {
		return nil, err
	}
	typed := ConsentResponse(*response)
	return &typed, nil
}

func (h Handler) EvaluateConsent(ctx context.Context, command appauth.EvaluateConsentCommand) (*ConsentEvaluationResponse, error) {
	response, err := h.Service.EvaluateConsent(ctx, command)
	if err != nil {
		return nil, err
	}
	typed := ConsentEvaluationResponse(*response)
	return &typed, nil
}

func (h Handler) RevokeConsent(ctx context.Context, command appauth.RevokeConsentCommand) (*ConsentResponse, error) {
	response, err := h.Service.RevokeConsent(ctx, command)
	if err != nil {
		return nil, err
	}
	typed := ConsentResponse(*response)
	return &typed, nil
}

func (h Handler) ResetPassword(ctx context.Context, command appauth.ResetPasswordCommand) (*appauth.ResetPasswordResult, string, error) {
	return h.Service.ResetPassword(ctx, command)
}

func (h Handler) CheckPassword(ctx context.Context, command appauth.CheckPasswordCommand) (*appauth.CheckPasswordResult, error) {
	return h.Service.CheckPassword(ctx, command)
}

func (h Handler) Signup(ctx context.Context, command appauth.SignupCommand) (*appauth.SignupResult, error) {
	return h.Service.Signup(ctx, command)
}

func (h Handler) DeviceAuthorize(ctx context.Context, command appauth.DeviceAuthorizeCommand) (*appauth.DeviceAuthorizeResult, error) {
	return h.Service.DeviceAuthorize(ctx, command)
}

func (h Handler) DeviceToken(ctx context.Context, command appauth.DeviceTokenCommand) (*ExchangeAuthorizationCodeResponse, error) {
	response, err := h.Service.DeviceToken(ctx, command)
	if err != nil {
		return nil, err
	}
	typed := ExchangeAuthorizationCodeResponse(*response)
	return &typed, nil
}

func (h Handler) DeviceVerify(ctx context.Context, command appauth.DeviceVerifyCommand) error {
	return h.Service.DeviceVerify(ctx, command)
}

func (h Handler) CancelDeviceAuthorization(ctx context.Context, command appauth.DeviceCancelCommand) error {
	return h.Service.CancelDeviceAuthorization(ctx, command)
}

func (h Handler) ExitImpersonate(ctx context.Context, orgSlug string, username string) error {
	return h.Service.ExitImpersonate(ctx, orgSlug, username)
}

// ResolveSessionContextForClient resolves a session with request IP/User-Agent
// so the app layer can enforce optional client binding (ADR-013).
func (h Handler) ResolveSessionContextForClient(ctx context.Context, sessionID, clientIP, clientUserAgent string) (*SessionContext, error) {
	identity, err := h.Service.ResolveSessionContextForClient(ctx, sessionID, clientIP, clientUserAgent)
	if err != nil {
		return nil, err
	}
	if identity == nil {
		return nil, nil
	}
	return &SessionContext{
		ID:               identity.ID,
		UserID:           identity.UserID,
		OrganizationID:   identity.OrganizationID,
		OrganizationSlug: identity.OrganizationSlug,
		Username:         identity.Username,
		AccessToken:      identity.AccessToken,
		Source:           "repository",
		AuthTime:         identity.AuthTime,
		Sid:              identity.Sid,
		IsPrivileged:     identity.IsPrivileged,
		LifecycleID:      identity.LifecycleID,
		TicketID:         identity.TicketID,
	}, nil
}

func (h Handler) ResolveSessionContext(ctx context.Context, sessionID string) (*SessionContext, error) {
	identity, err := h.Service.ResolveSessionContext(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return &SessionContext{
		ID:               identity.ID,
		UserID:           identity.UserID,
		OrganizationID:   identity.OrganizationID,
		OrganizationSlug: identity.OrganizationSlug,
		Username:         identity.Username,
		AccessToken:      identity.AccessToken,
		Source:           "repository",
		AuthTime:         identity.AuthTime,
		Sid:              identity.Sid,
		IsPrivileged:     identity.IsPrivileged,
		LifecycleID:      identity.LifecycleID,
		TicketID:         identity.TicketID,
	}, nil
}

func (h Handler) GetJwtBearerToken(ctx context.Context, command appauth.GetJwtBearerTokenCommand) (*ExchangeAuthorizationCodeResponse, error) {
	response, err := h.Service.GetJwtBearerToken(ctx, command)
	if err != nil {
		return nil, err
	}
	typed := ExchangeAuthorizationCodeResponse(*response)
	return &typed, nil
}

func (h Handler) GetTokenExchangeToken(ctx context.Context, command appauth.GetTokenExchangeCommand) (*appauth.TokenExchangeResponse, error) {
	return h.Service.GetTokenExchangeToken(ctx, command)
}

func (h Handler) ValidateDPoPProof(ctx context.Context, command appauth.ValidateDPoPProofCommand) (*appauth.DPoPProofResult, error) {
	return h.Service.ValidateDPoPProof(ctx, command)
}

// GenerateDPoPNonce returns a fresh one-time-use DPoP nonce per RFC 9449 §4.2.2.
// The client must echo this nonce in the `nonce` claim of its next DPoP proof.
func (h Handler) GenerateDPoPNonce(ctx context.Context) (*appauth.NonceResult, error) {
	return h.Service.GenerateNonce(ctx, appauth.GenerateNonceCommand{})
}

func (h Handler) IntrospectToken(ctx context.Context, command appauth.IntrospectTokenCommand) (*appauth.TokenIntrospectionResponse, error) {
	return h.Service.IntrospectToken(ctx, command)
}

func (h Handler) SendBackchannelLogout(ctx context.Context, command appauth.SendBackchannelLogoutCommand) (*appauth.BackchannelLogoutResult, error) {
	return h.Service.SendBackchannelLogout(ctx, command)
}

func (h Handler) AuthenticateClient(ctx context.Context, command appauth.AuthenticateClientCommand) (*appauth.ClientAuthResult, error) {
	return h.Service.AuthenticateClient(ctx, command)
}

func (h Handler) CompleteNativeSso(ctx context.Context, accessToken string, applicationID string) error {
	return h.Service.CompleteNativeSso(ctx, accessToken, applicationID)
}

func (h Handler) GetCurrentUserPermissions(ctx context.Context, username string, orgSlug string) (*appauth.UserPermissionSet, error) {
	return h.Service.GetCurrentUserPermissions(ctx, username, orgSlug)
}

// ─────────────────────────────────────────────
// QR Code Login
// ─────────────────────────────────────────────

func (h Handler) CreateQRCode(ctx context.Context, command appauth.CreateQRCodeCommand) (*appauth.CreateQRCodeResult, error) {
	return h.Service.CreateQRCode(ctx, command)
}

func (h Handler) ScanQRCode(ctx context.Context, command appauth.ScanQRCodeCommand) (*appauth.ScanQRCodeResult, string, error) {
	return h.Service.ScanQRCode(ctx, command)
}

func (h Handler) ConfirmQRCode(ctx context.Context, command appauth.ConfirmQRCodeCommand) error {
	return h.Service.ConfirmQRCode(ctx, command)
}

// UpdateAccount 是会话主体的自助资料更新/联系方式绑定（PATCH /api/v1/auth/account），
// 返回与 GET 同形的账号上下文，便于控制台就地刷新。
func (h Handler) UpdateAccount(ctx context.Context, command appauth.UpdateAccountCommand) (*AccountResponse, error) {
	return h.Service.UpdateAccount(ctx, command)
}

func (h Handler) PollQRCodeStatus(ctx context.Context, codeID string) (*appauth.PollQRCodeResult, error) {
	return h.Service.PollQRCodeStatus(ctx, codeID)
}

// ExchangeQRCode 与 /faceid/complete 同形：返回登录结果（含协议上下文产物）与会话 id，
// 传输层据此种会话 cookie（OPEN-API-17）。
func (h Handler) ExchangeQRCode(ctx context.Context, command appauth.ExchangeQRCodeCommand) (*LoginResponse, string, error) {
	result, sessionID, err := h.Service.ExchangeQRCode(ctx, command)
	if err != nil {
		return nil, "", err
	}
	return &result.Response, sessionID, nil
}

// ─────────────────────────────────────────────
// Face ID Login
// ─────────────────────────────────────────────

func (h Handler) InitiateFaceID(ctx context.Context, command appauth.InitiateFaceIDCommand) (*appauth.InitiateFaceIDResult, error) {
	return h.Service.InitiateFaceID(ctx, command)
}

func (h Handler) VerifyFaceID(ctx context.Context, command appauth.VerifyFaceIDCommand) (*appauth.VerifyFaceIDResult, error) {
	return h.Service.VerifyFaceID(ctx, command)
}

func (h Handler) CompleteFaceIDLogin(ctx context.Context, command appauth.CompleteFaceIDLoginCommand) (*LoginResponse, string, error) {
	result, err := h.Service.CompleteFaceIDLogin(ctx, command)
	if err != nil {
		return nil, "", err
	}
	typed := LoginResponse(result.Response)
	return &typed, result.SessionID, nil
}

// ─────────────────────────────────────────────
// Push MFA
// ─────────────────────────────────────────────

func (h Handler) InitiatePushMfa(ctx context.Context, command appauth.InitiatePushMfaCommand) (*appauth.InitiatePushMfaResult, error) {
	return h.Service.InitiatePushMfa(ctx, command)
}

func (h Handler) VerifyPushMfa(ctx context.Context, command appauth.VerifyPushMfaCommand) (*appauth.VerifyPushMfaResult, error) {
	return h.Service.VerifyPushMfa(ctx, command)
}

func (h Handler) EnablePushMfa(ctx context.Context, command appauth.EnablePushMfaCommand) (string, error) {
	return h.Service.EnablePushMfa(ctx, command)
}

func (h Handler) DisablePushMfa(ctx context.Context, command appauth.DisablePushMfaCommand) (string, error) {
	return h.Service.DisablePushMfa(ctx, command)
}

// ─────────────────────────────────────────────
// Social Login
// ─────────────────────────────────────────────

func (h Handler) LoginWithProvider(ctx context.Context, command appauth.SocialLoginCommand) (*LoginResponse, string, error) {
	result, err := h.Service.LoginWithProvider(ctx, command)
	if err != nil {
		return nil, "", err
	}

	typed := LoginResponse(result.Response)
	return &typed, result.SessionID, nil
}

func (h Handler) GetSocialRedirectURL(ctx context.Context, command appauth.SocialAuthorizeCommand) (string, error) {
	return h.Service.GetSocialRedirectURL(ctx, command)
}

func (h Handler) LoginWithLDAP(ctx context.Context, command appauth.LdapLoginCommand) (*LoginResponse, string, error) {
	result, err := h.Service.LoginWithLDAP(ctx, command)
	if err != nil {
		return nil, "", err
	}

	typed := LoginResponse(result.Response)
	return &typed, result.SessionID, nil
}

// ─────────────────────────────────────────────
// WebAuthn
// ─────────────────────────────────────────────

func (h Handler) WebAuthnRegistrationBegin(ctx context.Context, command appauth.WebAuthnRegistrationBeginCommand) (*appauth.WebAuthnRegistrationBeginResult, error) {
	if session, ok := SessionContextFromContext(ctx); ok {
		command.SessionUserID = session.UserID
		command.SessionOrganizationID = session.OrganizationID
	}
	return h.Service.WebAuthnRegistrationBegin(ctx, command)
}

func (h Handler) WebAuthnRegistrationFinish(ctx context.Context, command appauth.WebAuthnRegistrationFinishCommand) (*appauth.WebAuthnRegistrationFinishResult, string, error) {
	if session, ok := SessionContextFromContext(ctx); ok {
		command.SessionUserID = session.UserID
		command.SessionOrganizationID = session.OrganizationID
	}
	return h.Service.WebAuthnRegistrationFinish(ctx, command)
}

func (h Handler) WebAuthnAuthenticationBegin(ctx context.Context, command appauth.WebAuthnAuthenticationBeginCommand) (*appauth.WebAuthnAuthenticationBeginResult, error) {
	return h.Service.WebAuthnAuthenticationBegin(ctx, command)
}

func (h Handler) WebAuthnAuthenticationFinish(ctx context.Context, command appauth.WebAuthnAuthenticationFinishCommand) (*appauth.LoginResult, string, error) {
	return h.Service.WebAuthnAuthenticationFinish(ctx, command)
}

// ─────────────────────────────────────────────
// RFC 7009 Token Revocation
// ─────────────────────────────────────────────

func (h Handler) RevokeToken(ctx context.Context, command appauth.RevokeTokenCommand) error {
	return h.Service.RevokeToken(ctx, command)
}

// ─────────────────────────────────────────────
// RFC 7662 Token Introspection (RFC-compliant)
// ─────────────────────────────────────────────

func (h Handler) IntrospectTokenRFC(ctx context.Context, command appauth.IntrospectTokenRFCCommand) (*appauth.TokenIntrospectionRFCResponse, error) {
	return h.Service.IntrospectTokenRFC(ctx, command)
}

// GetCaptchaStatus checks if captcha is required for login in a given organization.
func (h Handler) GetCaptchaStatus(ctx context.Context, organizationID string) (*appauth.CaptchaStatusResult, error) {
	return h.Service.GetCaptchaStatus(ctx, organizationID)
}

// GenerateCaptcha generates a new CAPTCHA challenge for the given organization.
func (h Handler) GenerateCaptcha(ctx context.Context, organizationID string) (*appauth.CaptchaGenerateResult, error) {
	return h.Service.GenerateCaptcha(ctx, organizationID)
}

// VerifyCaptcha verifies a CAPTCHA response.
func (h Handler) VerifyCaptcha(ctx context.Context, organizationID string, token string, remoteIP string) (*appauth.CaptchaVerifyResult, error) {
	return h.Service.VerifyCaptcha(ctx, organizationID, token, remoteIP)
}

// GetMaskedContact returns masked contact info for a user (for password reset flow).
func (h Handler) GetMaskedContact(ctx context.Context, organizationSlug string, username string) (*appauth.MaskedContactResult, error) {
	return h.Service.GetMaskedContact(ctx, organizationSlug, username)
}

// VerifyCode checks a standalone verification code (email or SMS).
func (h Handler) VerifyCode(ctx context.Context, command appauth.VerifyCodeCommand) error {
	return h.Service.VerifyCode(ctx, command)
}

// SsoLogout performs single sign-out across all SSO-enabled applications.
func (h Handler) SsoLogout(ctx context.Context, sessionID string) (*appauth.SsoLogoutResult, error) {
	return h.Service.SsoLogout(ctx, sessionID)
}

// GetAppLogin returns the login page configuration for an application.
func (h Handler) GetAppLogin(ctx context.Context, orgSlug string, appSlug string) (*appauth.AppLoginResult, error) {
	return h.Service.GetAppLogin(ctx, orgSlug, appSlug)
}

// ChangePassword lets an authenticated user change their own password.
// AC-AUTH-009: credential-changing op → ForceLogoutUser full cascade.
func (h Handler) ChangePassword(ctx context.Context, command appauth.ChangePasswordCommand) (*appauth.ChangePasswordResult, string, error) {
	return h.Service.ChangePassword(ctx, command)
}
