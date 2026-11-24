package crypto

import (
	"time"

	"github.com/alexedwards/argon2id"
)

// Argon2id 参数。依据 OWASP Password Storage Cheat Sheet（2024）对 Argon2id
// 的推荐档位 m=47104 KiB、t=1、p=1 的“更高内存、更多轮次”方向：这里取
// m=64 MiB、t=3、p=2，CPU 与内存成本都不低于该推荐值。
//
// 这些值必须与库中已存哈希的编码参数一致；调整参数属于密钥迁移动作，
// 不能只改常量。
const (
	// Argon2MemoryKiB 为每条哈希占用的内存（64 MiB）。
	Argon2MemoryKiB uint32 = 64 * 1024
	// Argon2Iterations 为时间成本（轮数）。
	Argon2Iterations uint32 = 3
	// Argon2Parallelism 为并行度（线程数）。
	Argon2Parallelism uint8 = 2
	// Argon2SaltLength 为随机盐长度（字节）。
	Argon2SaltLength uint32 = 16
	// Argon2KeyLength 为派生密钥长度（字节，即 256 bit）。
	Argon2KeyLength uint32 = 32
)

// dummyClientSecretHash 是一个用上述参数生成、对应固定随机口令的合法哈希。
// 令牌端点在对端客户端不存在或没有可用 secret 时，仍然执行一次同成本的
// 校验，使“客户端不存在”和“密钥错误”在耗时口径上不可区分。
const dummyClientSecretHash = "$argon2id$v=19$m=65536,t=3,p=2$JmM9/vyZnEm9oXxluIplzA$ga2u8abOQdThx4MwczTpBNtbUcNVQnbNxGGUGcfzMTw"

func clientSecretParams() *argon2id.Params {
	return &argon2id.Params{
		Memory:      Argon2MemoryKiB,
		Iterations:  Argon2Iterations,
		Parallelism: Argon2Parallelism,
		SaltLength:  Argon2SaltLength,
		KeyLength:   Argon2KeyLength,
	}
}

// HashClientSecret hashes a client secret using argon2id.
// This is used for storing OAuth application client secrets securely.
func HashClientSecret(secret string) (string, error) {
	start := time.Now()
	hash, err := hashClientSecretInner(secret)
	result := "ok"
	if err != nil {
		result = "error"
	}
	argon2Duration.WithLabelValues("hash", result).Observe(time.Since(start).Seconds())
	argon2Total.WithLabelValues("hash", result).Inc()
	return hash, err
}

func hashClientSecretInner(secret string) (string, error) {
	return argon2id.CreateHash(secret, clientSecretParams())
}

// VerifyClientSecret verifies a client secret against an argon2id hash.
func VerifyClientSecret(secret, hash string) (bool, error) {
	start := time.Now()
	ok, err := verifyClientSecretInner(secret, hash)
	result := "ok"
	if err != nil {
		result = "error"
	}
	argon2Duration.WithLabelValues("verify", result).Observe(time.Since(start).Seconds())
	argon2Total.WithLabelValues("verify", result).Inc()
	return ok, err
}

func verifyClientSecretInner(secret, hash string) (bool, error) {
	// alexedwards/argon2id 用 subtle.ConstantTimeCompare 比较派生密钥与解码结果，
	// 不会因前缀匹配提前返回。
	return argon2id.ComparePasswordAndHash(secret, hash)
}

// BurnClientSecretVerification 在客户端不存在、或没有可用 secret 行时执行一次
// 与真实校验等成本的 dummy 校验，用于抹平“客户端不存在 vs 密钥错误”的耗时差。
// 返回值恒为 false。
func BurnClientSecretVerification(secret string) bool {
	ok, _ := VerifyClientSecret(secret, dummyClientSecretHash)
	return ok
}
