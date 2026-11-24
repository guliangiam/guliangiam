package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	trusteddevicesdomain "github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	trustedDeviceChallengeTimeout = 5 * time.Minute
	trustedDeviceTrustDuration    = 30 * 24 * time.Hour
	trustedDeviceRelyingPartyID   = "trusted-device"
	trustedDeviceSignaturePrefix  = "trusted-device:v1:"
)

// TrustedDeviceView is the only shape returned to clients. Public keys and UA
// hashes stay server-side.
type TrustedDeviceView struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"userId"`
	DeviceCredentialID string     `json:"deviceCredentialId"`
	Status             string     `json:"status"`
	CreatedAt          time.Time  `json:"createdAt"`
	LastSeenAt         time.Time  `json:"lastSeenAt"`
	ExpiresAt          time.Time  `json:"expiresAt"`
	RevokedAt          *time.Time `json:"revokedAt,omitempty"`
}

type TrustedDeviceBeginCommand struct {
	OrganizationID string
	UserID         string
}

type TrustedDeviceBeginResult struct {
	ChallengeID string    `json:"challengeId"`
	Nonce       string    `json:"nonce"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type TrustedDeviceRegisterCommand struct {
	OrganizationID     string
	UserID             string
	ChallengeID        string
	Nonce              string
	DeviceCredentialID string
	DevicePublicKey    string
	Signature          string
	IPAddress          string
	UserAgent          string
}

// trustedDeviceTx is a narrow capability interface: appuow.AuthUoW deliberately
// does not declare TrustedDevices(), so expanding it (and every fake) is
// avoided. The concrete postgres UoW satisfies this assertion.
type trustedDeviceTx interface {
	TrustedDevices() trusteddevicesdomain.Repository
}

func trustedDeviceRepo(tx appuow.UnitOfWork) (trusteddevicesdomain.Repository, error) {
	return trustedDeviceRepoFrom(tx)
}

func trustedDeviceRepoFrom(src any) (trusteddevicesdomain.Repository, error) {
	if p, ok := src.(trustedDeviceTx); ok {
		if repo := p.TrustedDevices(); repo != nil {
			return repo, nil
		}
	}
	return nil, apperrors.NewInternal("trusted device repository unavailable")
}

func (s *Service) ListTrustedDevices(ctx context.Context, organizationID, userID string) ([]TrustedDeviceView, error) {
	if strings.TrimSpace(organizationID) == "" || strings.TrimSpace(userID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId and userId are required")
	}
	repo, err := trustedDeviceRepoFrom(s.UOW)
	if err != nil {
		return nil, err
	}
	devices, err := repo.ListByUser(ctx, organizationID, userID)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now().UTC()
	views := make([]TrustedDeviceView, 0, len(devices))
	for _, device := range devices {
		views = append(views, toTrustedDeviceView(device, now))
	}
	return views, nil
}

