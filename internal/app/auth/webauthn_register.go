package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	web3identitydomain "github.com/guliangiam/guliangiam/internal/domain/web3identity"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformweb3 "github.com/guliangiam/guliangiam/internal/platform/web3"
	platformwebauthn "github.com/guliangiam/guliangiam/internal/platform/webauthn"
)

func (s *Service) WebAuthnRegistrationBegin(ctx context.Context, command WebAuthnRegistrationBeginCommand) (*WebAuthnRegistrationBeginResult, error) {
	if strings.TrimSpace(command.Username) == "" {
		return nil, apperrors.NewInvalidArgument("username is required")
	}

	organization, err := s.UOW.Organizations().GetBySlug(ctx, command.OrganizationSlug)
	if err != nil {
		return nil, err
	}
	if !passkeyEnabled(organization) {
		return nil, apperrors.NewForbidden("passkey is disabled for this organization")
	}

	user, err := s.registrationUserForCommand(ctx, organization, command.SessionUserID, command.SessionOrganizationID, command.Username)
	if err != nil {
		return nil, err
	}

	// 同一认证器重复注册在客户端就被拦住：把该用户已有活跃凭据全部下发为
	// excludeCredentials。服务端仍有唯一约束与落库前预检兜底。
	existing, err := s.UOW.WebAuthnCredentials().ListByUserID(ctx, user.ID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "list webauthn credentials")
	}
	excludeCredentials := platformwebauthn.EncodeCredentialIDs(webauthndomain.ActiveCredentialIDs(existing))

	challenge, err := generateWebAuthnChallenge()
	if err != nil {
		return nil, apperrors.NewInternal("generate webauthn challenge")
	}

	// S2: bind the challenge to an unpredictable, server-side row. Finish MUST
	// present this id; knowing only the user ID is insufficient. 表化后 begin/finish
	// 落在不同副本（或进程重启）也能配对——进程内存 map 做不到（《开发设计》31.4）。
	rpID, err := s.webAuthnRelyingPartyID()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	challengeID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate webauthn challenge id")
	}
	if err := s.UOW.WebAuthnCredentials().CreateChallenge(ctx, &webauthndomain.Challenge{
		ID:             challengeID,
		ChallengeHash:  webauthndomain.HashChallenge(challenge),
		CeremonyType:   webauthndomain.CeremonyTypeRegistration,
		UserID:         user.ID,
		OrganizationID: organization.ID,
		RelyingPartyID: rpID,
		ExpiresAt:      now.Add(webAuthnChallengeTimeout),
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}); err != nil {
		return nil, apperrors.NewInternal("store webauthn challenge")
	}

	return &WebAuthnRegistrationBeginResult{
		ChallengeID:        challengeID,
		Challenge:          challenge,
		RelyingPartyID:     rpID,
		RelyingPartyName:   "GuliangIAM",
		UserID:             user.ID,
		UserDisplayName:    user.Profile.DisplayName,
		Timeout:            int64(webAuthnChallengeTimeout / time.Millisecond),
		ExcludeCredentials: excludeCredentials,
		UserVerification:   platformwebauthn.UserVerificationValue(s.webAuthnUVRequired(organization)),
	}, nil
}

