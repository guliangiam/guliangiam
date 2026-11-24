package auth

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

func (s *Service) Login(ctx context.Context, command LoginCommand) (*LoginResult, error) {
	if err := validateLoginCommand(command); err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "Login command",
		"organization_slug", command.OrganizationSlug,
		"username", command.Username,
		"application_id", command.ApplicationID,
		"challenge_id", command.ChallengeID,
	)

	var result *LoginResult
	// Remember-me token delivered back to the client (rotated during an
	// exchange, or freshly issued when EnableMfaRemember completes MFA).
	var (
		loginRememberMeToken     string
		loginRememberMeExpiresAt *time.Time
	)
	// Failure counters cannot be written inside the login transaction: a
	// credential error makes WithinTransaction roll back, discarding the
	// increment. Remember the verified identity and persist the failure in a
	// separate transaction after the main one has rolled back.
	var (
		credentialFailure           bool
		failedLoginUserID           string
		failedLoginOrgID            string
		failedLoginAt               time.Time
		failedLoginReason           string
		failedLoginLockoutRemaining time.Duration

		// MFA rejections roll the challenge counter back with the login
		// transaction, so they are committed separately after the rollback.
		failedMfaChallenge *mfaChallengeFailure
		staleChallengeID   string
	)
	failedLoginUsername := strings.TrimSpace(command.Username)
	// 关键：Login 是匿名场景，还没知道当前用户属于哪个 org，
	// 但 organizations / applications / users 全 FORCE RLS ——
	// ctx 没 TenantScope → setTenantContext 注入空 orgID + member role
	// → RLS tenant_isolation current_org()=NULL → DENY ALL。
	// TenantFilterSystem 框架层系统任务语义：根据登录请求解析 org/app 是
	// 认证流程固有职责，不属于业务跨租户特权滥用。映射到 super_admin role
	// → is_super_admin()=true → 三张表 RLS tenant_isolation 放行。
	systemCtx := tenantctx.System(ctx)
	err := s.UOW.WithinTransaction(systemCtx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		now := s.Clock.Now()
		failedLoginAt = now

		// --- Security Enhancement: Account Lockout Check ---
		// Before resolving the login context (which verifies the password),
		// check if the user is currently locked out. We need to look up the user
		// first to get the userID for the lockout check.
		// If a challengeID is provided, the user has already been authenticated
		// and is just completing MFA, so skip the lockout check for the initial
		// user lookup.
		challengeID := strings.TrimSpace(command.ChallengeID)
		if challengeID == "" {
			// Look up the user to check account lockout before password verification.
			preOrg, err := tx.Organizations().GetBySlug(ctx, command.OrganizationSlug)
			if err != nil {
				return err
			}
			// Human-check gate: when the organization enables a captcha provider,
			// a valid response is required before any credential check runs
			// (anti-bot, mirrors the Signup gate). The provider is resolved
			// server-side, so a caller cannot downgrade or skip it.
			if err := s.EnforceCaptcha(ctx, preOrg.ID, command.CaptchaToken, command.CaptchaID, command.IPAddress); err != nil {
				return err
			}
			if err := rejectServiceAccountPasswordLogin(ctx, tx, preOrg.ID, command.Username); err != nil {
				return err
			}
			preUser, err := tx.Users().GetByLoginIdentifier(ctx, preOrg.ID, command.Username)
			if err != nil {
				// User not found - return a generic error to avoid user enumeration.
				// The audit trail still records the attempt, scoped to the
				// organization slug the caller supplied.
				failedLoginOrgID = preOrg.ID
				failedLoginReason = loginFailureReasonInvalidCredentials
				return genericLoginFailureError()
			}
			failedLoginUserID = preUser.ID
			failedLoginOrgID = preOrg.ID

			// Check if the account is locked due to too many failed login attempts.
			// A locked account is answered exactly like a wrong password; the
			// remaining lock window is only logged and audited server-side.
			locked, remaining, lockErr := accountLockoutStatus(ctx, tx, preUser.ID, now)
			if lockErr != nil {
				return lockErr
			}
			if locked {
				failedLoginReason = loginFailureReasonAccountLocked
				failedLoginLockoutRemaining = remaining
				return genericLoginFailureError()
			}

			// Pre-login checks before password verification.
			if !isUserAllowedToLogin(preUser.Status, preUser.IsForbidden) {
				return apperrors.NewUnauthorized("user cannot sign in")
			}
			if preOrg.DisableSignin {
				return apperrors.NewUnauthorized("organization has sign-in disabled")
			}
			preApplicationReference := strings.TrimSpace(command.ApplicationID)
			if preApplicationReference == "" && strings.TrimSpace(command.ResponseType) == "" {
				preApplicationReference = preOrg.Slug
			}
			if preApplicationReference != "" {
				preApp, appErr := resolveApplicationReference(ctx, tx.Applications(), preOrg, preApplicationReference)
				if appErr != nil {
					return appErr
				}
				if preApp.DisableSignin {
					return apperrors.NewUnauthorized("application has sign-in disabled")
				}
				if err := checkApplicationTagAccess(preApp, preUser); err != nil {
					return err
				}
			}
		}

		organization, application, user, challenge, err := s.resolveLoginContext(ctx, tx, command, now)
		if err != nil {
			credentialFailure = isCredentialError(err)
			if credentialFailure && failedLoginUserID != "" {
				failedLoginReason = loginFailureReasonInvalidCredentials
			}
			// The expired-challenge cleanup in resolveLoginContext would be
			// rolled back with the rejection; remember it for the post-rollback
			// transaction, which re-checks expiry before deleting.
			if strings.TrimSpace(command.ChallengeID) != "" && challenge == nil {
				staleChallengeID = strings.TrimSpace(command.ChallengeID)
			}
			return err
		}

		// --- Security Enhancement: Reset Login Failures on Successful Authentication ---
		if resetErr := resetLoginFailuresWithTx(ctx, tx, user.ID); resetErr != nil {
			slog.WarnContext(ctx, "failed to reset login failures", "user_id", user.ID, "error", resetErr)
		}

		// --- Entitlement Check ---
		// A `paid-user` may only receive a session while a subscription still
		// grants entitlement. The check sits here, after the credentials are
		// verified and before any session is issued, so every sign-in path is
		// covered by one gate.
		if subErr := checkPaidUserSubscription(ctx, tx, user, now); subErr != nil {
			return subErr
		}

		// --- Password Algorithm Auto-Upgrade ---
		// If the user's password was verified with a legacy algorithm (not argon2id),
		// automatically re-hash it with argon2id and persist the upgrade. The
		// upgrade mutates the credential value, so the old algorithm must be
		// captured first or the logs would report argon2id as both sides.
		oldPasswordAlgo := strings.TrimSpace(user.Credentials.PasswordAlgo)
		if oldPasswordAlgo != "" &&
			!strings.EqualFold(oldPasswordAlgo, hashArgon2ID) &&
			strings.TrimSpace(command.Password) != "" {
			if upgraded, upgradeErr := upgradePasswordAlgorithm(command.Password, &user.Credentials); upgradeErr != nil {
				slog.WarnContext(ctx, "failed to upgrade password algorithm",
					"user_id", user.ID,
					"old_algorithm", oldPasswordAlgo,
					"error", upgradeErr,
				)
			} else if upgraded {
				if persistErr := tx.Users().UpdateCredentials(ctx, &user.Credentials); persistErr != nil {
					slog.WarnContext(ctx, "failed to persist password algorithm upgrade",
						"user_id", user.ID,
						"old_algorithm", oldPasswordAlgo,
						"error", persistErr,
					)
				} else {
					slog.InfoContext(ctx, "password algorithm upgraded",
						"user_id", user.ID,
						"old_algorithm", oldPasswordAlgo,
						"new_algorithm", hashArgon2ID,
					)
				}
			}
		}

		// --- Password Expiration Check ---
		// EvaluatePasswordState is the single threshold source shared with the
		// request-time password gate. Login must not persist a max-age result
		// into need_update_password: doing so would make every later request hit
		// the flag branch and password_expired would be unreachable. The state is
		// applied to the login response only; the admin-forced flag is maintained
		// by the administrator write paths.
		passwordState := EvaluatePasswordState(user.Credentials, resolvePasswordPolicy(organization), now)

		if requiredSetupTypes := requiredMfaSetupTypes(organization, user); len(requiredSetupTypes) > 0 {
			if challenge == nil {
				challenge, err = s.createLoginChallenge(ctx, tx, organization.ID, user.ID, application.ID, now)
				if err != nil {
					return err
				}
			}
			result = &LoginResult{Response: toMfaSetupRequiredLoginResponse(requiredSetupTypes, challenge.ID, challenge.ExpiresAt)}
			return nil
		}

		// OPEN-API-18: authLevel 追踪本次登录的认证等级。
		// 声明在 MFA 检查之前，在 MFA 验证通过后被修改为 "mfa"，
		// session 创建时据此写入 AuthenticationLevel。
		authLevel := loginMethodPassword

		// F1: 信任设备命中时跳过本次 MFA 校验（第二因子已在 30 天信任窗口内满足）。
		// 任何错误都 fall through 到原 MFA 流程，不向调用方暴露。
		trustedDeviceSatisfied := false
		if credentialID := strings.TrimSpace(command.TrustedDeviceCredentialID); credentialID != "" {
			if s.trustedDeviceSatisfiesMfa(ctx, tx, organization.ID, user.ID, credentialID, command.UserAgent, now) {
				trustedDeviceSatisfied = true
				authLevel = loginMethodMFA
			}
		}

		// F3: a valid remember-me token substitutes the MFA passcode and rotates
		// in place. Any error is silent: the normal MFA flow below still runs.
		rememberMeSatisfied := false
		if token := strings.TrimSpace(command.RememberMeToken); token != "" {
			if methods := enabledVerifiedMfaMethods(user); len(methods) > 0 && requiresMfaVerification(now, user) {
				rotated, expiresAt, verifyErr := s.VerifyRememberMeForLogin(ctx, tx, organization.ID, user.ID, token, now)
				if verifyErr != nil {
					slog.DebugContext(ctx, "remember-me token rejected; falling back to MFA", "user_id", user.ID)
				} else {
					rememberMeSatisfied = true
					authLevel = loginMethodMFA
					loginRememberMeToken = rotated
					loginRememberMeExpiresAt = &expiresAt
				}
			}
		}

		if methods := enabledVerifiedMfaMethods(user); !trustedDeviceSatisfied && !rememberMeSatisfied && len(methods) > 0 && requiresMfaVerification(now, user) {
			if !hasMfaVerificationAttempt(command) {
				if challenge == nil {
					challenge, err = s.createLoginChallenge(ctx, tx, organization.ID, user.ID, application.ID, now)
					if err != nil {
						return err
					}
				}
				availableMethods := s.mfaMethodsWithPasskeySecondFactor(ctx, tx, organization, user, methods)
				result = &LoginResult{Response: toMfaRequiredLoginResponse(user, user.PreferredMfaType, availableMethods, challenge.ID, challenge.State, challenge.ExpiresAt)}
				return nil
			}

			verification, err := verifyLoginMfa(ctx, tx, command, organization, user, challenge, methods, now, s.PushNotifier)
			if err != nil {
				failedMfaChallenge = verification.Failure
				return err
			}
			if !verification.Completed {
				result = &LoginResult{Response: verification.Response}
				return nil
			}

			// MFA 实际完成验证 — 提升认证等级
			authLevel = loginMethodMFA

			// ADR-029 mfa extension point: challenge verified, gate before
			// the authenticated session proceeds. Fail-closed.
			if err := s.enforceActionsMFA(ctx, organization.ID, user.ID, user.Username, application.ID, command.MfaType); err != nil {
				return err
			}

			if challenge != nil {
				if err := tx.MfaChallenges().Delete(ctx, challenge.ID); err != nil {
					return err
				}

				if err := s.embedAuditOutboxPlain(
					tx,
					ctx,
					"",
					"",
					"",
					organization.ID,
					"",
					"mfa.challenge.deleted",
					loginMethodMFA,
					challenge.ID,
					loginMethodMFA,
					"mfa.challenge.deleted",
					map[string]any{},
				); err != nil {
					return err
				}
			}

			// The aggregate update uses the optimistic-lock convention (the
			// repository expects the post-increment revision and guards on
			// Revision-1), so the MFA success path must bump it here; without
			// this every MFA login ended in a 409 revision conflict.
			user.UpdatedAt = now
			user.Revision++
			if err := tx.Users().Update(ctx, user); err != nil {
				return err
			}

			if err := tx.Users().ReplaceMfaMethods(ctx, user.ID, user.MfaMethods); err != nil {
				return err
			}

			if err := s.embedAuditOutboxPlain(
				tx,
				ctx,
				"",
				"",
				"",
				user.OrganizationID,
				user.ID,
				"user.updated",
				"user",
				user.ID,
				"user",
				"user.updated",
				map[string]any{},
			); err != nil {
				return err
			}

			// F3: EnableMfaRemember mints a standalone remember-me family once MFA
			// actually completes. Failures are warn-only so the login still succeeds.
			if command.EnableMfaRemember {
				if repo, repoErr := rememberMeRepo(tx); repoErr != nil {
					slog.WarnContext(ctx, "remember-me repository unavailable; skipping issuance", "user_id", user.ID, "error", repoErr)
				} else if issued, issueErr := s.issueRememberMeInTx(ctx, tx, repo, organization.ID, user.ID, application.ID, now); issueErr != nil {
					slog.WarnContext(ctx, "failed to issue remember-me token after MFA", "user_id", user.ID, "error", issueErr)
				} else {
					loginRememberMeToken = issued.Token
					loginRememberMeExpiresAt = &issued.ExpiresAt
				}
			}
		} else if challenge != nil {
			if err := tx.MfaChallenges().Delete(ctx, challenge.ID); err != nil {
				return err
			}

			if err := s.embedAuditOutboxPlain(
				tx,
				ctx,
				"",
				"",
				"",
				organization.ID,
				"",
				"mfa.challenge.deleted",
				loginMethodMFA,
				challenge.ID,
				loginMethodMFA,
				"mfa.challenge.deleted",
				map[string]any{},
			); err != nil {
				return err
			}
		}

		// OPEN-API-18 Phase 3: require_mfa 两阶段拆分 pre-check
		// session 创建前检查并发策略是否触发 require_mfa：
		//  超限 + require_mfa + authLevel < "mfa" → 返回 MFA challenge（同组织级 MFA 复用）
		//  authLevel 已经是 "mfa"/"fido2"（本次登录里已经过了 MFA / FaceID / WebAuthn）→ 直接跳过
		//  并发上限未超限 / 策略不是 require_mfa → 跳过
		//  用户没有 MFA 方法 + require_mfa → fail-closed（无法满足策略）
		if triggered, err := preCheckRequireMFA(ctx, tx, user, organization.ID, authLevel); err != nil {
			return err
		} else if triggered {
			if challenge == nil {
				challenge, err = s.createLoginChallenge(ctx, tx, organization.ID, user.ID, application.ID, now)
				if err != nil {
					return err
				}
			}
			methods := s.mfaMethodsWithPasskeySecondFactor(ctx, tx, organization, user, enabledVerifiedMfaMethods(user))
			result = &LoginResult{Response: toMfaRequiredLoginResponse(user, user.PreferredMfaType, methods, challenge.ID, challenge.State, challenge.ExpiresAt)}
			return nil
		}

		// ── Risk engine evaluation ──────────────────────────────────────
		// Pre-session risk gate (AC-RISK-001): deny → reject outright,
		// challenge → force MFA (password login can step up),
		// allow → proceed. nil evaluator skips the check (backward-compat).
		var userIDPtr *string
		uid := user.ID
		userIDPtr = &uid
		evalErr := s.evaluateLoginRisk(ctx, tx, channelFromCommand(command), organization.ID, userIDPtr,
			command.IPAddress, command.UserAgent, command.TrustedDeviceCredentialID, map[string]any{})
		switch e := evalErr.(type) {
		case nil: // allow
		case *RiskDeniedError:
			return apperrors.NewUnauthorized("login blocked by risk assessment")
		case *RiskChallengeError:
			if authLevel != loginMethodMFA {
				if challenge == nil {
					challenge, err = s.createLoginChallenge(ctx, tx, organization.ID, user.ID, application.ID, now)
					if err != nil {
						return err
					}
				}
				methods := enabledVerifiedMfaMethods(user)
				if len(methods) == 0 {
					slog.WarnContext(ctx, "risk challenge requested but user has no MFA methods",
						"organization_id", organization.ID, "user_id", user.ID, "decision_id", e.DecisionID)
					return apperrors.NewUnauthorized("additional verification required but no method available")
				}
				availableMethods := s.mfaMethodsWithPasskeySecondFactor(ctx, tx, organization, user, methods)
				result = &LoginResult{Response: toMfaRequiredLoginResponse(user, user.PreferredMfaType, availableMethods, challenge.ID, challenge.State, challenge.ExpiresAt)}
				return nil
			}
		default: // engine unavailable → fail-closed
			return evalErr
		}

		// ── ADR-029 pre_login extension point ───────────────────────────
		// Credentials verified, risk passed; policy actions get the final
		// say before a session exists. Fail-closed; deny reasons stay
		// server-side (audit/execution logs), never echoed to the caller.
		if err := s.enforceActionsPreLogin(ctx, channelFromCommand(command), organization.ID, user.ID, user.Username,
			application.ID, command.IPAddress, command.UserAgent); err != nil {
			return err
		}

		sessionID, err := s.UUIDGenerator.NewString()
		if err != nil {
			return apperrors.NewInternal("generate session id")
		}

		// Determine session TTL based on EnableSigninSession flag.
		effectiveTTL := sessionTTL
		if !application.EnableSigninSession {
			effectiveTTL = shortSessionTTL
		}

		// authLevel 已在 MFA 检查之前声明（"password" / "mfa"），
		// 依据 MFA 是否实际完成验证来决定。
		sid, err := newSessionSid()
		if err != nil {
			return err
		}
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
			DeviceInfo:          map[string]any{},
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
			return err
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
			return err
		}
		// S9: revoke the session carried by the request (prevents session fixation
		// when an attacker has pre-planted a cookie on the victim's browser).
		// Not best-effort: if the revoke fails, the whole login transaction rolls
		// back, because keeping both sessions alive would silently defeat the
		// rotation. Missing/already-revoked sessions stay tolerated (see
		// revokePreviousSessionForRotation).
		if strings.TrimSpace(command.PreviousSessionID) != "" {
			if err := revokePreviousSessionForRotation(ctx, tx.Sessions(), command.PreviousSessionID, now); err != nil {
				return err
			}
			if err := s.embedAuditOutboxPlain(
				tx,
				ctx,
				"",
				"",
				"",
				organization.ID,
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
		}

		// 登录方式无关的会话策略与 signin 统计（排他登录 / 并发上限 / LastSignin*）。
		if err := s.applyLoginSessionPolicies(ctx, tx, user, application, organization.ID, session, now); err != nil {
			return err
		}
		// 异常登录检测只记录告警，不阻断登录。
		s.warnAnomalousLogin(ctx, tx, user.ID, session.IPAddress)

		// HandleLoggedIn: generate the appropriate response based on response_type.
		loginResponse, err := s.handleLoggedIn(ctx, tx, command, organization, application, user, sid, now)
		if err != nil {
			return err
		}
		applyPasswordStateNotice(loginResponse, user.Credentials, resolvePasswordPolicy(organization), passwordState)

		result = &LoginResult{
			SessionID:           sessionID,
			Response:            *loginResponse,
			RememberMeToken:     loginRememberMeToken,
			RememberMeExpiresAt: loginRememberMeExpiresAt,
		}
		return nil
	})
	if err != nil {
		// The main login transaction has already rolled back. Persist the
		// failure counter in its own transaction so lockout increments survive
		// a credential rejection. Fail-open on availability: a recording error
		// is logged and never changes the response returned to the caller.
		if credentialFailure && failedLoginUserID != "" && failedLoginOrgID != "" {
			attempt, recordErr := s.persistLoginFailure(systemCtx, failedLoginUserID, failedLoginOrgID, failedLoginAt)
			if recordErr != nil {
				slog.WarnContext(systemCtx, "failed to record login failure",
					"user_id", failedLoginUserID, "error", recordErr)
			} else if attempt != nil && attempt.IsLocked(failedLoginAt) {
				// 锁定是安全高信号事件：避免每次失败都通知，只在本次失败触发
				// 锁定时写入个人通知中心（收件箱 + 按偏好扇出外部渠道）。
				// source_id 是 UUID 列：用确定性 UUID 标识本次锁定事件，
				// 同一锁定时间窗重复投递不会产生重复消息。
				lockoutSourceID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(failedLoginUserID+":"+attempt.LockedUntil.UTC().Format(time.RFC3339))).String()
				s.notifyUser(systemCtx, NotificationCommand{
					OrganizationID: failedLoginOrgID,
					UserID:         failedLoginUserID,
					Category:       "security",
					EventType:      "auth.login.failed",
					Title:          "账号已被临时锁定",
					Content:        "检测到连续多次登录失败，账号已被临时锁定至 " + attempt.LockedUntil.Local().Format("2006-01-02 15:04") + "。如非本人操作，请尽快重置密码并联系管理员。",
					Link:           "/notifications",
					SourceType:     "auth_lockout",
					SourceID:       lockoutSourceID,
				})
			}
		}
		// Same boundary for the MFA attempt counter: the challenge row is
		// reloaded and updated (or deleted at the limit) in its own transaction.
		if failedMfaChallenge != nil {
			if recordErr := s.persistMfaChallengeFailure(systemCtx, *failedMfaChallenge); recordErr != nil {
				slog.WarnContext(systemCtx, "failed to persist MFA challenge failure",
					"challenge_id", failedMfaChallenge.ChallengeID, "error", recordErr)
			}
		} else if staleChallengeID != "" {
			if delErr := s.deleteExpiredLoginChallenge(systemCtx, staleChallengeID); delErr != nil {
				slog.WarnContext(systemCtx, "failed to delete expired MFA challenge",
					"challenge_id", staleChallengeID, "error", delErr)
			}
		}
		// The audit row is written in its own transaction as well: it must not
		// ride the rolled-back login transaction, and an audit outage must not
		// take the lockout counter (or the response) down with it.
		if failedLoginReason != "" {
			auditInput := loginFailureAudit{
				OrganizationID:   failedLoginOrgID,
				UserID:           failedLoginUserID,
				Username:         failedLoginUsername,
				Reason:           failedLoginReason,
				IPAddress:        strings.TrimSpace(command.IPAddress),
				UserAgent:        strings.TrimSpace(command.UserAgent),
				LockoutRemaining: failedLoginLockoutRemaining,
			}
			if auditErr := s.persistLoginFailureAudit(systemCtx, auditInput); auditErr != nil {
				slog.WarnContext(systemCtx, "failed to write login failure audit",
					"reason", failedLoginReason, "user_id", failedLoginUserID, "error", auditErr)
			}
		}
		return nil, err
	}

	return result, nil
}

