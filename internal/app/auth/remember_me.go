package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	rememberme "github.com/guliangiam/guliangiam/internal/domain/rememberme"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const (
	// RememberMeTTL is the hard lifetime of a family and every token minted in it.
	RememberMeTTL = 30 * 24 * time.Hour
	// RememberMeTokenPrefix is the non-secret prefix that makes a leaked string
	// identifiable as a remember-me token.
	RememberMeTokenPrefix = "rm_"

	rememberMeTokenRandomBytes = 32
	rememberMeTokenPrefixLen   = 12

	rememberMeRevokeReasonReuse = "reuse_detected"
	rememberMeRevokeReasonUser  = "user"
)

// rememberMeNextID is a seam so tests can keep the pure exchange path
// deterministic. Production keeps the platform UUIDv7 generator.
var rememberMeNextID = platformuuid.V7Generator{}.NewString

// RememberMeIssueResult is the one-time plaintext delivery for a fresh family.
type RememberMeIssueResult struct {
	Token          string    `json:"token"`
	FamilyID       string    `json:"familyId"`
	ExpiresAt      time.Time `json:"expiresAt"`
	UserID         string    `json:"userId,omitempty"`
	OrganizationID string    `json:"organizationId,omitempty"`
}

// RememberMeExchangeResult replaces the consumed token with the next generation.
type RememberMeExchangeResult struct {
	Token          string    `json:"token"`
	FamilyID       string    `json:"familyId"`
	ExpiresAt      time.Time `json:"expiresAt"`
	UserID         string    `json:"userId,omitempty"`
	OrganizationID string    `json:"organizationId,omitempty"`
}

// rememberMeTx is a narrow capability interface: appuow.AuthUoW deliberately
// does not declare RememberMe(), so expanding it (and every fake) is avoided.
// The concrete postgres UoW satisfies this assertion.
type rememberMeTx interface {
	RememberMe() rememberme.Repository
}

func rememberMeRepo(tx appuow.UnitOfWork) (rememberme.Repository, error) {
	if p, ok := tx.(rememberMeTx); ok {
		if repo := p.RememberMe(); repo != nil {
			return repo, nil
		}
	}
	return nil, apperrors.NewInternal("remember-me repository unavailable")
}

// rememberMeAuditSink writes one security audit row inside the caller's
// transaction. It is a function (not a Service method) so the exchange core can
// be exercised with a fake repository and no audit backend.
type rememberMeAuditSink func(ctx context.Context, action, organizationID, userID, familyID string, payload map[string]any) error

