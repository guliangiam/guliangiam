package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	deviceauthdomain "github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	deviceCodeLength         = 32
	userCodeCharset          = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	userCodeRawLength        = 8
	deviceCodeTTL            = 15 * time.Minute
	deviceCodePollInterval   = 5 * time.Second
	deviceCodeAccessTokenTTL = 1 * time.Hour
)

// DeviceAuthorizeCommand contains parameters for the device authorization endpoint.
type DeviceAuthorizeCommand struct {
	ClientID string
	Scope    string
	// IssuerURL resolves the absolute verification URI (RFC 8628 §3.3.1).
	IssuerURL string
}

// DeviceAuthorizeResult contains the device_code and user_code.
type DeviceAuthorizeResult struct {
	DeviceCode              string `json:"deviceCode"`
	UserCode                string `json:"userCode"`
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete,omitempty"`
	ExpiresIn               int64  `json:"expiresIn"`
	Interval                int64  `json:"interval"`
}

// DeviceTokenCommand contains parameters for the device token endpoint.
type DeviceTokenCommand struct {
	ClientID     string
	ClientSecret string
	DeviceCode   string
}

// DeviceVerifyCommand contains parameters for user verification of a device code.
type DeviceVerifyCommand struct {
	UserCode       string
	OrganizationID string
	UserID         string
	Username       string
}

// DeviceCancelCommand contains parameters for cancelling a device authorization.
type DeviceCancelCommand struct {
	DeviceCode string
	ClientID   string
}

// DeviceAuthorize handles the device authorization request (RFC 8628 §3.1).
func (s *Service) DeviceAuthorize(ctx context.Context, command DeviceAuthorizeCommand) (*DeviceAuthorizeResult, error) {
	if strings.TrimSpace(command.ClientID) == "" {
		return nil, apperrors.NewInvalidArgument("client_id is required")
	}

	deviceCode, err := generateDeviceCode()
	if err != nil {
		return nil, apperrors.NewInternal("generate device code")
	}

	userCode, err := generateUserCode()
	if err != nil {
		return nil, apperrors.NewInternal("generate user code")
	}

	now := s.Clock.Now()
	codeID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate device auth code id")
	}

	entry := &deviceauthdomain.DeviceAuthCode{
		ID:              codeID,
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		ClientID:        strings.TrimSpace(command.ClientID),
		Scope:           strings.TrimSpace(command.Scope),
		ExpiresAt:       now.Add(deviceCodeTTL),
		PollIntervalSec: int(deviceCodePollInterval / time.Second),
		CreatedAt:       now,
		UpdatedAt:       now,
		Revision:        1,
	}

	if err := s.UOW.DeviceAuthCodes().Create(ctx, entry); err != nil {
		return nil, apperrors.WrapInternal(err, "create device authorization code")
	}

	return &DeviceAuthorizeResult{
		DeviceCode: deviceCode,
		UserCode:   userCode,
		// RFC 8628 §3.3.1 requires an absolute URL the user can type into a
		// browser they are already using. A relative "/device" was not usable
		// and left the client to guess the host.
		VerificationURI:         verificationURI(s.resolveIssuer(command.IssuerURL)),
		VerificationURIComplete: verificationURIComplete(s.resolveIssuer(command.IssuerURL), userCode),
		ExpiresIn:               int64(deviceCodeTTL / time.Second),
		Interval:                int64(deviceCodePollInterval / time.Second),
	}, nil
}