func (s *Service) BeginTrustedDeviceRegistration(ctx context.Context, command TrustedDeviceBeginCommand) (*TrustedDeviceBeginResult, error) {
	if strings.TrimSpace(command.OrganizationID) == "" || strings.TrimSpace(command.UserID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId and userId are required")
	}

	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, apperrors.NewInternal("generate trusted device nonce")
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)

	challengeID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate trusted device challenge id")
	}

	now := s.Clock.Now().UTC()
	expiresAt := now.Add(trustedDeviceChallengeTimeout)
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return tx.WebAuthnCredentials().CreateChallenge(ctx, &webauthndomain.Challenge{
			ID:             challengeID,
			ChallengeHash:  trustedDeviceChallengeHash(nonce),
			CeremonyType:   webauthndomain.CeremonyTypeAuthentication,
			UserID:         command.UserID,
			OrganizationID: command.OrganizationID,
			RelyingPartyID: trustedDeviceRelyingPartyID,
			ExpiresAt:      expiresAt,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		})
	}); err != nil {
		return nil, apperrors.NewInternal("store trusted device challenge")
	}

	return &TrustedDeviceBeginResult{
		ChallengeID: challengeID,
		Nonce:       nonce,
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *Service) RegisterTrustedDevice(ctx context.Context, command TrustedDeviceRegisterCommand) (*TrustedDeviceView, error) {
	if strings.TrimSpace(command.OrganizationID) == "" || strings.TrimSpace(command.UserID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId and userId are required")
	}
	if strings.TrimSpace(command.ChallengeID) == "" {
		return nil, apperrors.NewInvalidArgument("challengeId is required — call trusted-devices/begin first")
	}
	if strings.TrimSpace(command.Nonce) == "" {
		return nil, apperrors.NewInvalidArgument("nonce is required")
	}
	// 凭据强度在进入事务前校验：与摘要计算同一个入口，非法值直接 400，
	// 不消耗一次性挑战、不触达 SQL。
	credentialDigest, err := trustedDeviceCredentialDigest(command.DeviceCredentialID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(command.DevicePublicKey) == "" {
		return nil, apperrors.NewInvalidArgument("devicePublicKey is required")
	}
	if strings.TrimSpace(command.Signature) == "" {
		return nil, apperrors.NewInvalidArgument("signature is required")
	}

	publicKeyDER, err := trustedDeviceDecodeBase64(command.DevicePublicKey)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("invalid devicePublicKey encoding")
	}
	signature, err := trustedDeviceDecodeBase64(command.Signature)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("invalid signature encoding")
	}

	expectedChallengeHash := trustedDeviceChallengeHash(command.Nonce)
	message := trustedDeviceSignatureMessage(command.ChallengeID, command.UserID, command.Nonce)

	now := s.Clock.Now().UTC()
	var view TrustedDeviceView
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, repoErr := trustedDeviceRepo(tx)
		if repoErr != nil {
			return repoErr
		}

		// 一次性消费：过期、已消费、不存在都返回 NotFound。不得把仓储错误原文
		// 暴露给调用方（也避免泄露挑战是否存在）。
		challenge, consumeErr := tx.WebAuthnCredentials().ConsumeChallenge(ctx, command.ChallengeID, now)
		if consumeErr != nil {
			return apperrors.NewNotFound("trusted device challenge not found, already consumed, or expired")
		}
		if challenge.UserID != command.UserID ||
			challenge.OrganizationID != command.OrganizationID ||
			challenge.CeremonyType != webauthndomain.CeremonyTypeAuthentication ||
			challenge.RelyingPartyID != trustedDeviceRelyingPartyID {
			return apperrors.NewUnauthorized("challenge does not match this trusted device registration")
		}
		if subtle.ConstantTimeCompare([]byte(challenge.ChallengeHash), []byte(expectedChallengeHash)) != 1 {
			return apperrors.NewUnauthorized("challenge nonce does not match the one issued by begin")
		}

		if verifyErr := verifyTrustedDeviceSignature(publicKeyDER, message, signature); verifyErr != nil {
			return verifyErr
		}

		// 凭据只以 SHA-256 摘要进入持久化层与判定层；明文仅存在于本次请求内存中。
		// 摘要已在事务前由同一入口计算并校验强度。

		// 同一凭据重新注册时先吊销旧行，保证一个设备只有一条活跃信任记录。
		if revokeErr := repo.RevokeByCredential(ctx, command.OrganizationID, command.UserID, credentialDigest, now); revokeErr != nil {
			return revokeErr
		}

		deviceID, idErr := s.UUIDGenerator.NewString()
		if idErr != nil {
			return apperrors.NewInternal("generate trusted device id")
		}
		device := &trusteddevicesdomain.TrustedDevice{
			ID:                 deviceID,
			OrganizationID:     command.OrganizationID,
			UserID:             command.UserID,
			DeviceCredentialID: credentialDigest,
			DevicePublicKey:    publicKeyDER,
			UAHash:             trustedDeviceUAHash(command.UserAgent),
			CreatedAt:          now,
			LastSeenAt:         now,
			ExpiresAt:          now.Add(trustedDeviceTrustDuration),
			Revision:           1,
			UpdatedAt:          now,
		}
		if createErr := repo.Create(ctx, device); createErr != nil {
			return createErr
		}

		// 审计载荷只包含非敏感字段：绝不记录公钥、签名或 UA。
		if auditErr := s.embedAuditOutboxPlain(
			tx,
			ctx,
			command.IPAddress,
			command.UserAgent,
			"",
			command.OrganizationID,
			command.UserID,
			"auth.trusted_device.registered",
			"TrustedDevice",
			deviceID,
			"TrustedDevice",
			"auth.trusted_device.registered",
			// device_credential_id 只记掩码：不记明文，也不记完整摘要。
			map[string]any{
				"device_credential_id": trustedDeviceCredentialMask(credentialDigest),
				"expires_at":           device.ExpiresAt,
			},
		); auditErr != nil {
			return auditErr
		}

		view = toTrustedDeviceView(device, now)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func (s *Service) RevokeTrustedDevice(ctx context.Context, organizationID, userID, deviceID string) error {
	if strings.TrimSpace(organizationID) == "" || strings.TrimSpace(userID) == "" || strings.TrimSpace(deviceID) == "" {
		return apperrors.NewInvalidArgument("organizationId, userId and deviceId are required")
	}

	now := s.Clock.Now().UTC()
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, err := trustedDeviceRepo(tx)
		if err != nil {
			return err
		}
		device, err := repo.GetByID(ctx, deviceID)
		if err != nil {
			return err
		}
		// 跨租户/跨用户一律按不存在处理，避免泄露设备存在性。
		if device.OrganizationID != organizationID || device.UserID != userID {
			return apperrors.NewNotFound("trusted device not found")
		}
		// 幂等：已撤销的行不会再次写入（RowsAffected=0），重复调用仍返回成功。
		if err := repo.Revoke(ctx, organizationID, deviceID, now); err != nil {
			return err
		}
		return s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"",
			organizationID,
			userID,
			"auth.trusted_device.revoked",
			"TrustedDevice",
			deviceID,
			"TrustedDevice",
			"auth.trusted_device.revoked",
			map[string]any{},
		)
	})
}

