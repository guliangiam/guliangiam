package crypto

import (
	"testing"
	"time"

	"github.com/alexedwards/argon2id"
)

func TestHashTokenValue_Deterministic(t *testing.T) {
	token := "gl_pat_abc123def456"
	hash1 := HashTokenValue(token)
	hash2 := HashTokenValue(token)
	if hash1 != hash2 {
		t.Error("same input should produce same hash")
	}
}

func TestHashTokenValue_DifferentInputs(t *testing.T) {
	hash1 := HashTokenValue("token-a")
	hash2 := HashTokenValue("token-b")
	if hash1 == hash2 {
		t.Error("different inputs should produce different hashes")
	}
}

func TestHashTokenValue_OutputLength(t *testing.T) {
	hash := HashTokenValue("test-token")
	if len(hash) != 64 {
		t.Errorf("SHA-256 hex length = %d, want 64", len(hash))
	}
}

func TestHashTokenValue_EmptyString(t *testing.T) {
	hash := HashTokenValue("")
	if len(hash) != 64 {
		t.Errorf("empty string hash length = %d, want 64", len(hash))
	}
}

func TestHashClientSecret_AndVerify(t *testing.T) {
	secret := "my-super-secret-client-key"
	hash, err := HashClientSecret(secret)
	if err != nil {
		t.Fatalf("HashClientSecret() error = %v", err)
	}
	if hash == "" {
		t.Fatal("HashClientSecret() returned empty hash")
	}

	ok, err := VerifyClientSecret(secret, hash)
	if err != nil {
		t.Fatalf("VerifyClientSecret() error = %v", err)
	}
	if !ok {
		t.Error("VerifyClientSecret() = false, want true for correct secret")
	}
}

func TestVerifyClientSecret_WrongSecret(t *testing.T) {
	hash, err := HashClientSecret("correct-secret")
	if err != nil {
		t.Fatalf("HashClientSecret() error = %v", err)
	}

	ok, err := VerifyClientSecret("wrong-secret", hash)
	if err != nil {
		t.Fatalf("VerifyClientSecret() error = %v", err)
	}
	if ok {
		t.Error("VerifyClientSecret() = true, want false for wrong secret")
	}
}

func TestHashClientSecret_DifferentSecrets(t *testing.T) {
	hash1, err := HashClientSecret("secret-a")
	if err != nil {
		t.Fatalf("HashClientSecret() error = %v", err)
	}
	hash2, err := HashClientSecret("secret-b")
	if err != nil {
		t.Fatalf("HashClientSecret() error = %v", err)
	}
	if hash1 == hash2 {
		t.Error("different secrets should produce different hashes")
	}
}

func TestHashClientSecret_SameSecretDifferentHashes(t *testing.T) {
	hash1, err := HashClientSecret("same-secret")
	if err != nil {
		t.Fatalf("HashClientSecret() error = %v", err)
	}
	hash2, err := HashClientSecret("same-secret")
	if err != nil {
		t.Fatalf("HashClientSecret() error = %v", err)
	}
	if hash1 == hash2 {
		t.Error("argon2id with random salt should produce different hashes for same secret")
	}
}

// TestClientSecretArgon2ParametersArePinned 锁定 argon2id 的 m/t/p/salt/key
// 常量，并把依据（OWASP Password Storage Cheat Sheet 的 Argon2id 档位）固化在
// 断言里。参数调整属于密钥迁移动作，必须同时更新此测试与注释。
func TestClientSecretArgon2ParametersArePinned(t *testing.T) {
	params := clientSecretParams()
	if params.Memory != 64*1024 {
		t.Fatalf("Memory = %d, want 65536 KiB (OWASP minimum is 47104 KiB)", params.Memory)
	}
	if params.Iterations != 3 {
		t.Fatalf("Iterations = %d, want 3", params.Iterations)
	}
	if params.Parallelism != 2 {
		t.Fatalf("Parallelism = %d, want 2", params.Parallelism)
	}
	if params.SaltLength != 16 {
		t.Fatalf("SaltLength = %d, want 16 bytes", params.SaltLength)
	}
	if params.KeyLength != 32 {
		t.Fatalf("KeyLength = %d, want 32 bytes", params.KeyLength)
	}
}

// TestBurnVerificationUsesSameCostAsRealVerify 证明“客户端不存在 vs 密钥错误”
// 的抹平不是靠猜：dummy 哈希的编码参数与真实哈希参数逐项相同，且实测耗时
// 同量级。这样调用方无法用错误码或响应时间判别客户端是否存在。
func TestBurnVerificationUsesSameCostAsRealVerify(t *testing.T) {
	dummyParams, _, _, err := argon2id.DecodeHash(dummyClientSecretHash)
	if err != nil {
		t.Fatalf("DecodeHash(dummy) error = %v", err)
	}
	params := clientSecretParams()
	if dummyParams.Memory != params.Memory ||
		dummyParams.Iterations != params.Iterations ||
		dummyParams.Parallelism != params.Parallelism ||
		dummyParams.SaltLength != params.SaltLength ||
		dummyParams.KeyLength != params.KeyLength {
		t.Fatalf("dummy hash params %+v differ from real params %+v", dummyParams, params)
	}

	if BurnClientSecretVerification("presented-secret") {
		t.Fatal("BurnClientSecretVerification must always return false")
	}

	hash, err := HashClientSecret("real-secret")
	if err != nil {
		t.Fatalf("HashClientSecret() error = %v", err)
	}

	best := func(runs int, fn func()) time.Duration {
		var min time.Duration
		for i := 0; i < runs; i++ {
			start := time.Now()
			fn()
			if elapsed := time.Since(start); min == 0 || elapsed < min {
				min = elapsed
			}
		}
		return min
	}

	realVerify := best(3, func() { _, _ = VerifyClientSecret("real-secret", hash) })
	burnVerify := best(3, func() { BurnClientSecretVerification("presented-secret") })

	// 宽松区间：CI 抖动下仍要求两者处于同一量级（真实校验不会比 dummy 快 3 倍）。
	if burnVerify*3 < realVerify {
		t.Fatalf("burn verify %v is far cheaper than real verify %v; timing is distinguishable", burnVerify, realVerify)
	}
}
