package auth

import (
	"context"
	"log/slog"
	"strings"

	"github.com/guliangiam/guliangiam/cred"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	invitationsdomain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// ResetPasswordCommand contains parameters for resetting a user's password.
type ResetPasswordCommand struct {
	OrganizationSlug  string
	Username          string
	VerificationCode  string
	NewPassword       string
	PreviousSessionID string // S9-sensitive: old session to revoke
	// CaptchaToken / CaptchaID / ClientIP feed the organization's captcha gate.
	// The console already renders a captcha on the reset step; without these
	// fields the collected token was silently discarded and a required human
	// check could be skipped by calling the API directly.
	CaptchaToken string
	CaptchaID    string
	ClientIP     string
}

// CheckPasswordCommand contains parameters for checking a user's password.
type CheckPasswordCommand struct {
	OrganizationSlug string
	Username         string
	Password         string
}

// SignupCommand contains parameters for self-service user registration.
type SignupCommand struct {
	OrganizationSlug string
	Username         string
	Password         string
	Email            string
	Phone            string
	SignupAppID      string
	VerificationCode string
	InvitationCode   string
	CaptchaToken     string
	CaptchaID        string
	ClientIP         string
}

// ResetPasswordResult contains the result of a password reset operation.
type ResetPasswordResult struct {
	Status string `json:"status"`
}

// CheckPasswordResult contains the result of a password check operation.
type CheckPasswordResult struct {
	Valid bool `json:"valid"`
}

// SignupResult contains the result of a signup operation.
type SignupResult struct {
	Status  string `json:"status"`
	UserID  string `json:"userId,omitempty"`
	Message string `json:"message,omitempty"`
}

// resolveVerificationType determines the verification type based on which
// recipient field is non-empty, preferring email over phone.
func resolveVerificationType(email, phone string) (recipient, vType string) {
	email = strings.TrimSpace(email)
	phone = strings.TrimSpace(phone)
	if email != "" {
		return email, contactEmail
	}
	if phone != "" {
		return phone, contactPhone
	}
	return "", ""
}

// ResetPassword resets a user's password after verifying the verification code.
// Returns a generic success message regardless of whether the user exists,
// to prevent user enumeration attacks.
func (s *Service) ResetPassword(ctx context.Context, command ResetPasswordCommand) (*ResetPasswordResult, string, error) {
	if err := validateResetPasswordCommand(command); err != nil {
		return nil, "", err
	}

	// Anonymous flow: organizations/users are FORCE RLS and the caller has no
	// tenant scope yet — resolving org/user from the request body is inherent
	// to the reset handshake (mirrors Login, login_flow.go tenantctx.System).
	ctx = tenantctx.System(ctx)

	organization, err := s.UOW.Organizations().GetBySlug(ctx, command.OrganizationSlug)
	if err != nil {
		// Return generic message to avoid revealing org existence.
		slog.DebugContext(ctx, "password reset organization lookup failed", "error", err)
		return &ResetPasswordResult{Status: verificationPurposeReset}, "", nil
	}

	// Human-check gate: when the organization enables a captcha provider, a
	// valid response is required before the credential is changed. Placed after
	// org resolution (the gate is org-scoped) and before the user lookup so a
	// failed captcha cannot be used as an account-existence oracle.
	if err := s.EnforceCaptcha(ctx, organization.ID, command.CaptchaToken, command.CaptchaID, command.ClientIP); err != nil {
		return nil, "", err
	}

	// Policy and breach checks are organization-level (no user rows): run them
	// before the user lookup so a weak-password 422 cannot reveal whether the
	// user exists, and keep the breach query outside any transaction
	// (login.md §泄露口令检查).
	if err := s.ValidatePasswordPolicy(ctx, organization.ID, command.Username, command.NewPassword); err != nil {
		return nil, "", err
	}
	breachUnverified, breachErr := s.checkPasswordBreach(ctx, command.NewPassword)
	if breachErr != nil {
		return nil, "", breachErr
	}

	user, err := s.UOW.Users().GetByUsername(ctx, organization.ID, command.Username)
	if err != nil {
		// Return generic message to avoid revealing user existence.
		slog.DebugContext(ctx, "password reset user lookup failed", "error", err)
		return &ResetPasswordResult{Status: verificationPurposeReset}, "", nil
	}

	// Determine the recipient and verification type.
	recipient, vType := resolveVerificationType(user.Profile.Email, user.Profile.Phone)
	if recipient == "" {
		// Return generic message to avoid revealing user has no contact info.
		return &ResetPasswordResult{Status: verificationPurposeReset}, "", nil
	}

	// Update the password.
	credManager := cred.GetCredManager(hashArgon2ID)
	if credManager == nil {
		return nil, "", apperrors.NewInternal("credential manager unavailable")
	}

	now := s.Clock.Now()
	passwordHash := credManager.GetHashedPassword(command.NewPassword, "")
	if passwordHash == "" {
		return nil, "", apperrors.NewInternal("hash password")
	}

	newSessionID := ""
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Verify the verification code WITHIN the transaction to prevent TOCTOU attacks.
		// A concurrent request cannot consume the same code between validation and consumption.
		verification, err := matchVerificationCode(
			ctx, tx.Verifications(), organization.ID, recipient, vType, strings.TrimSpace(command.VerificationCode),
		)
		if err != nil {
			// Return an error here since the user already got past the existence check.
			return verificationMatchError(err, "verification code is invalid or expired")
		}

		// Check password history within the transaction
		if err := s.ValidatePasswordPolicyWithHistory(ctx, tx, organization.ID, user.ID, command.Username, command.NewPassword); err != nil {
			return err
		}

		// Save a history entry before overwriting. The old plaintext is not
		// available in a reset flow, so record the new password with the same
		// peppered algorithm the check uses. A write failure must roll back the
		// password change (same transaction).
		historyHash, hashErr := createPasswordHistoryHash(command.NewPassword)
		if hashErr != nil {
			return apperrors.NewInternal("hash password history")
		}
		historyID, histErr := s.UUIDGenerator.NewString()
		if histErr != nil {
			return apperrors.NewInternal("generate password history id")
		}
		if err := tx.Users().CreatePasswordHistory(ctx, &userdomain.PasswordHistoryEntry{
			ID:           historyID,
			UserID:       user.ID,
			PasswordHash: historyHash,
			PasswordAlgo: userdomain.PasswordAlgorithmArgon2ID,
			CreatedAt:    now,
		}); err != nil {
			return err
		}

		credentials := userdomain.UserCredentials{
			UserID:                user.ID,
			HasPassword:           true,
			PasswordHash:          passwordHash,
			PasswordAlgo:          hashArgon2ID,
			PasswordUpdatedAt:     &now,
			AccessKey:             user.Credentials.AccessKey,
			AccessSecretHash:      user.Credentials.AccessSecretHash,
			AccessSecretRotatedAt: user.Credentials.AccessSecretRotatedAt,
		}

		user.Credentials = credentials
		user.UpdatedAt = now
		user.Revision++

		if err := tx.Users().Update(ctx, user); err != nil {
			return err
		}

		if err := tx.Users().UpdateCredentials(ctx, &credentials); err != nil {
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
			withBreachAuditMarker(map[string]any{}, breachUnverified),
		); err != nil {
			return err
		}
		// Prune password history beyond the configured limit
		org, _ := tx.Organizations().GetByID(ctx, organization.ID)
		policy := resolvePasswordPolicy(org)
		if policy.PasswordHistoryCount > 0 {
			_ = tx.Users().DeletePasswordHistoryBeyond(ctx, user.ID, policy.PasswordHistoryCount)
		}

		// Consume the verification code within the same transaction.
		if err := tx.Verifications().UpdateStatus(ctx, verification.ID, "consumed"); err != nil {
			return err
		}

		// AC-AUTH-009: password reset is a credential-changing operation. Cascade
		// revoke the user's ENTIRE session set + refresh family + access tokens,
		// then issue a fresh session for the caller so they don't get logged out.
		if strings.TrimSpace(command.PreviousSessionID) != "" {
			var oldSess *sessionsdomain.Session
			if sess, getErr := tx.Sessions().GetByID(ctx, command.PreviousSessionID); getErr == nil {
				oldSess = sess
			}

			if _, err := tx.Sessions().RevokeByUser(ctx, user.ID, now); err != nil {
				return err
			}
			if _, err := tx.Tokens().RevokeRefreshByUser(ctx, user.ID, now); err != nil {
				return err
			}
			if _, err := tx.Tokens().RevokeByQuery(ctx, tokendomain.RevokeQuery{UserID: user.ID}, now); err != nil {
				return err
			}

			if oldSess != nil {
				sessID, idErr := s.UUIDGenerator.NewString()
				if idErr != nil {
					return apperrors.NewInternal("generate rotated session id")
				}
				sid, sidErr := newSessionSid()
				if sidErr != nil {
					return sidErr
				}
				// 轮换会话继承原会话应用的签到会话策略（login_flow.go 同口径）：
				// 未启用 EnableSigninSession 的应用只发短 TTL，不再硬编码 30 天。
				effectiveTTL := sessionTTL
				if oldSess.ApplicationID != "" {
					if application, appErr := tx.Applications().GetByID(ctx, oldSess.ApplicationID); appErr == nil && application != nil && !application.EnableSigninSession {
						effectiveTTL = shortSessionTTL
					}
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
					ExpiresAt:        now.Add(effectiveTTL),
					CreatedAt:        now,
					LastSeenAt:       now,
					UpdatedAt:        now,
					Revision:         1,
				}); cErr != nil {
					return cErr
				}
				newSessionID = sessID
			}
		}
		return nil
	})
	if err != nil {
		// If the error is from the verification code check, return generic message.
		if typed, ok := err.(*apperrors.Error); ok && typed.Code == apperrors.CodeInvalidArgument {
			return &ResetPasswordResult{Status: verificationPurposeReset}, "", nil
		}
		return nil, "", err
	}

	s.notifyUser(ctx, NotificationCommand{
		OrganizationID: organization.ID,
		UserID:         user.ID,
		Category:       "security",
		EventType:      "user.password.reset",
		Title:          "密码已重置",
		Content:        "你的账号密码刚刚被重置。如非本人操作，请立即联系管理员并检查登录记录。",
		Link:           "/account",
		SourceType:     "user",
		SourceID:       user.ID,
	})

	return &ResetPasswordResult{Status: verificationPurposeReset}, newSessionID, nil
}