// trustedDeviceSatisfiesMfa is the login-side check. Any error (missing repo,
// lookup failure, touch failure) returns false so the caller falls through to
// the normal MFA flow without surfacing the error.
func (s *Service) trustedDeviceSatisfiesMfa(ctx context.Context, tx appuow.UnitOfWork, organizationID, userID, credentialID, userAgent string, now time.Time) bool {
	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		return false
	}
	repo, err := trustedDeviceRepo(tx)
	if err != nil {
		slog.DebugContext(ctx, "trusted device repository unavailable; falling back to MFA")
		return false
	}
	// 先算摘要再查库：存量为明文的历史行（≠ 摘要）自然不命中，fail-closed 回退 MFA。
	// 不满足强度下限的凭据同样在这里失败，不进入仓储查询。
	digest, digestErr := trustedDeviceCredentialDigest(credentialID)
	if digestErr != nil {
		return false
	}
	device, err := repo.FindActiveByCredential(ctx, organizationID, userID, digest, trustedDeviceUAHash(userAgent), now)
	if err != nil || device == nil || !device.Active(now) {
		return false
	}
	if err := repo.TouchLastSeen(ctx, device.ID, now); err != nil {
		slog.WarnContext(ctx, "failed to touch trusted device last_seen; falling back to MFA", "user_id", userID, "error", err)
		return false
	}
	return true
}

func toTrustedDeviceView(device *trusteddevicesdomain.TrustedDevice, now time.Time) TrustedDeviceView {
	return TrustedDeviceView{
		ID:     device.ID,
		UserID: device.UserID,
		// 注册响应与两个列表接口共用此视图：只输出哈希前 8 位掩码，服务端任何
		// 响应体都不回显完整凭据（凭据本就由客户端生成）。
		DeviceCredentialID: trustedDeviceCredentialMask(device.DeviceCredentialID),
		Status:             trustedDeviceStatus(now, device.RevokedAt, device.ExpiresAt),
		CreatedAt:          device.CreatedAt,
		LastSeenAt:         device.LastSeenAt,
		ExpiresAt:          device.ExpiresAt,
		RevokedAt:          device.RevokedAt,
	}
}