// WebAuthnRegistrationFinish completes a WebAuthn credential registration.
func (s *Service) WebAuthnRegistrationFinish(ctx context.Context, command WebAuthnRegistrationFinishCommand) (*WebAuthnRegistrationFinishResult, string, error) {
	if strings.TrimSpace(command.ChallengeID) == "" {
		return nil, "", apperrors.NewInvalidArgument("challengeId is required — call registration:begin first")
	}
	if strings.TrimSpace(command.Username) == "" {
		return nil, "", apperrors.NewInvalidArgument("username is required")
	}
	if strings.TrimSpace(command.CredentialID) == "" {
		return nil, "", apperrors.NewInvalidArgument("credentialId is required")
	}
	if strings.TrimSpace(command.PublicKey) == "" {
		return nil, "", apperrors.NewInvalidArgument("publicKey is required")
	}

	// origin 校验依赖服务端配置的来源集合；为空说明部署缺配置，先给出可操作的错误，
	// 而不是让每一次 ceremony 都倒在「origin 不在允许清单」上（也避免白白消费挑战）。
	if err := s.requireWebAuthnOriginConfig(); err != nil {
		return nil, "", err
	}

	// S2: 原子消费挑战行——未消费且未过期的行才会被本调用抢到（并发 finish 只有一个能赢），
	// 同时把 begin→finish 绑到行里记录的用户。
	sessionData, consumeErr := s.UOW.WebAuthnCredentials().ConsumeChallenge(ctx, command.ChallengeID, s.Clock.Now())
	if consumeErr != nil {
		// 缺失 / 已消费 / 已过期在仓储层同为 NotFound，这里统一 410 且消息一致，
		// 调用方无法据此枚举挑战是否曾经存在。
		return nil, "", apperrors.NewGone("registration challenge is missing, expired or already used")
	}
	// 两个 ceremony 共用一张表：拿认证挑战来完成注册就是跨 ceremony 重放，必须拒绝。
	if sessionData.CeremonyType != webauthndomain.CeremonyTypeRegistration {
		return nil, "", apperrors.NewForbidden("challenge was issued for another ceremony")
	}
	// 明文挑战不落库，只能靠客户端回传的明文重算哈希比对（库被读走也伪造不了断言）。
	if !sessionData.MatchesChallenge(command.Challenge) {
		return nil, "", apperrors.NewUnprocessableEntity("challenge does not match the one issued by begin")
	}

	organization, err := s.UOW.Organizations().GetBySlug(ctx, command.OrganizationSlug)
	if err != nil {
		return nil, "", err
	}
	if !passkeyEnabled(organization) {
		return nil, "", apperrors.NewForbidden("passkey is disabled for this organization")
	}

	user, err := s.registrationUserForCommand(ctx, organization, command.SessionUserID, command.SessionOrganizationID, command.Username)
	if err != nil {
		return nil, "", err
	}

	// S2: bind the submitted username to the session-bound user ID.
	if sessionData.UserID != user.ID {
		return nil, "", apperrors.NewForbidden("challenge session user does not match submitted username")
	}

	// 服务端唯一可信来源：由 attestationObject 派生 credentialId / COSE 公钥 /
	// authenticatorData / attestation 格式。请求体里的同名字段只做一致性校验，
	// 不一致即拒绝——否则合法 attestation 可以搭配任意客户端公钥入库。
	material, err := platformwebauthn.DeriveRegistrationMaterial(command.AttestationObject)
	if err != nil {
		return nil, "", apperrors.NewUnprocessableEntity("WebAuthn registration verification failed: attestationObject is malformed")
	}
	if command.CredentialID != material.CredentialIDB64 {
		return nil, "", apperrors.NewUnprocessableEntity("credentialId does not match the attestationObject")
	}
	submittedPublicKey, err := base64.RawURLEncoding.DecodeString(command.PublicKey)
	if err != nil {
		return nil, "", apperrors.NewInvalidArgument("invalid publicKey encoding")
	}
	if !bytes.Equal(submittedPublicKey, material.PublicKey) {
		return nil, "", apperrors.NewUnprocessableEntity("publicKey does not match the attestationObject")
	}
	if strings.TrimSpace(command.AuthenticatorData) != "" {
		submittedAuthData, err := base64.RawURLEncoding.DecodeString(command.AuthenticatorData)
		if err != nil {
			return nil, "", apperrors.NewInvalidArgument("invalid authenticatorData encoding")
		}
		if !bytes.Equal(submittedAuthData, material.AuthenticatorData) {
			return nil, "", apperrors.NewUnprocessableEntity("authenticatorData does not match the attestationObject")
		}
	}
	if strings.TrimSpace(command.AttestationType) != "" && command.AttestationType != material.AttestationType {
		return nil, "", apperrors.NewUnprocessableEntity("attestationType does not match the attestationObject")
	}

	// S1: verify the full registration ceremony before storing anything.
	aaguid, registrationSignCount, err := platformwebauthn.VerifyRegistration(
		command.Challenge,
		sessionData.RelyingPartyID,
		material.CredentialIDB64,
		material.PublicKey,
		command.ClientDataJSON,
		material.AuthenticatorDataB64,
		command.AttestationObject,
		s.webAuthnAllowedOrigins(),
		s.webAuthnUVRequired(organization),
	)
	if err != nil {
		return nil, "", apperrors.NewUnprocessableEntityf("WebAuthn registration verification failed: %s", err.Error())
	}

	credentialUUID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, "", apperrors.NewInternal("generate credential id")
	}

	now := s.Clock.Now().UTC()
	credential := &webauthndomain.Credential{
		ID:              credentialUUID,
		UserID:          user.ID,
		CredentialID:    material.CredentialID,
		PublicKey:       material.PublicKey,
		AttestationType: material.AttestationType,
		Transport:       command.Transports,
		SignCount:       registrationSignCount,
		Name:            "WebAuthn Passkey",
		Status:          webauthndomain.CredentialStatusActive,
		AAGUID:          aaguid,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// S9-sensitive: credential enrollment elevates assurance. Wrap credential
	// persistence and session rotation in one transaction — no gap where a
	// pre-planted cookie survives the account hardening.
	var newSessionID string
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// 落库前预检：唯一约束负责兜并发，这里负责给出可映射为
		// WebAuthn InvalidStateError 的语义（409），而不是等 DB 报错变 500。
		existing, err := tx.WebAuthnCredentials().GetByCredentialID(ctx, material.CredentialID)
		if err != nil {
			var appErr *apperrors.Error
			if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
				return apperrors.WrapInternal(err, "check duplicate webauthn credential")
			}
		} else if existing != nil {
			return apperrors.NewAlreadyExists("credential is already registered")
		}

		if err := tx.WebAuthnCredentials().Create(ctx, credential); err != nil {
			return err
		}

		// Layer 2: passkey → EVM address deterministic mapping (Liquid Auth).
		// Same transaction as credential creation — address must not exist without
		// a valid credential row, and we want credential + mapping to appear or disappear together.
		addr, seed, derr := platformweb3.DeriveAddressFromCOSEPubkey(credential.PublicKey)
		if derr != nil {
			// Non-fatal — address derivation failure should not block credential
			// registration (a future retry path can create the mapping later).
			// Logged via the embedAuditOutboxPlain below for observability.
			_ = derr
		} else {
			mappingID, midErr := s.UUIDGenerator.NewString()
			if midErr != nil {
				return apperrors.NewInternal("generate web3 identity mapping id")
			}
			if cerr := tx.Web3Identities().Create(ctx, &web3identitydomain.Mapping{
				ID:               mappingID,
				UserID:           user.ID,
				CredentialID:     credential.ID,
				WalletAddress:    addr,
				DerivationMethod: "keccak256-passkey",
				DerivationSeed:   seed,
				ChainIDs:         []int64{1, 137, 56, 10, 42161, 42170},
				IsPrimary:        true,
				VerifiedAt:       now,
				CreatedAt:        now,
				UpdatedAt:        now,
				Revision:         1,
			}); cerr != nil {
				// UNIQUE violation on wallet_address → another credential already maps here.
				// UNIQUE violation on credential_id → duplicate registration attempt.
				// Either way: log but don't fail registration — the passkey itself is what
				// matters. Address mapping can be added later via a migration job.
				_ = cerr
			} else {
				// Audit trail for the mapping creation.
				if err := s.embedAuditOutboxPlain(
					tx,
					ctx,
					"",
					"",
					"",
					organization.ID,
					user.ID,
					"web3.identity.created",
					"web3_identity_mapping",
					mappingID,
					"web3_identity_mapping",
					"web3.identity.created",
					map[string]any{
						"wallet_address":    addr,
						"derivation_method": "keccak256-passkey",
						"credential_id":     credential.ID,
					},
				); err != nil {
					return err
				}
			}
		}
		// aaguid 已由 VerifyRegistration 解析并填充到 credential.AAGUID 字段 (hex string)
		if strings.TrimSpace(command.PreviousSessionID) != "" {
			oldSess, getErr := tx.Sessions().GetByID(ctx, command.PreviousSessionID)
			if getErr == nil {
				_ = tx.Sessions().Revoke(ctx, command.PreviousSessionID, now)
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
					return err
				}
				sessID, idErr := s.UUIDGenerator.NewString()
				if idErr != nil {
					return apperrors.NewInternal("generate rotated session id")
				}
				sid, sidErr := newSessionSid()
				if sidErr != nil {
					return sidErr
				}
				if cErr := tx.Sessions().Create(ctx, &sessionsdomain.Session{
					ID:               sessID,
					Sid:              sid,
					UserID:           oldSess.UserID,
					Username:         oldSess.Username,
					OrganizationID:   oldSess.OrganizationID,
					OrganizationSlug: oldSess.OrganizationSlug,
					ApplicationID:    oldSess.ApplicationID,
					ApplicationSlug:  oldSess.ApplicationSlug,
					SessionState:     statusActive,
					DeviceInfo:       oldSess.DeviceInfo,
					IPAddress:        oldSess.IPAddress,
					UserAgent:        oldSess.UserAgent,
					ExpiresAt:        now.Add(30 * 24 * time.Hour),
					CreatedAt:        now,
					LastSeenAt:       now,
					UpdatedAt:        now,
					Revision:         1,
				}); cErr != nil {
					return cErr
				}
				// 轮换出来的会话与其它签发路径一样留审计行，否则
				// session.revoked 之后会出现一段无 session.created 的空档。
				if err := s.embedAuditOutboxPlain(
					tx,
					ctx,
					"",
					"",
					"",
					oldSess.OrganizationID,
					oldSess.UserID,
					"session.created",
					"session",
					sessID,
					"session",
					"session.created",
					map[string]any{},
				); err != nil {
					return err
				}
				newSessionID = sessID
			}
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	return &WebAuthnRegistrationFinishResult{
		Status:       "registered",
		CredentialID: material.CredentialIDB64,
	}, newSessionID, nil
}

