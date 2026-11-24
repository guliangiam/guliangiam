package scopeauthorizer

import (
	"context"
	"sync"

	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
)

// BE-E-14：显式 target 的管理员请求会先经 Authorize（Gate 5 membership）再经
// annotateAuthorizedAdminScope → ResolveAdminScope（又一次 membership），同一
// 请求对 platform_admins 重复查询。这里提供请求级 ctx 缓存：中间件在请求入口
// 安装一次，Authorize 与 ResolveAdminScope 共享同一条 membership 结果。
// 缓存只存活于单个请求的 ctx，不跨请求，避免作用域变更后的陈旧授权。

type adminLookupCacheKey struct{}

type adminLookupResult struct {
	admin *platformadmins.PlatformAdmin
	err   error
}

type adminLookupCache struct {
	mu      sync.Mutex
	entries map[string]adminLookupResult
}

// WithAdminLookupCache 在 ctx 上安装请求级 membership 缓存（幂等）。
func WithAdminLookupCache(ctx context.Context) context.Context {
	if _, ok := ctx.Value(adminLookupCacheKey{}).(*adminLookupCache); ok {
		return ctx
	}
	return context.WithValue(ctx, adminLookupCacheKey{}, &adminLookupCache{
		entries: make(map[string]adminLookupResult),
	})
}

// cachedAdminByUserID 与 repo.GetByUserID 语义一致（含错误与 nil 结果），
// 仅在安装了缓存的 ctx 内去重；错误同样缓存，保持请求内 fail-closed 口径一致。
func cachedAdminByUserID(ctx context.Context, repo platformadmins.PlatformAdminRepository, userID string) (*platformadmins.PlatformAdmin, error) {
	cache, ok := ctx.Value(adminLookupCacheKey{}).(*adminLookupCache)
	if !ok {
		return repo.GetByUserID(ctx, userID)
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if result, ok := cache.entries[userID]; ok {
		return result.admin, result.err
	}
	admin, err := repo.GetByUserID(ctx, userID)
	cache.entries[userID] = adminLookupResult{admin: admin, err: err}
	return admin, err
}