const (
	loginFailureReasonInvalidCredentials = "invalid_credentials"
	loginFailureReasonAccountLocked      = "account_locked"
)

// loginFailureAudit is the server-side sidecar of a rejected login. Username
// is recorded because an unknown-username attempt has no local user UUID; in
// that case UserID is empty and the payload is marked anonymous.
type loginFailureAudit struct {
	OrganizationID   string
	UserID           string
	Username         string
	Reason           string
	IPAddress        string
	UserAgent        string
	LockoutRemaining time.Duration
}

// persistLoginFailureAudit writes the failure audit in a standalone
// transaction. It is deliberately not joined with persistLoginFailure: if the
// audit store is unavailable the lockout counter must still commit, and the
// caller's credential response must stay unchanged.
//
// Only the audit record is written, not an Outbox event: failed logins are a
// high-volume signal and the outbox worker dead-letters aggregate types without
// a registered handler (platform/outbox/worker.go). The records table is the
// audit fact source, and metadata carries the reason/anonymous marker that a
// record column cannot express.
func (s *Service) persistLoginFailureAudit(ctx context.Context, input loginFailureAudit) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo := tx.Records()
		if repo == nil {
			return nil
		}
		recordID, err := s.UUIDGenerator.NewString()
		if err != nil {
			return apperrors.NewInternal("generate login failure audit id")
		}
		metadata := map[string]any{
			"reason":    input.Reason,
			"username":  input.Username,
			"anonymous": strings.TrimSpace(input.UserID) == "",
		}
		if input.LockoutRemaining > 0 {
			metadata["lockout_remaining_seconds"] = int(input.LockoutRemaining.Round(time.Second) / time.Second)
		}
		record := &recordsdomain.Record{
			ID:             recordID,
			OrganizationID: input.OrganizationID,
			ActorID:        input.UserID,
			Action:         audit.EventAuthLoginFailed,
			ResourceType:   "user",
			ResourceID:     input.UserID,
			IP:             input.IPAddress,
			UserAgent:      input.UserAgent,
			Surface:        recordsdomain.SurfaceAuthentication,
			Metadata:       metadata,
			CreatedAt:      s.Clock.Now().UTC(),
		}
		if err := repo.Create(ctx, record); err != nil {
			return apperrors.WrapInternal(err, "create login failure audit record")
		}
		return nil
	})
}

