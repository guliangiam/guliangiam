package jwks

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

const (
	// defaultUnknownKidCooldown bounds how often one unknown kid may trigger a
	// source refresh. It is short relative to maxAge: it only absorbs repeated
	// probes for a kid the source does not publish, while a genuine rotation
	// still gets one refresh on demand.
	defaultUnknownKidCooldown = 30 * time.Second
	// maxUnknownKidRefreshesPerWindow caps distinct-kid refreshes per issuer
	// inside one cooldown window. Without it an attacker rotating the kid on
	// every request would bypass the per-kid cooldown and turn a verification
	// flood into an outbound-request flood.
	maxUnknownKidRefreshesPerWindow = 8
	// maxUnknownKidAttempts bounds the per-kid cooldown map. Unknown kids are
	// caller-controlled, so the map must not grow without limit.
	maxUnknownKidAttempts = 4096
)

// RemoteJWKSCache 从外部 issuer 的 jwks_uri 拉取 JWKS 并缓存。
// 与 Manager（本系统本地 key pair）解耦 — 本缓存只管外部 issuer 的公钥集合。
//
// 设计约束（与 infra/jwt/issuer.go 风格一致）：
//   - RS256 only — 对称 HS* 禁止（与 ValidateSigningAlgorithm 平台策略对齐）
//   - 零第三方依赖 — 纯 crypto/rsa + encoding/json；single-flight 用互斥锁 +
//     通道实现，不引入 golang.org/x/sync
//   - 主动刷新 + 被动过期 — 后台 goroutine 按 refreshInterval 刷新；
//     调用 GetPublicKey 时如缓存超过 maxAge 则先刷新
//   - 未知 kid 回源一次（AC-JWKS-002）— 新鲜缓存未命中触发单次回源，
//     回源后仍未知则 fail-close；同一 kid 冷却窗口内不重复回源
type RemoteJWKSCache struct {
	mu              sync.RWMutex
	cache           map[string]*cachedJWKS // jwks_uri → cached
	httpClient      *http.Client
	refreshInterval time.Duration
	maxAge          time.Duration
	stopCh          chan struct{}

	unknownKidCooldown time.Duration
	// unknownKidAttempts records the last demand refresh per jwks_uri+"\x00"+kid.
	unknownKidAttempts map[string]time.Time
	// unknownKidWindows bounds distinct-kid refreshes per jwks_uri.
	unknownKidWindows map[string]*unknownKidWindow
	// flights deduplicates concurrent refreshes of the same jwks_uri: one
	// caller fetches, the rest wait for the same result.
	flights map[string]*refreshFlight
}

type unknownKidWindow struct {
	start time.Time
	count int
}

type refreshFlight struct {
	done   chan struct{}
	cached *cachedJWKS
	err    error
}