// registrationUserForCommand resolves the user a passkey registration may act
// for. With a trusted session it MUST be the session user (self-service):
//   - body username may be omitted, but if present it must equal the session
//     username; a mismatch is 403, not a victim-account write;
//   - the session organization must match the resolved organization.
//
// Without a session (login flow) the body identity is used and the existing
// challenge binding still applies.
func (s *Service) registrationUserForCommand(ctx context.Context, organization *orgdomain.Organization, sessionUserID, sessionOrganizationID, username string) (*userdomain.User, error) {
	sessionUserID = strings.TrimSpace(sessionUserID)
	if sessionUserID == "" {
		return s.UOW.Users().GetByUsername(ctx, organization.ID, strings.TrimSpace(username))
	}
	if sessionOrganizationID := strings.TrimSpace(sessionOrganizationID); sessionOrganizationID != "" && sessionOrganizationID != organization.ID {
		return nil, apperrors.NewForbidden("authenticated session belongs to another organization")
	}
	user, err := s.UOW.Users().GetByIDForOrganization(ctx, sessionUserID, organization.ID)
	if err != nil {
		return nil, apperrors.NewForbidden("authenticated session user does not belong to this organization")
	}
	if requested := strings.TrimSpace(username); requested != "" && requested != user.Username {
		return nil, apperrors.NewForbidden("registration username must match the authenticated user")
	}
	return user, nil
}