func (s *Service) persistLoginFailure(ctx context.Context, userID, orgID string, now time.Time) (*lockoutattempts.Attempt, error) {
	var attempt *lockoutattempts.Attempt
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		written, writeErr := recordLoginFailureWithTx(ctx, tx, userID, orgID, now)
		if writeErr != nil {
			return writeErr
		}
		attempt = written
		return nil
	})
	return attempt, err
}

// channelFromCommand infers the risk-engine login channel from the LoginCommand.
// This covers every channel that currently funnels through Login; channels
// with their own entry points (device_flow, web3, saml, cas, oauth2) are
// expected to pass their own channel string when wiring RiskEvaluator later.
func channelFromCommand(cmd LoginCommand) string {
	switch {
	case strings.TrimSpace(cmd.SAMLRequest) != "" || strings.TrimSpace(cmd.RelayState) != "":
		return "saml"
	case strings.TrimSpace(cmd.CASService) != "":
		return "cas"
	case strings.TrimSpace(cmd.CodeChallenge) != "" || strings.TrimSpace(cmd.Resource) != "":
		return "oauth2"
	case strings.TrimSpace(cmd.ChallengeID) != "":
		return "password_mfa"
	default:
		return loginMethodPassword
	}
}

// evaluateLoginRisk is the single cross-channel risk gate. Every independent
// login path (password / device / CAS / QR / FaceID) calls this before
// creating a session or minting tokens.
//
// Decisions:
//   - "allow"    → returns nil error, caller proceeds
//   - "deny"     → returns an error carrying DecisionID, caller must reject
//   - "challenge"→ returns an error carrying DecisionID, caller decides how
//     to react per-channel (password → force MFA; device/CAS/QR
//     → treat as deny since those flows have no step-up path)
//   - engine unavailable → fail-closed (no risk eval → no session)
//
// Nil RiskEvaluator → returns "allow" (backward-compat fallback).
type RiskDeniedError struct {
	DecisionID string
}