// DeviceToken handles the device token request (RFC 8628 §3.4).
func (s *Service) DeviceToken(ctx context.Context, command DeviceTokenCommand) (*ExchangeAuthorizationCodeResponse, error) {
	if strings.TrimSpace(command.DeviceCode) == "" {
		return nil, apperrors.NewInvalidArgument("device_code is required")
	}

	entry, err := s.UOW.DeviceAuthCodes().GetByDeviceCode(ctx, command.DeviceCode)
	if err != nil {
		// RFC 8628 §3.5: an unknown device_code is `expired_token` (the previous
		// `invalid_device_code` was a private code clients could not handle).
		return nil, newDeviceProtocolError(deviceErrorExpiredToken, "device code is invalid or expired")
	}

	now := s.Clock.Now()
	if now.After(entry.ExpiresAt) {
		_ = s.UOW.DeviceAuthCodes().DeleteByDeviceCode(ctx, entry.DeviceCode)
		return nil, newDeviceProtocolError(deviceErrorExpiredToken, "device code has expired")
	}

	if entry.ClientID != strings.TrimSpace(command.ClientID) {
		return nil, apperrors.NewUnauthorized("invalid_client")
	}

	// RFC 8628 §3.5 throttle. Polling faster than `interval` earns `slow_down`,
	// and a client that keeps doing it is terminated: without this, one
	// attacker-chosen device_code value could be polled in a tight loop for the
	// life of the grant.
	if err := s.enforceDevicePollInterval(ctx, entry, now); err != nil {
		return nil, err
	}

	if !entry.Verified {
		return nil, newDeviceProtocolError(deviceErrorAuthorizationPending, "authorization is pending")
	}

	// Consume the entry to prevent replay.
	_ = s.UOW.DeviceAuthCodes().DeleteByDeviceCode(ctx, entry.DeviceCode)

	var response *ExchangeAuthorizationCodeResponse
	// ADR-029 门禁错误必须保持协议错误类型：外层 WrapInternal 会把
	// invalid_grant/access_denied 变成 500，因此单独接住、事务后原样返回。
	var actionsGateErr error
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		organization, err := tx.Organizations().GetByID(ctx, entry.OrganizationID)
		if err != nil {
			return err
		}
		user, err := tx.Users().GetByID(ctx, entry.UserID)
		if err != nil {
			return err
		}
		application, err := tx.Applications().GetByClientID(ctx, entry.ClientID)
		if err != nil {
			return err
		}

		// ── Risk engine evaluation (device_flow channel) ──────────────
		// device code is already verified (user passed the browser grant page),
		// but the token mint must still be gated by risk — the device_code could
		// have been stolen between verification and polling, or the user's
		// environment changed. Device flow has no step-up path, so challenge
		// is treated as deny.
		uid := user.ID
		evalErr := s.evaluateLoginRisk(ctx, tx, "device_flow", organization.ID, &uid,
			"", "", "", map[string]any{"device_code": entry.DeviceCode})
		switch evalErr.(type) {
		case nil: // allow
		case *RiskDeniedError, *RiskChallengeError:
			return apperrors.NewUnauthorized("access denied by risk assessment")
		default:
			return evalErr // engine unavailable → fail-closed
		}

		// ADR-029 pre_login (device_flow channel): device code verified,
		// fail-closed policy gate before any token is minted.
		if gateErr := s.enforceActionsPreLogin(ctx, "device_flow", organization.ID, user.ID, user.Username,
			application.ID, "", ""); gateErr != nil {
			actionsGateErr = gateErr
			return gateErr
		}

		accessTokenValue, accessTokenPrefix, accessTokenHash, err := s.newAccessTokenSecret()
		if err != nil {
			return err
		}
		accessTokenID, err := s.UUIDGenerator.NewString()
		if err != nil {
			return apperrors.NewInternal("generate access token id")
		}

		scopes := strings.Fields(entry.Scope)
		if len(scopes) == 0 {
			scopes = []string{"openid", "profile", contactEmail}
		}

		// ADR-029 pre_token: fail-closed gate immediately before issuance.
		// RFC 8628 §3.5 style: deny renders access_denied.
		if gateErr := s.enforceActionsPreToken(ctx, organization.ID, user.ID, user.Username, application.ID,
			"urn:ietf:params:oauth:grant-type:device_code", scopes,
			newDeviceProtocolError(deviceErrorAccessDenied, "token issuance denied by policy")); gateErr != nil {
			actionsGateErr = gateErr
			return gateErr
		}

		accessToken := &tokendomain.Token{
			ID:               accessTokenID,
			UserID:           user.ID,
			Username:         user.Username,
			OrganizationID:   organization.ID,
			OrganizationSlug: organization.Slug,
			ApplicationID:    application.ID,
			ApplicationSlug:  application.Slug,
			TokenPrefix:      accessTokenPrefix,
			TokenHash:        accessTokenHash,
			Scopes:           scopes,
			Audience:         []string{},
			Metadata:         map[string]any{paramGrantType: "urn:ietf:params:oauth:grant-type:device_code"},
			ExpiresAt:        now.Add(deviceCodeAccessTokenTTL),
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if err := tx.Tokens().Create(ctx, accessToken); err != nil {
			return err
		}

		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"",
			accessToken.OrganizationID,
			accessToken.UserID,
			"token.created",
			"token",
			accessToken.ID,
			"token",
			"token.created",
			map[string]any{},
		); err != nil {
			return err
		}
		response = &ExchangeAuthorizationCodeResponse{
			Status:      paramIssued,
			AccessToken: accessTokenValue,
			TokenType:   tokenTypeBearer,
			ExpiresIn:   int64(deviceCodeAccessTokenTTL / time.Second),
			Scope:       strings.Join(scopes, " "),
		}
		return nil
	})
	if actionsGateErr != nil {
		return nil, actionsGateErr
	}
	if err != nil {
		return nil, apperrors.WrapInternal(err, "device token transaction")
	}

	return response, nil
}