func (s *Service) IssueRememberMe(ctx context.Context, organizationID, userID, applicationID string) (*RememberMeIssueResult, error) {
	organizationID = strings.TrimSpace(organizationID)
	userID = strings.TrimSpace(userID)
	if organizationID == "" || userID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId and userId are required")
	}

	now := s.Clock.Now().UTC()
	var result *RememberMeIssueResult
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, repoErr := rememberMeRepo(tx)
		if repoErr != nil {
			return repoErr
		}
		var issueErr error
		result, issueErr = s.issueRememberMeInTx(ctx, tx, repo, organizationID, userID, strings.TrimSpace(applicationID), now)
		return issueErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) issueRememberMeInTx(ctx context.Context, tx appuow.UnitOfWork, repo rememberme.Repository, organizationID, userID, applicationID string, now time.Time) (*RememberMeIssueResult, error) {
	familyID, err := rememberMeNextID()
	if err != nil {
		return nil, apperrors.NewInternal("generate remember-me family id")
	}
	tokenID, err := rememberMeNextID()
	if err != nil {
		return nil, apperrors.NewInternal("generate remember-me token id")
	}
	plaintext, hash, prefix, err := generateRememberMeToken()
	if err != nil {
		return nil, apperrors.NewInternal("generate remember-me token")
	}

	var appPtr *string
	if applicationID != "" {
		value := applicationID
		appPtr = &value
	}

	expiresAt := now.Add(RememberMeTTL)
	family := &rememberme.Family{
		ID:             familyID,
		OrganizationID: organizationID,
		UserID:         userID,
		ApplicationID:  appPtr,
		ExpiresAt:      expiresAt,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	if err := repo.CreateFamily(ctx, family); err != nil {
		return nil, apperrors.WrapInternal(err, "create remember-me family")
	}

	token := &rememberme.Token{
		ID:             tokenID,
		FamilyID:       familyID,
		OrganizationID: organizationID,
		TokenHash:      hash,
		TokenPrefix:    prefix,
		ExpiresAt:      expiresAt,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	if err := repo.CreateToken(ctx, token); err != nil {
		return nil, apperrors.WrapInternal(err, "create remember-me token")
	}

	// 审计载荷只包含非敏感字段：绝不记录明文或哈希。
	if err := s.embedAuditOutboxPlain(
		tx,
		ctx,
		"",
		"",
		"",
		organizationID,
		userID,
		"auth.remember_me.issued",
		"RememberMeFamily",
		familyID,
		"RememberMeFamily",
		"auth.remember_me.issued",
		map[string]any{
			"expires_at":     expiresAt,
			"application_id": applicationID,
		},
	); err != nil {
		return nil, err
	}

	return &RememberMeIssueResult{
		Token:          plaintext,
		FamilyID:       familyID,
		ExpiresAt:      expiresAt,
		UserID:         userID,
		OrganizationID: organizationID,
	}, nil
}

// ExchangeRememberMe is the anonymous rotation endpoint. It never trusts a
// client-supplied organization or user: the token hash resolves the row, and
// the row's tenant/user become the audit context.
func (s *Service) ExchangeRememberMe(ctx context.Context, plaintext string) (*RememberMeExchangeResult, error) {
	now := s.Clock.Now().UTC()
	var result *RememberMeExchangeResult
	var resultErr error
	// tenantctx.System lets the anonymous lookup read the RLS-protected tables;
	// the token hash is the only selector. The callback always returns nil so a
	// reuse-detection revocation still commits before the 401 is returned.
	err := s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, repoErr := rememberMeRepo(tx)
		if repoErr != nil {
			resultErr = repoErr
			return nil
		}
		audit := func(auditCtx context.Context, action, organizationID, userID, familyID string, payload map[string]any) error {
			return s.embedAuditOutboxPlain(
				tx,
				auditCtx,
				"",
				"",
				"",
				organizationID,
				userID,
				action,
				"RememberMeFamily",
				familyID,
				"RememberMeFamily",
				action,
				payload,
			)
		}
		result, resultErr = exchangeRememberMeToken(ctx, repo, plaintext, now, audit)
		if resultErr != nil && !rememberMeAuthRejection(resultErr) {
			// 存储/生成类错误必须回滚，避免“旧 token 已消费但新 token 未写入”。
			return resultErr
		}
		return nil
	})
	if err != nil {
		return nil, apperrors.WrapInternal(err, "exchange remember-me token")
	}
	if resultErr != nil {
		return nil, resultErr
	}
	return result, nil
}

// exchangeRememberMeToken is the side-effect-only core of the exchange. It
// rotates a single-use token, revokes the whole family on reuse, and returns a
// 401-shaped error for every invalid state.
func exchangeRememberMeToken(ctx context.Context, repo rememberme.Repository, plaintext string, now time.Time, audit rememberMeAuditSink) (*RememberMeExchangeResult, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return nil, apperrors.NewUnauthorized("invalid remember-me token")
	}

	token, err := repo.GetTokenByHash(ctx, rememberMeTokenHash(plaintext))
	if err != nil {
		if rememberMeNotFound(err) {
			return nil, apperrors.NewUnauthorized("invalid remember-me token")
		}
		return nil, apperrors.WrapInternal(err, "load remember-me token")
	}
	family, err := repo.GetFamily(ctx, token.FamilyID)
	if err != nil {
		if rememberMeNotFound(err) {
			return nil, apperrors.NewUnauthorized("invalid remember-me token")
		}
		return nil, apperrors.WrapInternal(err, "load remember-me family")
	}
	// 跨租户 token/家族组合一律按无效处理（正常签发不会出现）。
	if token.OrganizationID != family.OrganizationID {
		return nil, apperrors.NewUnauthorized("invalid remember-me token")
	}

	if token.ConsumedAt != nil {
		return nil, revokeRememberMeFamilyForReuse(ctx, repo, family, now, audit)
	}
	if !rememberMeTokenUsable(token, now) || !rememberMeFamilyActive(family, now) {
		return nil, apperrors.NewUnauthorized("invalid remember-me token")
	}

	if err := repo.ConsumeToken(ctx, token.ID, now); err != nil {
		if !rememberMeNotFound(err) {
			return nil, apperrors.WrapInternal(err, "consume remember-me token")
		}
		// 并发下另一个请求先消费了同一 token：按重用处理，撤销整个家族。
		return nil, revokeRememberMeFamilyForReuse(ctx, repo, family, now, audit)
	}

	next, nextPlaintext, err := issueRememberMeTokenTx(ctx, repo, family, now)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "rotate remember-me token")
	}
	if audit != nil {
		if auditErr := audit(ctx, "auth.remember_me.rotated", family.OrganizationID, family.UserID, family.ID, map[string]any{
			"expires_at": next.ExpiresAt,
		}); auditErr != nil {
			slog.WarnContext(ctx, "failed to audit remember-me rotation", "family_id", family.ID, "error", auditErr)
		}
	}

	return &RememberMeExchangeResult{
		Token:          nextPlaintext,
		FamilyID:       family.ID,
		ExpiresAt:      family.ExpiresAt,
		UserID:         family.UserID,
		OrganizationID: family.OrganizationID,
	}, nil
}