func (s *Service) CheckPassword(ctx context.Context, command CheckPasswordCommand) (*CheckPasswordResult, error) {
	if strings.TrimSpace(command.Username) == "" {
		return nil, apperrors.NewInvalidArgument("username is required")
	}
	if strings.TrimSpace(command.Password) == "" {
		return nil, apperrors.NewInvalidArgument("password is required")
	}

	organization, err := s.UOW.Organizations().GetBySlug(ctx, command.OrganizationSlug)
	if err != nil {
		return nil, err
	}

	user, err := s.UOW.Users().GetByUsername(ctx, organization.ID, command.Username)
	if err != nil {
		// Don't reveal whether the user exists.
		slog.DebugContext(ctx, "check password user lookup failed", "error", err)
		return &CheckPasswordResult{Valid: false}, nil
	}

	// A user without a local hash (LDAP/social-only, or a row whose hash was
	// cleared) never validates as a local password. verifyPassword also
	// rejects provider markers such as "ldap" instead of selecting a verifier.
	if err := verifyPassword(command.Password, user.Credentials.PasswordAlgo, user.Credentials.PasswordHash); err != nil {
		if isCredentialError(err) {
			return &CheckPasswordResult{Valid: false}, nil
		}
		return nil, err
	}
	return &CheckPasswordResult{Valid: true}, nil
}

