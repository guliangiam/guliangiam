package scopeauthorizer

import (
	"context"
	"sync"
	"time"

	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
)

// PrincipalTypeResolver 从 session user_id 解析主体类型。
// Phase C-Step 1：session.PrincipalType 的真实值来源。
type PrincipalTypeResolver interface {
	// Resolve 根据 session user_id 返回 PrincipalType。
	// 未查到 platform_admin 记录 → 默认 "user"。
	Resolve(ctx context.Context, userID string) string
}

// cachedDBResolver 从 platform_admins 表查询 + 60s 内存缓存。
// 用 PlatformAdminRepository（内部 TenantFilterSystem ctx 绕过 RLS）。
// now 是可替换的时钟接缝：生产注入 time.Now，测试注入假时钟。
type cachedDBResolver struct {
	adminRepo platformadmins.PlatformAdminRepository
	mu        sync.RWMutex
	cache     map[string]cachedEntry
	ttl       time.Duration
	now       func() time.Time
}

type cachedEntry struct {
	principalType string
	expiresAt     time.Time
}

// NewCachedDBResolver 构造带缓存的 DB resolver。
// adminRepo 为 nil 时 Resolve 永远返回 "user"（测试友好）。
// now 为 nil 时默认 time.Now。
func NewCachedDBResolver(adminRepo platformadmins.PlatformAdminRepository, now func() time.Time) *cachedDBResolver {
	if now == nil {
		now = time.Now
	}
	return &cachedDBResolver{
		adminRepo: adminRepo,
		cache:     make(map[string]cachedEntry),
		ttl:       60 * time.Second,
		now:       now,
	}
}

// Resolve 查 platform_admins.user_id = userID AND is_active = TRUE。
// 通过 PlatformAdminRepository.GetByUserID，内部自动 TenantFilterSystem ctx 绕过 RLS。
// 查不到 / 出错 / inactive → 默认 "user"（fail-open for principal classification，不是 authorization）。
func (r *cachedDBResolver) Resolve(ctx context.Context, userID string) string {
	if r.adminRepo == nil {
		return "user"
	}

	// 1. 查缓存
	r.mu.RLock()
	if entry, ok := r.cache[userID]; ok && r.now().Before(entry.expiresAt) {
		r.mu.RUnlock()
		return entry.principalType
	}
	r.mu.RUnlock()

	// 2. 查 DB（repository 内部 TenantFilterSystem ctx 绕过 RLS）
	result := "user"
	if admin, err := r.adminRepo.GetByUserID(ctx, userID); err == nil {
		if admin.IsActive {
			result = "platform_admin"
		}
	}
	// NotFound / DB error → 保持 "user"（fail-open）

	// 3. 写缓存
	r.mu.Lock()
	r.cache[userID] = cachedEntry{
		principalType: result,
		expiresAt:     r.now().Add(r.ttl),
	}
	r.mu.Unlock()

	return result
}

// 确保 cachedDBResolver 满足 PrincipalTypeResolver。
var _ PrincipalTypeResolver = (*cachedDBResolver)(nil)

// Invalidate drops the cached principal classification for one user. It is
// called when a platform admin is deactivated so Gate 5/7 stop treating the
// account as platform_admin immediately instead of after the 60s TTL.
func (r *cachedDBResolver) Invalidate(userID string) {
	if r == nil || userID == "" {
		return
	}
	r.mu.Lock()
	delete(r.cache, userID)
	r.mu.Unlock()
}
