package auth

import (
	"context"
	"time"

	"log/slog"

	"github.com/guliangiam/guliangiam/cred"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/redact"
)

// ChangePasswordCommand parameters for self-service password change.
// Caller must be authenticated — PreviousSessionID extracted from __Host-session cookie.
type ChangePasswordCommand struct {
	OrganizationSlug  string
	Username          string
	OldPassword       string
	NewPassword       string
	PreviousSessionID string // opaque session ID to capture context before cascade
}

// ChangePasswordResult is returned on successful password change.
type ChangePasswordResult struct {
	Status       string
	RevokedCount int64
	RefreshCount int64
	AccessCount  int64
}

// ChangePassword lets an authenticated user change their own password.
// Unlike password:reset (which uses a verification code and can be public),
// password:change verifies the OLD password first — it only runs for callers
// who already have a valid session.
//
// AC-AUTH-009: credential-changing op → ForceLogoutUser cascade for every other
// session. The caller's own session is rotated (new opaque ID + same device
// context) so the user is not logged out mid-operation.
//
// 性能（PERF-04）：旧的 argon2id 校验、历史口令校验与新口令/历史哈希计算全部
// 移出数据库事务。校验阶段只用连接池读路径（不占事务连接），事务内只做
// 「确认原口令未被并发修改 + 写入 + 吊销 + 审计」等必须原子的操作，
// 毫秒级的 CPU 计算不再持有连接。
func (s *Service) ChangePassword(ctx context.Context, command ChangePasswordCommand) (*ChangePasswordResult, string, error) {
	if command.OrganizationSlug == "" {
		return nil, "", apperrors.NewInvalidArgument("organizationSlug is required")
	}
	if command.Username == "" {
		return nil, "", apperrors.NewInvalidArgument("username is required")
	}
	if command.OldPassword == "" {
		return nil, "", apperrors.NewInvalidArgument("oldPassword is required")
	}
	if command.NewPassword == "" {
		return nil, "", apperrors.NewInvalidArgument("newPassword is required")
	}
	if command.OldPassword == command.NewPassword {
		return nil, "", apperrors.NewInvalidArgument("new password must differ from old")
	}

	revokedAt := s.Clock.Now().UTC()

	result := &ChangePasswordResult{Status: "changed"}
	var newSessionID string

	err := s.changePassword(ctx, command, revokedAt, result, &newSessionID)
	if err != nil {
		// A constraint error here can carry the password-history row values in
		// its detail, so only the platform error code is logged.
		slog.ErrorContext(ctx, "password change failed",
			"username", command.Username,
			"error_category", redact.ErrorCategory(err))
		return nil, "", err
	}

	slog.InfoContext(ctx, "password changed",
		"username", command.Username,
		"sessions_revoked", result.RevokedCount,
		"refresh_revoked", result.RefreshCount,
		"access_revoked", result.AccessCount)
	return result, newSessionID, nil
}