// DeviceVerify handles user verification of a device code (RFC 8628 §3.3).
func (s *Service) DeviceVerify(ctx context.Context, command DeviceVerifyCommand) error {
	if strings.TrimSpace(command.UserCode) == "" {
		return apperrors.NewInvalidArgument("user_code is required")
	}

	entry, err := s.UOW.DeviceAuthCodes().GetByUserCode(ctx, command.UserCode)
	if err != nil {
		return apperrors.NewInvalidArgument("invalid user_code")
	}

	now := s.Clock.Now()
	if now.After(entry.ExpiresAt) {
		return apperrors.NewInvalidArgument("user_code has expired")
	}

	if entry.Verified {
		return apperrors.NewConflict("user_code already verified")
	}

	entry.Verified = true
	entry.VerifiedAt = &now
	entry.UserID = command.UserID
	entry.Username = command.Username
	entry.OrganizationID = command.OrganizationID
	entry.UpdatedAt = now

	if err := s.UOW.DeviceAuthCodes().UpdateVerification(ctx, entry); err != nil {
		return apperrors.WrapInternal(err, "update device auth verification")
	}
	return nil
}

// CancelDeviceAuthorization cancels a pending device authorization (RFC 8628 extension).
func (s *Service) CancelDeviceAuthorization(ctx context.Context, command DeviceCancelCommand) error {
	if strings.TrimSpace(command.DeviceCode) == "" {
		return apperrors.NewInvalidArgument("device_code is required")
	}

	entry, err := s.UOW.DeviceAuthCodes().GetByDeviceCode(ctx, command.DeviceCode)
	if err != nil {
		return apperrors.NewNotFound("device_code not found")
	}

	if entry.ClientID != strings.TrimSpace(command.ClientID) {
		return apperrors.NewUnauthorized("invalid_client")
	}

	if entry.Verified {
		return apperrors.NewConflict("device_code already verified, cannot cancel")
	}

	if err := s.UOW.DeviceAuthCodes().DeleteByDeviceCode(ctx, command.DeviceCode); err != nil {
		return apperrors.WrapInternal(err, "cancel device authorization")
	}
	return nil
}

func generateDeviceCode() (string, error) {
	b := make([]byte, deviceCodeLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateUserCode() (string, error) {
	b := make([]byte, userCodeRawLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = userCodeCharset[b[i]%byte(len(userCodeCharset))]
	}
	code := string(b)
	return code[:4] + "-" + code[4:], nil
}
