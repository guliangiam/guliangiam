package auth

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/guliangiam/guliangiam/internal/app/uow"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

func NewService(unitOfWork uow.UnitOfWork) Service {
	return Service{
		Clock:           clock.SystemClock{},
		UUIDGenerator:   platformuuid.V7Generator{},
		UOW:             unitOfWork,
		permissionCache: &sync.Map{},
	}
}

func NewServiceWithDependencies(clockProvider clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.UnitOfWork) Service {
	// Wire verification adapters to use the repository-based implementation.
	verifyEmailCode = func(ctx context.Context, dest string, passcode string) error {
		return verifyEmailCodeWithRepo(ctx, unitOfWork.Verifications(), dest, passcode)
	}
	verifySmsCode = func(ctx context.Context, phone string, countryCode string, passcode string) error {
		return verifySmsCodeWithRepo(ctx, unitOfWork.Verifications(), phone, countryCode, passcode)
	}

	return Service{
		Clock:           clockProvider,
		UUIDGenerator:   uuidGenerator,
		UOW:             unitOfWork,
		permissionCache: &sync.Map{},
	}
}

func isIgnorableLogoutError(err error) bool {
	typed, ok := err.(*apperrors.Error)
	if !ok {
		return false
	}

	return typed.Code == apperrors.CodeNotFound || typed.Code == apperrors.CodeUnauthorized
}

// revokePreviousSessionForRotation 撤销「登录/轮换新会话」请求携带的旧会话。
//
// 语义是失败即回滚：撤销返回的非可忽略错误（数据库、连接、超时等）会原样返回
// 给调用方，让登录事务回滚、本次登录失败。否则会留下「新会话已签发 + 旧会话仍
// 有效」的中间态 —— 调用方以为旧会话已被踢掉，实际没有。
//
// 可忽略错误沿用登出路径的 isIgnorableLogoutError 判定：
//   - CodeNotFound：旧会话不存在（已被清理、伪造或早已删除），没有可撤销对象，
//     也就不存在残留会话，不应因为过期 cookie 把一次正常登录打成失败；
//   - CodeUnauthorized：旧会话已不可用，效果等同于已撤销。
//
// 已撤销的会话不构成错误：仓储 Revoke 对 revoked_at 非空的行幂等返回 nil。
func revokePreviousSessionForRotation(ctx context.Context, repo sessionsdomain.Repository, previousSessionID string, now time.Time) error {
	if strings.TrimSpace(previousSessionID) == "" {
		return nil
	}

	if err := repo.Revoke(ctx, previousSessionID, now); err != nil && !isIgnorableLogoutError(err) {
		return apperrors.WrapInternalf(err, "revoke previous session %s during login rotation", previousSessionID)
	}

	return nil
}