func (e *RiskDeniedError) Error() string { return "login blocked by risk assessment" }

type RiskChallengeError struct {
	DecisionID string
}

func (e *RiskChallengeError) Error() string {
	return "additional verification required by risk assessment"
}

func (s Service) evaluateLoginRisk(ctx context.Context, tx appuow.UnitOfWork, channel, orgID string, userID *string, ip, userAgent, deviceCredentialID string, signals map[string]any) error {
	if s.RiskEvaluator == nil {
		return nil
	}
	// BE-D-10：引擎信号由内部事实集中采集（跨渠道统一），调用方显式传入的
	// 键优先覆盖。is_vpn/is_tor/geo 键依赖外部情报源（OPEN-API-77）。
	merged := s.collectRiskSignals(ctx, tx, orgID, userID, deviceCredentialID, userAgent, s.Clock.Now())
	for k, v := range signals {
		merged[k] = v
	}
	var ipPtr *string
	ip = strings.TrimSpace(ip)
	if ip != "" {
		ipPtr = &ip
	}
	result, err := s.RiskEvaluator.EvaluateAndRecordLoginRisk(ctx, tx, RiskLoginCommand{
		OrganizationID: orgID, UserID: userID, LoginChannel: channel,
		IP: ipPtr, UserAgent: strings.TrimSpace(userAgent), Signals: merged,
	})
	if err != nil {
		slog.WarnContext(ctx, "risk engine unavailable, failing closed",
			"organization_id", orgID, "channel", channel, "error", err)
		return apperrors.NewUnavailable("risk engine unavailable")
	}
	switch result.Decision {
	case "deny":
		slog.InfoContext(ctx, "risk denied login",
			"organization_id", orgID, "channel", channel, "decision_id", result.DecisionID, "score", result.Score)
		return &RiskDeniedError{DecisionID: result.DecisionID}
	case "challenge":
		slog.InfoContext(ctx, "risk challenge",
			"organization_id", orgID, "channel", channel, "decision_id", result.DecisionID, "score", result.Score)
		return &RiskChallengeError{DecisionID: result.DecisionID}
	default:
		slog.InfoContext(ctx, "risk allowed login",
			"organization_id", orgID, "channel", channel, "decision_id", result.DecisionID, "score", result.Score)
		return nil
	}
}