// Signup performs self-service user registration.
func (s *Service) Signup(ctx context.Context, command SignupCommand) (*SignupResult, error) {
	if err := validateSignupCommand(command); err != nil {
		return nil, err
	}

	// Anonymous flow: organizations/users/invitations are FORCE RLS and the
	// caller has no tenant scope yet — resolving the org from the request body
	// is inherent to the signup handshake (mirrors Login, login_flow.go
	// tenantctx.System).
	ctx = tenantctx.System(ctx)

	organization, err := s.UOW.Organizations().GetBySlug(ctx, command.OrganizationSlug)
	if err != nil {
		return nil, err
	}

	// Human-check gate: when the organization enables a captcha provider, a
	// valid response is required before any account is created (anti-bot).
	if err := s.EnforceCaptcha(ctx, organization.ID, command.CaptchaToken, command.CaptchaID, command.ClientIP); err != nil {
		return nil, err
	}

	// Check if organization allows self-registration.
	if organization.DisableSignup {
		// If an invitation code is provided, allow signup even if registration is disabled.
		if strings.TrimSpace(command.InvitationCode) == "" {
			return nil, apperrors.NewForbidden("registration is disabled for this organization")
		}
	}

	// Validate invitation code if provided.
	var invitationOrgID string
	var invitationRoleIDs []string
	var invitationGroupIDs []string
	if strings.TrimSpace(command.InvitationCode) != "" {
		invitation, err := s.UOW.Invitations().GetByCode(ctx, strings.TrimSpace(command.InvitationCode))
		if err != nil {
			return nil, apperrors.NewInvalidArgument("invalid invitation code")
		}
		if !invitation.IsAcceptable(s.Clock.Now()) {
			switch {
			case invitation.Status == invitationsdomain.StatusAccepted:
				return nil, apperrors.NewInvitationUsed("invitation has already been used")
			case invitation.Status == invitationsdomain.StatusExpired || invitation.IsExpired(s.Clock.Now()):
				return nil, apperrors.NewInvitationExpired("invitation has expired")
			default:
				return nil, apperrors.NewInvalidArgument("invitation is not acceptable")
			}
		}
		// Ensure the invitation belongs to the same organization.
		if invitation.OrganizationID != organization.ID {
			return nil, apperrors.NewInvalidArgument("invitation does not belong to this organization")
		}
		invitationOrgID = invitation.OrganizationID
		invitationRoleIDs = invitation.RoleIDs
		invitationGroupIDs = invitation.GroupIDs
		// Email-targeted invitation: an empty signup email inherits the
		// invitation's; a different email is rejected so the binding cannot
		// be bypassed.
		if invitation.Email != "" {
			if strings.TrimSpace(command.Email) == "" {
				command.Email = invitation.Email
			} else if !strings.EqualFold(strings.TrimSpace(command.Email), invitation.Email) {
				return nil, apperrors.NewUnprocessableEntity("email does not match the invitation")
			}
		}
	}

	// Validate password against the organization's password policy.
	if err := s.ValidatePasswordPolicy(ctx, organization.ID, command.Username, command.Password); err != nil {
		return nil, err
	}

	// Breach check outside the transaction (may issue an outbound range query).
	breachUnverified, breachErr := s.checkPasswordBreach(ctx, command.Password)
	if breachErr != nil {
		return nil, breachErr
	}

	// Hash the password.
	credManager := cred.GetCredManager(hashArgon2ID)
	if credManager == nil {
		return nil, apperrors.NewInternal("credential manager unavailable")
	}

	passwordHash := credManager.GetHashedPassword(command.Password, "")
	if passwordHash == "" {
		return nil, apperrors.NewInternal("hash password")
	}

	now := s.Clock.Now()
	var userID string

	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Check if username already exists in this organization within the transaction
		// to prevent race conditions between the check and the insert.
		existingUser, err := tx.Users().GetByUsername(ctx, organization.ID, command.Username)
		if err == nil && existingUser != nil {
			return apperrors.NewConflict("username already exists in this organization")
		}

		// Verify the verification code WITHIN the transaction if provided.
		// This prevents TOCTOU attacks where the code is validated outside
		// and consumed inside a different transaction.
		if strings.TrimSpace(command.VerificationCode) != "" {
			recipient, vType := resolveVerificationType(
				strings.TrimSpace(command.Email),
				strings.TrimSpace(command.Phone),
			)
			if recipient == "" {
				return apperrors.NewInvalidArgument("email or phone is required when providing a verification code")
			}
			verification, err := matchVerificationCode(
				ctx, tx.Verifications(), organization.ID, recipient, vType, strings.TrimSpace(command.VerificationCode),
			)
			if err != nil {
				return verificationMatchError(err, "verification code is invalid or expired")
			}
			// Consume the verification code within the same transaction.
			if err := tx.Verifications().UpdateStatus(ctx, verification.ID, "consumed"); err != nil {
				return err
			}
		}

		id, err := s.UUIDGenerator.NewString()
		if err != nil {
			return apperrors.NewInternal("generate user id")
		}

		user := &userdomain.User{
			ID:                  id,
			OrganizationID:      organization.ID,
			Username:            strings.TrimSpace(command.Username),
			UserType:            userTypeNormal,
			Status:              statusActive,
			SignupApplicationID: strings.TrimSpace(command.SignupAppID),
			RegisterType:        "Application Signup",
			RegisterSource:      organization.Slug + "/" + strings.TrimSpace(command.SignupAppID),
			Score:               organization.InitScore,
			CreatedAt:           now,
			UpdatedAt:           now,
			Revision:            1,
			Profile: userdomain.UserProfile{
				UserID:      id,
				DisplayName: strings.TrimSpace(command.Username),
				Email:       strings.TrimSpace(command.Email),
				Phone:       strings.TrimSpace(command.Phone),
				AvatarURL:   organization.DefaultAvatar,
			},
			Credentials: userdomain.UserCredentials{
				UserID:            id,
				HasPassword:       true,
				PasswordHash:      passwordHash,
				PasswordAlgo:      hashArgon2ID,
				PasswordUpdatedAt: &now,
			},
		}

		// Assign the first tag from the organization's tag list.
		if len(organization.Settings.Tags) > 0 {
			user.Tag = organization.Settings.Tags[0]
		}

		if err := tx.Users().Create(ctx, user); err != nil {
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
			// 审计动作区分自助注册与管理端建号（user.created）；outbox 事件
			// 类型保留 user.created —— SCIM/JML connector 契约
			// （domain/connectors.EvtUserCreated）按该值分发，不得更名。
			"user.registered",
			"user",
			user.ID,
			"user",
			"user.created",
			withBreachAuditMarker(map[string]any{}, breachUnverified),
		); err != nil {
			return err
		}

		// Save initial password to history with the same peppered algorithm the
		// check uses; a write failure rolls back the signup.
		historyHash, hashErr := createPasswordHistoryHash(command.Password)
		if hashErr != nil {
			return apperrors.NewInternal("hash password history")
		}
		historyID, histErr := s.UUIDGenerator.NewString()
		if histErr != nil {
			return apperrors.NewInternalf("generate password history id: %v", histErr)
		}
		if err := tx.Users().CreatePasswordHistory(ctx, &userdomain.PasswordHistoryEntry{
			ID:           historyID,
			UserID:       id,
			PasswordHash: historyHash,
			PasswordAlgo: userdomain.PasswordAlgorithmArgon2ID,
			CreatedAt:    now,
		}); err != nil {
			return err
		}

		userID = id

		// Apply invitation role bindings and group memberships if provided.
		if invitationOrgID != "" {
			if len(invitationRoleIDs) > 0 {
				roleBindings := make([]userdomain.UserRoleBinding, 0, len(invitationRoleIDs))
				for _, roleID := range invitationRoleIDs {
					roleBindings = append(roleBindings, userdomain.UserRoleBinding{UserID: id, RoleID: roleID})
				}
				if err := tx.Users().ReplaceRoleBindings(ctx, id, roleBindings); err != nil {
					slog.WarnContext(ctx, "failed to apply invitation role bindings", "error", err, "user_id", id)
				}
			}
			if len(invitationGroupIDs) > 0 {
				groupMemberships := make([]userdomain.UserGroupMembership, 0, len(invitationGroupIDs))
				for _, groupID := range invitationGroupIDs {
					groupMemberships = append(groupMemberships, userdomain.UserGroupMembership{UserID: id, GroupID: groupID})
				}
				if err := tx.Users().ReplaceGroupMemberships(ctx, id, groupMemberships); err != nil {
					slog.WarnContext(ctx, "failed to apply invitation group memberships", "error", err, "user_id", id)
				}
			}
			// Mark the invitation as accepted. 失败必须回滚注册：否则邀请码可被无限复用。
			invitation, invErr := tx.Invitations().GetByCode(ctx, strings.TrimSpace(command.InvitationCode))
			if invErr == nil && invitation.IsAcceptable(s.Clock.Now()) {
				now := s.Clock.Now()
				invitation.Status = "accepted"
				invitation.AcceptedAt = &now
				invitation.UpdatedAt = now
				// Update 的乐观锁守卫传自增前的 revision（仓储层 Revision-1），与 revoke/resend 一致。
				invitation.Revision++
				if err := s.embedAuditOutboxPlain(
					tx,
					ctx,
					"",
					"",
					"",
					invitation.OrganizationID,
					"",
					"invitation.updated",
					"invitation",
					invitation.ID,
					"invitation",
					"invitation.updated",
					map[string]any{},
				); err != nil {
					return err
				}
				if err := tx.Invitations().Update(ctx, invitation); err != nil {
					return err
				}
			} else if invErr != nil {
				return invErr
			} else {
				// 并发场景：同码另一请求已先行接受，此处必须拒绝，防止一码多用。
				return apperrors.NewInvitationUsed("invitation has already been used")
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// ADR-029 post_registration：事务已提交，fail-open —— 失败只记
	// 日志与审计，绝不把已成功的注册变成错误。
	s.dispatchPostRegistration(ctx, organization.ID, userID, strings.TrimSpace(command.Username),
		strings.TrimSpace(command.SignupAppID), "Application Signup")

	return &SignupResult{
		Status: "created",
		UserID: userID,
	}, nil
}

func validateResetPasswordCommand(command ResetPasswordCommand) error {
	if strings.TrimSpace(command.OrganizationSlug) == "" {
		return apperrors.NewInvalidArgument("organizationSlug is required")
	}
	if strings.TrimSpace(command.Username) == "" {
		return apperrors.NewInvalidArgument("username is required")
	}
	if strings.TrimSpace(command.VerificationCode) == "" {
		return apperrors.NewInvalidArgument("verificationCode is required")
	}
	if strings.TrimSpace(command.NewPassword) == "" {
		return apperrors.NewInvalidArgument("newPassword is required")
	}
	return nil
}

func validateSignupCommand(command SignupCommand) error {
	if strings.TrimSpace(command.OrganizationSlug) == "" {
		return apperrors.NewInvalidArgument("organizationSlug is required")
	}
	if strings.TrimSpace(command.Username) == "" {
		return apperrors.NewInvalidArgument("username is required")
	}
	if strings.TrimSpace(command.Password) == "" {
		return apperrors.NewInvalidArgument("password is required")
	}
	return nil
}