func trustedDeviceStatus(now time.Time, revokedAt *time.Time, expiresAt time.Time) string {
	if revokedAt != nil {
		return "revoked"
	}
	if !expiresAt.After(now) {
		return "expired"
	}
	return "active"
}

// trustedDeviceCredentialDigest is the single conversion point from a client
// device credential to its stored form: validate the credential floor, then
// SHA-256 over the trimmed credential, lowercase hex. Registration,
// revocation-by-credential, login verification, views and audit payloads all
// call this function — no caller hashes on its own.
//
// Accepted input is 64 hex characters (256 bits) or base64url with at least 32
// characters (6 bits/char => >=192 bits). The 32-character floor is deliberately
// above the 128-bit client contract so partial-alphabet generators keep margin.
// Length/charset is a floor, not proof of entropy: the credential is generated
// client-side and the server cannot verify its randomness. OPEN-API-49 records
// the remaining client-contract gap.
func trustedDeviceCredentialDigest(credential string) (string, error) {
	if err := trusteddevicesdomain.ValidateCredentialInput(credential); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(credential)))
	return hex.EncodeToString(sum[:]), nil
}

// trustedDeviceCredentialMask renders the display/audit form of a credential
// digest: first 8 hex characters (32 bits) plus an ellipsis. The prefix is
// only for locating a device within one user's list; it is never used as a
// global identifier and cannot be reversed into the credential. Collision
// probability is birthday-bounded (~n²/2³³): ~1.2e-8 for 10 devices of one user.
// Residual risk: length/charset is only a floor. If a client uses a weak or
// biased generator and the actual entropy is below 128 bits, the prefix narrows
// an offline guess by 2³² (OPEN-API-49).
func trustedDeviceCredentialMask(digest string) string {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return ""
	}
	if len(digest) <= 8 {
		return digest + "…"
	}
	return digest[:8] + "…"
}

func trustedDeviceUAHash(userAgent string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(userAgent)))
	return hex.EncodeToString(sum[:])
}

func trustedDeviceChallengeHash(nonce string) string {
	sum := sha256.Sum256([]byte(nonce))
	return hex.EncodeToString(sum[:])
}

func trustedDeviceSignatureMessage(challengeID, userID, nonce string) string {
	return trustedDeviceSignaturePrefix + challengeID + ":" + userID + ":" + nonce
}

// verifyTrustedDeviceSignature accepts PKIX DER for ECDSA P-256 (SHA-256),
// RSA (PKCS#1 v1.5 SHA-256) and Ed25519. Other key types are unsupported and
// map to 422; a well-formed key with a bad signature maps to 401.
func verifyTrustedDeviceSignature(publicKeyDER []byte, message string, signature []byte) error {
	publicKey, err := x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		return apperrors.NewUnprocessableEntity("invalid device public key")
	}

	switch key := publicKey.(type) {
	case *ecdsa.PublicKey:
		if key.Curve != elliptic.P256() {
			return apperrors.NewUnprocessableEntity("unsupported ECDSA curve")
		}
		digest := sha256.Sum256([]byte(message))
		if !ecdsa.VerifyASN1(key, digest[:], signature) {
			return apperrors.NewUnauthorized("device signature verification failed")
		}
	case *rsa.PublicKey:
		digest := sha256.Sum256([]byte(message))
		if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
			return apperrors.NewUnauthorized("device signature verification failed")
		}
	case ed25519.PublicKey:
		if !ed25519.Verify(key, []byte(message), signature) {
			return apperrors.NewUnauthorized("device signature verification failed")
		}
	default:
		return apperrors.NewUnprocessableEntity("unsupported device public key type")
	}
	return nil
}

func trustedDeviceDecodeBase64(value string) ([]byte, error) {
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := encoding.DecodeString(value); err == nil {
			return decoded, nil
		}
	}
	return nil, apperrors.NewInvalidArgument("invalid base64 encoding")
}