func (f *refreshFlight) wait(ctx context.Context) (*cachedJWKS, error) {
	select {
	case <-f.done:
		return f.cached, f.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type cachedJWKS struct {
	keys      map[string]*rsa.PublicKey // kid → pub
	fetchedAt time.Time
}

// NewRemoteJWKSCache 构造远程 JWKS 缓存。
// refreshInterval: 后台刷新周期（默认 5min）
// maxAge: 缓存最长保留期（默认 30min，超过则强制刷新）
// httpClient: 可注入自定义（超时控制），nil 则用 10s timeout 的默认 client
func NewRemoteJWKSCache(refreshInterval, maxAge time.Duration, httpClient *http.Client) *RemoteJWKSCache {
	if refreshInterval <= 0 {
		refreshInterval = 5 * time.Minute
	}
	if maxAge <= 0 {
		maxAge = 30 * time.Minute
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	c := &RemoteJWKSCache{
		cache:              make(map[string]*cachedJWKS),
		httpClient:         httpClient,
		refreshInterval:    refreshInterval,
		maxAge:             maxAge,
		unknownKidCooldown: defaultUnknownKidCooldown,
		unknownKidAttempts: make(map[string]time.Time),
		unknownKidWindows:  make(map[string]*unknownKidWindow),
		flights:            make(map[string]*refreshFlight),
		stopCh:             make(chan struct{}),
	}
	recover.SafeGo("jwks-refresh", c.refreshLoop)
	return c
}

// Close 停止后台刷新 goroutine。
func (c *RemoteJWKSCache) Close() {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
}

func (c *RemoteJWKSCache) refreshLoop() {
	ticker := time.NewTicker(c.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.refreshAll()
		case <-c.stopCh:
			return
		}
	}
}

func (c *RemoteJWKSCache) refreshAll() {
	c.mu.RLock()
	uris := make([]string, 0, len(c.cache))
	for uri := range c.cache {
		uris = append(uris, uri)
	}
	c.mu.RUnlock()
	for _, uri := range uris {
		_, _ = c.refresh(context.Background(), uri) // 忽略错误，下次再来
	}
}

// GetPublicKey 从 jwks_uri 对应的缓存里按 kid 取公钥。
//
// 解析顺序（AC-JWKS-002）：
//  1. 命中未过期缓存 → 直接返回；已退役 kid 在重叠窗口内仍可验签。
//  2. 缓存新鲜但缺 kid → 触发一次回源（受冷却与 single-flight 约束），
//     刷新后重新解析；仍缺 kid 则明确拒绝，绝不回退到其他 key/算法。
//  3. 缓存缺失/过期 → 回源；成功则按 kid 解析，失败时仅当旧缓存仍持有该
//     kid 才降级（令牌在重叠窗口内仍可验），未知 kid 一律拒绝。
func (c *RemoteJWKSCache) GetPublicKey(ctx context.Context, jwksURI, kid string) (*rsa.PublicKey, error) {
	if jwksURI == "" {
		return nil, apperrors.NewInvalidArgument("jwks_uri is empty")
	}

	// 快速路径：命中未过期缓存
	c.mu.RLock()
	cached, hit := c.cache[jwksURI]
	fresh := hit && time.Since(cached.fetchedAt) < c.maxAge
	var pub *rsa.PublicKey
	if fresh {
		pub = cached.keys[kid]
	}
	c.mu.RUnlock()

	if fresh {
		if pub != nil {
			return pub, nil
		}
		return c.refreshForUnknownKid(ctx, jwksURI, kid)
	}

	// 缓存 miss 或过期 → 强制刷新
	refreshed, err := c.refresh(ctx, jwksURI)
	if err != nil {
		// 刷新失败 — 旧缓存（即使过期）仍是重叠窗口内的本地验证材料，但只
		// 有它确实持有该 kid 时才降级；未知 kid 不允许借失败刷新放行。
		c.mu.RLock()
		old, oldHas := c.cache[jwksURI]
		c.mu.RUnlock()
		if oldHas {
			if pub, ok := old.keys[kid]; ok {
				return pub, nil
			}
		}
		return nil, apperrors.WrapInternalf(err, "fetch JWKS from %s", jwksURI)
	}

	pub, ok := refreshed.keys[kid]
	if !ok {
		recordUnknownKid("rejected")
		return nil, unknownSigningKeyError()
	}
	return pub, nil
}

// refreshForUnknownKid 处理「缓存新鲜但没有该 kid」：最多回源一次，然后
// 要么返回新解析到的公钥，要么以 unauthorized 拒绝。并发请求通过 flights
// 共享同一次回源；同一 kid 在冷却窗口内不会重复探测，同一 issuer 在一个
// 窗口内的不同 kid 探测总量有上限（防 kid 轮换绕过冷却）。
func (c *RemoteJWKSCache) refreshForUnknownKid(ctx context.Context, jwksURI, kid string) (*rsa.PublicKey, error) {
	attemptKey := unknownKidAttemptKey(jwksURI, kid)

	c.mu.Lock()
	if flight, ok := c.flights[jwksURI]; ok {
		c.mu.Unlock()
		return c.awaitFlight(ctx, jwksURI, kid, flight)
	}

	now := time.Now()
	if c.unknownKidCooldown > 0 {
		if last, ok := c.unknownKidAttempts[attemptKey]; ok && now.Sub(last) < c.unknownKidCooldown {
			c.mu.Unlock()
			recordUnknownKid("cooldown")
			return nil, unknownSigningKeyError()
		}
		if !c.admitUnknownKidRefreshLocked(jwksURI, now) {
			c.mu.Unlock()
			recordUnknownKid("throttled")
			return nil, unknownSigningKeyError()
		}
		c.recordUnknownKidAttemptLocked(attemptKey, now)
	}

	flight := &refreshFlight{done: make(chan struct{})}
	c.flights[jwksURI] = flight
	c.mu.Unlock()

	cached, err := c.fetch(ctx, jwksURI)
	c.finishFlight(jwksURI, flight, cached, err)
	return c.resolveRefreshedKid(cached, err, jwksURI, kid)
}

// awaitFlight 等待同 issuer 上已在途的回源，并与发起者拿到同一结果。
func (c *RemoteJWKSCache) awaitFlight(ctx context.Context, jwksURI, kid string, flight *refreshFlight) (*rsa.PublicKey, error) {
	cached, err := flight.wait(ctx)
	return c.resolveRefreshedKid(cached, err, jwksURI, kid)
}

func (c *RemoteJWKSCache) resolveRefreshedKid(cached *cachedJWKS, err error, jwksURI, kid string) (*rsa.PublicKey, error) {
	if err != nil {
		recordUnknownKid("refresh_failed")
		return nil, apperrors.WrapInternalf(err, "refresh JWKS from %s", jwksURI)
	}
	if pub, ok := cached.keys[kid]; ok {
		recordUnknownKid("resolved")
		return pub, nil
	}
	recordUnknownKid("rejected")
	return nil, unknownSigningKeyError()
}

// refresh 对同一 jwks_uri 做 single-flight 回源：首个调用者拉取，其余等待。
func (c *RemoteJWKSCache) refresh(ctx context.Context, jwksURI string) (*cachedJWKS, error) {
	c.mu.Lock()
	if flight, ok := c.flights[jwksURI]; ok {
		c.mu.Unlock()
		return flight.wait(ctx)
	}
	flight := &refreshFlight{done: make(chan struct{})}
	c.flights[jwksURI] = flight
	c.mu.Unlock()

	cached, err := c.fetch(ctx, jwksURI)
	c.finishFlight(jwksURI, flight, cached, err)
	return cached, err
}

func (c *RemoteJWKSCache) finishFlight(jwksURI string, flight *refreshFlight, cached *cachedJWKS, err error) {
	c.mu.Lock()
	flight.cached = cached
	flight.err = err
	delete(c.flights, jwksURI)
	close(flight.done)
	c.mu.Unlock()
}

// admitUnknownKidRefreshLocked 限制单一 issuer 在一个冷却窗口内由未知 kid
// 触发的回源总数。计数窗口与 per-kid 冷却同长，避免 kid 变化绕过防抖。
func (c *RemoteJWKSCache) admitUnknownKidRefreshLocked(jwksURI string, now time.Time) bool {
	window := c.unknownKidWindows[jwksURI]
	if window == nil || now.Sub(window.start) >= c.unknownKidCooldown {
		window = &unknownKidWindow{start: now}
		c.unknownKidWindows[jwksURI] = window
	}
	if window.count >= maxUnknownKidRefreshesPerWindow {
		return false
	}
	window.count++
	return true
}

func (c *RemoteJWKSCache) recordUnknownKidAttemptLocked(attemptKey string, now time.Time) {
	if len(c.unknownKidAttempts) >= maxUnknownKidAttempts {
		for key, at := range c.unknownKidAttempts {
			if now.Sub(at) >= c.unknownKidCooldown {
				delete(c.unknownKidAttempts, key)
			}
		}
		if len(c.unknownKidAttempts) >= maxUnknownKidAttempts {
			// Dropping the debounce is preferable to unbounded growth on a
			// caller-controlled key space.
			for key := range c.unknownKidAttempts {
				delete(c.unknownKidAttempts, key)
			}
		}
	}
	c.unknownKidAttempts[attemptKey] = now
}

func unknownKidAttemptKey(jwksURI, kid string) string {
	return jwksURI + "\x00" + kid
}

// unknownSigningKeyError 是「回源后仍未知」的 fail-close 结果：不得降级为
// 其他公钥或算法（AC-JWKS-002 / AC-TOKEN-007）。
func unknownSigningKeyError() error {
	return apperrors.NewUnauthorized("unknown signing key: kid not present in JWKS")
}

// fetch 从 jwksURI 拉取并解析 JWKS → 存缓存。
func (c *RemoteJWKSCache) fetch(ctx context.Context, jwksURI string) (cached *cachedJWKS, err error) {
	defer func() {
		result := "success"
		if err != nil {
			result = "failure"
		}
		jwksRefreshTotal.WithLabelValues(result).Inc()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.Newf(apperrors.CodeUnavailable, "JWKS fetch: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 最多 1MB
	if err != nil {
		return nil, err
	}

	var jwks JWKS
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, apperrors.WrapInternal(err, "JWKS parse")
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" {
			continue // 只支持 RSA（RS256）
		}
		pub, err := jwkToRSAPublicKey(&jwk)
		if err != nil {
			continue // 跳过坏的 key，不阻止整个 JWKS
		}
		if jwk.Kid == "" {
			continue // 没有 kid 的 JWK 没法按 kid 索引
		}
		keys[jwk.Kid] = pub
	}

	cached = &cachedJWKS{keys: keys, fetchedAt: time.Now().UTC()}

	c.mu.Lock()
	c.cache[jwksURI] = cached
	c.mu.Unlock()

	return cached, nil
}

// jwkToRSAPublicKey 把 JWK (RSA) 转 *rsa.PublicKey。
// 只支持标准 RSA 格式（n + e），不支持 RSA-PSS 或 ec 曲线。
func jwkToRSAPublicKey(jwk *JWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		// 有些 Issuer 用标准 base64 — 容错
		nBytes, err = base64.StdEncoding.DecodeString(jwk.N)
		if err != nil {
			return nil, apperrors.WrapInternal(err, "decode JWK n")
		}
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		eBytes, err = base64.StdEncoding.DecodeString(jwk.E)
		if err != nil {
			return nil, apperrors.WrapInternal(err, "decode JWK e")
		}
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes).Int64()
	if e <= 0 {
		return nil, apperrors.New(apperrors.CodeUnprocessableEntity, "invalid JWK e value")
	}

	pub := &rsa.PublicKey{
		N: n,
		E: int(e),
	}

	// 验证格式合法性
	if _, err := x509.MarshalPKIXPublicKey(pub); err != nil {
		return nil, apperrors.WrapInternal(err, "invalid RSA public key")
	}
	return pub, nil
}