// changePassword splits the operation into a read phase (one short RLS-scoped
// transaction), a CPU phase outside any transaction, and an atomic write phase
// inside a second transaction.
func (s *Service) changePassword(ctx context.Context, command ChangePasswordCommand, revokedAt time.Time, result *ChangePasswordResult, newSessionID *string) error {
	// organizations/users/password_histories 均为 FORCE RLS 表：池级裸读没有
	// 事务局部 GUC，会被租户隔离遮蔽（NotFound/空历史集导致复用校验旁路）。
	// 读阶段必须走 WithinTransaction，由 setTenantContext 按调用方租户作用域
	// 设置 GUC；Argon2id 校验与策略判定（CPU）保持在事务外。
	var (
		org            *orgdomain.Organization
		user           *userdomain.User
		historyEntries []userdomain.PasswordHistoryEntry
	)
	if readErr := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		org, err = tx.Organizations().GetBySlug(ctx, command.OrganizationSlug)
		if err != nil {
			return err
		}
		user, err = tx.Users().GetByUsername(ctx, org.ID, command.Username)
		if err != nil {
			return err
		}
		if p := resolvePasswordPolicy(org); p.PasswordHistoryCount > 0 {
			historyEntries = fetchPasswordHistoryForReuse(ctx, tx.Users(), user.ID, p.PasswordHistoryCount)
		}
		return nil
	}); readErr != nil {
		return readErr
	}

	// Verify old password — this is the "already-authenticated" gate.
	// Argon2id verification is CPU-bound; it must not run while holding a
	// transaction connection.
	if err := verifyPassword(command.OldPassword, user.Credentials.PasswordAlgo, user.Credentials.PasswordHash); err != nil {
		return err
	}

	// 组织密码策略是改密门槛的唯一事实源：长度/字符类/弱口令/用户名
	// 均在此判定，缺失策略时使用 domain 的显式默认值。
	policy := resolvePasswordPolicy(org)
	if err := validatePasswordAgainstPolicy(command.NewPassword, user.Username, policy); err != nil {
		return err
	}

	// Enforce password history — cannot reuse recent passwords. Entries were
	// fetched in the read transaction; comparison is pure CPU here.
	if policy.PasswordHistoryCount > 0 {
		if histErr := checkPasswordHistoryEntries(historyEntries, command.NewPassword, policy.PasswordHistoryCount); histErr != nil {
			return histErr
		}
	}

	// Breach check runs outside any transaction: it may perform an outbound
	// HTTP range query (AUTH-009) and must not hold a DB connection.
	breachUnverified, breachErr := s.checkPasswordBreach(ctx, command.NewPassword)
	if breachErr != nil {
		return breachErr
	}

	// New local password hashes are always argon2id. The old hash may be a
	// legacy format; it was only accepted above for verification and is now
	// replaced, so a historical algorithm value is never written back.
	algo := userdomain.PasswordAlgorithmArgon2ID
	manager := cred.GetCredManager(algo)
	if manager == nil {
		return apperrors.NewInternal("credential manager unavailable")
	}
	newHash := manager.GetHashedPassword(command.NewPassword, "")
	if newHash == "" {
		return apperrors.NewInternal("hash new password")
	}

	// Snapshot the credential that was verified so the write transaction can
	// detect a concurrent password change instead of overwriting it blindly.
	verifiedHash := user.Credentials.PasswordHash
	verifiedAlgo := user.Credentials.PasswordAlgo
	saveHistory := user.Credentials.HasPassword && verifiedHash != ""

	// Save old password to history BEFORE overwriting. History rows must use
	// the peppered history algorithm; storing the credential hash made every
	// later comparison miss.
	var (
		historyHash string
		historyID   string
	)
	if saveHistory {
		var err error
		historyHash, err = createPasswordHistoryHash(command.OldPassword)
		if err != nil {
			return apperrors.NewInternal("hash password history")
		}
		historyID, err = s.UUIDGenerator.NewString()
		if err != nil {
			return apperrors.NewInternal("generate password history id")
		}
	}

	// ---- write phase: everything that must be atomic stays inside one tx ----
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Re-read the credential inside the transaction and compare it with the
		// snapshot that was verified outside. If another change committed in
		// between, the old password the user proved is no longer authoritative;
		// return the same 401 as a failed verification and write nothing.
		current, err := tx.Users().GetByUsername(ctx, org.ID, command.Username)
		if err != nil {
			return err
		}
		if current.Credentials.PasswordHash != verifiedHash || current.Credentials.PasswordAlgo != verifiedAlgo {
			return apperrors.NewUnauthorized("invalid username or password")
		}
		user = current

		// Capture caller session context BEFORE ForceLogoutUser kills every session row.
		var oldSess *sessionsdomain.Session
		if command.PreviousSessionID != "" {
			if sess, getErr := tx.Sessions().GetByID(ctx, command.PreviousSessionID); getErr == nil {
				oldSess = sess
			}
		}

		if saveHistory {
			if err := tx.Users().CreatePasswordHistory(ctx, &userdomain.PasswordHistoryEntry{
				ID:           historyID,
				UserID:       user.ID,
				PasswordHash: historyHash,
				PasswordAlgo: userdomain.PasswordAlgorithmArgon2ID,
				CreatedAt:    revokedAt,
			}); err != nil {
				return err
			}
		}

		// Update credentials. 只写口令列：整行 upsert 会用零值覆盖
		// access_key/access_secret_hash，导致自助改密顺带吊销访问密钥。
		credentials := &userdomain.UserCredentials{
			UserID:             user.ID,
			HasPassword:        true,
			PasswordHash:       newHash,
			PasswordAlgo:       algo,
			PasswordUpdatedAt:  &revokedAt,
			NeedUpdatePassword: false,
		}
		if err := tx.Users().UpdatePasswordCredential(ctx, credentials); err != nil {
			return err
		}

		// 自助改密：吊销该用户除当前会话外的全部会话（其他设备立即失效），
		// 当前会话在下方轮换（旧会话随后显式吊销，新会话沿用同一设备上下文），
		// 避免调用者在操作中途被自己的改密踢下线。
		result.RevokedCount, err = tx.Sessions().RevokeByUserExcept(ctx, user.ID, command.PreviousSessionID, revokedAt)
		if err != nil {
			return err
		}
		result.RefreshCount, err = tx.Tokens().RevokeRefreshByUser(ctx, user.ID, revokedAt)
		if err != nil {
			return err
		}
		result.AccessCount, err = tx.Tokens().RevokeByQuery(ctx, tokendomain.RevokeQuery{UserID: user.ID}, revokedAt)
		if err != nil {
			return err
		}

		// 自助改密审计 + outbox（同事务）：actor 为当前会话用户本人；
		// payload 仅记录身份与来源，不含任何口令或哈希。
		auditPayload := map[string]any{
			"userId":    user.ID,
			"sessionId": command.PreviousSessionID,
			"method":    "self",
		}
		if breachUnverified {
			auditPayload["breachCheck"] = "unverified"
		}
		if auditErr := s.embedAuditOutboxPlain(
			tx, ctx,
			"", "", recordsdomain.SurfaceAuthentication,
			user.OrganizationID, user.ID,
			"auth.password.changed",
			"User", user.ID,
			"auth", "auth.password.changed",
			auditPayload,
		); auditErr != nil {
			return auditErr
		}

		// Issue fresh session for caller so they continue after the cascade.
		if oldSess != nil {
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
				ExpiresAt:        revokedAt.Add(30 * 24 * time.Hour),
				CreatedAt:        revokedAt,
				LastSeenAt:       revokedAt,
				UpdatedAt:        revokedAt,
				Revision:         1,
			}); cErr != nil {
				return cErr
			}
			// 轮换完成：旧会话显式吊销，同一时刻只保留新会话。
			if rErr := tx.Sessions().Revoke(ctx, oldSess.ID, revokedAt); rErr != nil {
				return rErr
			}
			result.RevokedCount++
			*newSessionID = sessID
		}

		// Prune old password history. 裁剪失败不阻断改密，但须留痕（错误明细
		// 可能含历史哈希，只记类别）。
		if policy.PasswordHistoryCount > 0 {
			if pruneErr := tx.Users().DeletePasswordHistoryBeyond(ctx, user.ID, policy.PasswordHistoryCount); pruneErr != nil {
				slog.WarnContext(ctx, "password history prune failed",
					"user_id", user.ID, "error_category", redact.ErrorCategory(pruneErr))
			}
		}

		return nil
	}); err != nil {
		return err
	}

	// 改密成功后向本人发送安全提醒（其他设备会话已失效）；通知失败只记录日志，
	// 不影响已提交的改密与审计。
	s.notifyUser(ctx, NotificationCommand{
		OrganizationID: user.OrganizationID,
		UserID:         user.ID,
		Category:       "security",
		EventType:      "auth.password.changed",
		Title:          "密码已修改",
		Content:        "你的账号密码已通过「修改密码」更新，其他设备的登录会话已失效。如非本人操作，请立即联系管理员。",
		Link:           "/account",
		SourceType:     "user",
		SourceID:       user.ID,
	})
	return nil
}