// RevokeRememberMe always acts on the authenticated user's own families. An
// empty familyID revokes every active family of that user. Cross-tenant or
// cross-user family IDs are reported as 404 so existence does not leak.
func (s *Service) RevokeRememberMe(ctx context.Context, organizationID, userID, familyID string) error {
	organizationID = strings.TrimSpace(organizationID)
	userID = strings.TrimSpace(userID)
	familyID = strings.TrimSpace(familyID)
	if organizationID == "" || userID == "" {
		return apperrors.NewInvalidArgument("organizationId and userId are required")
	}

	now := s.Clock.Now().UTC()
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, err := rememberMeRepo(tx)
		if err != nil {
			return err
		}

		auditFamily := familyID
		if familyID == "" {
			count, revokeErr := repo.RevokeAllForUser(ctx, organizationID, userID, now, rememberMeRevokeReasonUser)
			if revokeErr != nil {
				return apperrors.WrapInternal(revokeErr, "revoke remember-me families")
			}
			auditFamily = ""
			if auditErr := s.embedAuditOutboxPlain(
				tx, ctx, "", "", "",
				organizationID, userID,
				"auth.remember_me.revoked", "RememberMeFamily", "",
				"RememberMeFamily", "auth.remember_me.revoked",
				map[string]any{"revoked_families": count, "scope": "all"},
			); auditErr != nil {
				return auditErr
			}
			return nil
		}

		family, getErr := repo.GetFamily(ctx, familyID)
		if getErr != nil {
			return getErr
		}
		if family.OrganizationID != organizationID || family.UserID != userID {
			return apperrors.NewNotFound("remember-me family not found")
		}
		// 幂等：已撤销的家族不再写入，重复调用仍返回成功。
		if err := repo.RevokeFamily(ctx, familyID, now, rememberMeRevokeReasonUser); err != nil {
			return apperrors.WrapInternal(err, "revoke remember-me family")
		}
		return s.embedAuditOutboxPlain(
			tx, ctx, "", "", "",
			organizationID, userID,
			"auth.remember_me.revoked", "RememberMeFamily", auditFamily,
			"RememberMeFamily", "auth.remember_me.revoked",
			map[string]any{},
		)
	})
}

// VerifyRememberMeForLogin consumes a valid token inside the login transaction
// and mints the next generation. Any error must be treated as a fall-through to
// the normal MFA flow by the caller; the error itself is never surfaced.
func (s *Service) VerifyRememberMeForLogin(ctx context.Context, tx appuow.UnitOfWork, organizationID, userID, plaintext string, now time.Time) (string, time.Time, error) {
	repo, err := rememberMeRepo(tx)
	if err != nil {
		return "", time.Time{}, err
	}

	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return "", time.Time{}, apperrors.NewUnauthorized("invalid remember-me token")
	}
	token, err := repo.GetTokenByHash(ctx, rememberMeTokenHash(plaintext))
	if err != nil {
		if rememberMeNotFound(err) {
			return "", time.Time{}, apperrors.NewUnauthorized("invalid remember-me token")
		}
		return "", time.Time{}, apperrors.WrapInternal(err, "load remember-me token")
	}
	family, err := repo.GetFamily(ctx, token.FamilyID)
	if err != nil {
		if rememberMeNotFound(err) {
			return "", time.Time{}, apperrors.NewUnauthorized("invalid remember-me token")
		}
		return "", time.Time{}, apperrors.WrapInternal(err, "load remember-me family")
	}
	// 跨租户/跨用户一律 401，且不向登录侧暴露任何细节。
	if token.OrganizationID != organizationID ||
		family.OrganizationID != organizationID ||
		family.UserID != userID {
		return "", time.Time{}, apperrors.NewUnauthorized("invalid remember-me token")
	}

	if token.ConsumedAt != nil {
		_ = revokeRememberMeFamilyForReuse(ctx, repo, family, now, nil)
		slog.DebugContext(ctx, "remember-me reuse detected during login", "family_id", family.ID)
		return "", time.Time{}, apperrors.NewUnauthorized("invalid remember-me token")
	}
	if !rememberMeTokenUsable(token, now) || !rememberMeFamilyActive(family, now) {
		return "", time.Time{}, apperrors.NewUnauthorized("invalid remember-me token")
	}
	if err := repo.ConsumeToken(ctx, token.ID, now); err != nil {
		if !rememberMeNotFound(err) {
			return "", time.Time{}, apperrors.WrapInternal(err, "consume remember-me token")
		}
		_ = revokeRememberMeFamilyForReuse(ctx, repo, family, now, nil)
		return "", time.Time{}, apperrors.NewUnauthorized("invalid remember-me token")
	}

	next, nextPlaintext, err := issueRememberMeTokenTx(ctx, repo, family, now)
	if err != nil {
		return "", time.Time{}, apperrors.WrapInternal(err, "rotate remember-me token")
	}
	slog.InfoContext(ctx, "remember-me token rotated during login", "family_id", family.ID, "user_id", userID)
	return nextPlaintext, next.ExpiresAt, nil
}

