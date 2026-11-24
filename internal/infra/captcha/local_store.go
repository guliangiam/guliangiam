package captcha

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrRedisUnavailable 表示 Redis store 构造前置检查失败（客户端缺失/Ping 不通）。
// 调用方（bootstrap）fail-open：回退 MemoryStore 并记日志，不阻断启动与登录。
var ErrRedisUnavailable = errors.New("captcha redis store unavailable")

// RedisStore 用 go-redis v9 实现 Store（本地图形验证码答案存储：
// default / go_captcha 共用）。key 前缀 captcha:，TTL 由构造函数决定。
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisStore 构造 Redis store 并做连通性 Ping；失败返回 error 供调用方降级。
func NewRedisStore(client *redis.Client, ttl time.Duration) (*RedisStore, error) {
	if client == nil {
		return nil, ErrRedisUnavailable
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &RedisStore{client: client, ttl: ttl}, nil
}

// Set 实现 Store。
func (s *RedisStore) Set(ctx context.Context, id, code string) error {
	return s.client.Set(ctx, "captcha:"+id, code, s.ttl).Err()
}

// Get 实现 Store（不消费；消费由 Delete 显式完成，与 MemoryStore 语义一致）。
func (s *RedisStore) Get(ctx context.Context, id string) (string, error) {
	v, err := s.client.Get(ctx, "captcha:"+id).Result()
	if errors.Is(err, redis.Nil) {
		return "", errNotFound
	}
	return v, err
}

// Delete 实现 Store。
func (s *RedisStore) Delete(ctx context.Context, id string) error {
	return s.client.Del(ctx, "captcha:"+id).Err()
}

// b64Store 把本项目 ctx 版 Store 适配为 base64Captcha 的无 ctx Store 接口
// （Set(id,value) / Get(id,clear)）。TTL 由被包装 Store 自身管理（Redis TTL /
// Memory 过期时间），适配层不再传 ttl。
type b64Store struct {
	store Store
}

// Set 实现 base64Captcha.Store。
func (a b64Store) Set(id string, value string) error {
	return a.store.Set(context.Background(), id, value)
}

// Get 实现 base64Captcha.Store；clear=true 读取后删除（一次性消费）。
func (a b64Store) Get(id string, clear bool) string {
	v, err := a.store.Get(context.Background(), id)
	if err != nil {
		return ""
	}
	if clear {
		_ = a.store.Delete(context.Background(), id)
	}
	return v
}

// Verify 实现 base64Captcha.Store（大小写不敏感比较，与 DefaultProvider.Verify 一致）。
func (a b64Store) Verify(id, answer string, clear bool) bool {
	v := a.Get(id, clear)
	return strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(answer))
}

var (
	_ Store = (*MemoryStore)(nil)
	_ Store = (*RedisStore)(nil)
)