func revokeRememberMeFamilyForReuse(ctx context.Context, repo rememberme.Repository, family *rememberme.Family, now time.Time, audit rememberMeAuditSink) error {
	if err := repo.RevokeFamily(ctx, family.ID, now, rememberMeRevokeReasonReuse); err != nil {
		slog.WarnContext(ctx, "failed to revoke remember-me family on reuse", "family_id", family.ID, "error", err)
	}
	if err := repo.RevokeTokensByFamily(ctx, family.ID, now); err != nil {
		slog.WarnContext(ctx, "failed to invalidate remember-me tokens on reuse", "family_id", family.ID, "error", err)
	}
	if audit != nil {
		if auditErr := audit(ctx, "auth.remember_me.reuse_detected", family.OrganizationID, family.UserID, family.ID, map[string]any{
			"reason": rememberMeRevokeReasonReuse,
		}); auditErr != nil {
			slog.WarnContext(ctx, "failed to audit remember-me reuse", "family_id", family.ID, "error", auditErr)
		}
	}
	return apperrors.NewUnauthorized("remember-me token reuse detected")
}

func issueRememberMeTokenTx(ctx context.Context, repo rememberme.Repository, family *rememberme.Family, now time.Time) (*rememberme.Token, string, error) {
	tokenID, err := rememberMeNextID()
	if err != nil {
		return nil, "", err
	}
	plaintext, hash, prefix, err := generateRememberMeToken()
	if err != nil {
		return nil, "", err
	}
	token := &rememberme.Token{
		ID:             tokenID,
		FamilyID:       family.ID,
		OrganizationID: family.OrganizationID,
		TokenHash:      hash,
		TokenPrefix:    prefix,
		ExpiresAt:      family.ExpiresAt,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	if err := repo.CreateToken(ctx, token); err != nil {
		return nil, "", err
	}
	return token, plaintext, nil
}

func generateRememberMeToken() (string, string, string, error) {
	buf := make([]byte, rememberMeTokenRandomBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", err
	}
	plaintext := RememberMeTokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	prefix := plaintext
	if len(prefix) > rememberMeTokenPrefixLen {
		prefix = prefix[:rememberMeTokenPrefixLen]
	}
	return plaintext, rememberMeTokenHash(plaintext), prefix, nil
}

func rememberMeTokenHash(plaintext string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(plaintext)))
	return hex.EncodeToString(sum[:])
}

func rememberMeFamilyActive(family *rememberme.Family, now time.Time) bool {
	return family != nil && family.RevokedAt == nil && family.ExpiresAt.After(now)
}

func rememberMeTokenUsable(token *rememberme.Token, now time.Time) bool {
	return token != nil && token.ConsumedAt == nil && token.ExpiresAt.After(now)
}

// rememberMeNotFound distinguishes "row absent / already consumed" (an
// authentication outcome) from storage failures (an internal error).
func rememberMeNotFound(err error) bool {
	var appErr *apperrors.Error
	return errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound
}

func rememberMeAuthRejection(err error) bool {
	var appErr *apperrors.Error
	return errors.As(err, &appErr) && appErr.Code == apperrors.CodeUnauthorized
}
