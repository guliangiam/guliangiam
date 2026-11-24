package scopeauthorizer

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComputeHash_Deterministic 验证 computeHash 可复现性（ExperienceRecall 教训 #1）。
// 相同输入 → 相同 hash；不同输入 → 不同 hash。
func TestComputeHash_Deterministic(t *testing.T) {
	actorID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sourceTenant := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	targetTenant := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	createdAt := time.Date(2026, 10, 17, 10, 30, 0, 0, time.UTC).Truncate(time.Microsecond)

	prevHash := "abc123def456"

	rec := AuditRecord{
		ActorID:      actorID,
		ActorType:    PrincipalPlatformAdmin,
		SourceTenant: sourceTenant,
		TargetTenant: targetTenant,
		ResourceType: "users",
		Action:       "impersonate",
		RiskLevel:    RiskCritical,
		Decision:     "allow",
		Reason:       "scope validation passed",
	}

	hash1 := computeHash(&prevHash, createdAt, rec)
	hash2 := computeHash(&prevHash, createdAt, rec)

	// 相同输入 → 完全相同 hash（可复现）
	assert.Equal(t, hash1, hash2, "same input must produce same hash")

	// SHA-256 hex 长度固定 64 字符
	assert.Len(t, hash1, 64)
	assert.NotEmpty(t, hash1)
	// 后续 KnownValue 测试精确验证输入→输出的可预测性
}

// TestComputeHash_DifferentInput 验证不同输入产生不同 hash。
func TestComputeHash_DifferentInput(t *testing.T) {
	actorID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	createdAt := time.Date(2026, 10, 17, 10, 30, 0, 0, time.UTC).Truncate(time.Microsecond)
	prevHash := "prev123"

	base := AuditRecord{
		ActorID:      actorID,
		ActorType:    PrincipalPlatformAdmin,
		ResourceType: "users",
		Action:       "read",
		RiskLevel:    RiskLow,
		Decision:     "allow",
	}

	hashBase := computeHash(&prevHash, createdAt, base)

	// 每个字段改动 → hash 必须不同
	cases := []struct {
		name   string
		mutate func(AuditRecord) AuditRecord
	}{
		{"decision", func(r AuditRecord) AuditRecord { r.Decision = "deny"; return r }},
		{"action", func(r AuditRecord) AuditRecord { r.Action = "delete"; return r }},
		{"resource_type", func(r AuditRecord) AuditRecord { r.ResourceType = "applications"; return r }},
		{"risk_level", func(r AuditRecord) AuditRecord { r.RiskLevel = RiskHigh; return r }},
		{"reason", func(r AuditRecord) AuditRecord { r.Reason = "blocked"; return r }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mutated := c.mutate(base)
			assert.NotEqual(t, hashBase, computeHash(&prevHash, createdAt, mutated),
				"changing %s must change hash", c.name)
		})
	}

	// prev_hash 改动 → hash 不同
	assert.NotEqual(t, hashBase, computeHash(nil, createdAt, base), "prev_hash nil must differ")

	// created_at 改动 → hash 不同
	laterCreatedAt := createdAt.Add(time.Second)
	assert.NotEqual(t, hashBase, computeHash(&prevHash, laterCreatedAt, base),
		"changing created_at must change hash")
}

// TestComputeHash_EmptyValues 验证空值处理（uuid.Nil → 空字符串）。
func TestComputeHash_EmptyValues(t *testing.T) {
	createdAt := time.Date(2026, 10, 17, 10, 30, 0, 0, time.UTC).Truncate(time.Microsecond)
	rec := AuditRecord{
		ActorType:    PrincipalUser,
		ResourceType: "users",
		Action:       "read",
		RiskLevel:    RiskLow,
		Decision:     "allow",
		// ActorID / SourceTenant / TargetTenant 全部 Nil
	}

	hash := computeHash(nil, createdAt, rec)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64) // SHA-256 hex = 64 chars

	// hash 输入拼接必须正确（uuid.Nil → ""）
	input := hashInputForTest(nil, createdAt, rec)
	assert.Contains(t, input, "||")          // 两个空 uuid 之间的管道符
	assert.Contains(t, input, "||")          // prev_hash=nil
	assert.NotContains(t, input, "00000000") // 不会有 uuid.Nil 的 string 表示
}

// TestComputeHash_KnownValue 精确验证固定输入 → 固定输出。
// 如果此测试失败说明 hash 输入字段顺序/编码变了（破坏链可验证性）。
func TestComputeHash_KnownValue(t *testing.T) {
	// 手动计算已知输入的 SHA-256
	parts := []string{
		"prev_hash_val",        // prev_hash
		"2026-10-17T10:30:00Z", // created_at (RFC3339Nano 无纳秒)
		"",                     // actor_id (空)
		"platform_admin",       // actor_type
		"",                     // source_tenant (空)
		"",                     // target_tenant (空)
		"users",                // resource_type
		"read",                 // action
		"low",                  // risk_level
		"allow",                // decision
		"ok",                   // reason
	}
	joined := strings.Join(parts, "|")
	h := sha256.Sum256([]byte(joined))
	expected := hex.EncodeToString(h[:])

	createdAt, _ := time.Parse(time.RFC3339, "2026-10-17T10:30:00Z")
	prevHash := "prev_hash_val"
	rec := AuditRecord{
		ActorType:    PrincipalPlatformAdmin,
		ResourceType: "users",
		Action:       "read",
		RiskLevel:    RiskLow,
		Decision:     "allow",
		Reason:       "ok",
	}

	actual := computeHash(&prevHash, createdAt, rec)
	assert.Equal(t, expected, actual,
		"known input must produce known hash — if it fails, hash input format changed")
}

// TestValidateChain_Valid 验证正确哈希链全部通过。
func TestValidateChain_Valid(t *testing.T) {
	createdAt := time.Date(2026, 10, 17, 10, 30, 0, 0, time.UTC).Truncate(time.Microsecond)

	// 3 条记录：链头 prev_hash=NULL
	rec1 := chainRecord{CreatedAt: createdAt, ActorType: "platform_admin", ResourceType: "users", Action: "read", RiskLevel: "low", Decision: "allow", Reason: "r1"}
	hash1 := computeHash(nil, createdAt, AuditRecord{ActorType: PrincipalPlatformAdmin, ResourceType: "users", Action: "read", RiskLevel: RiskLow, Decision: "allow", Reason: "r1"})
	rec1.Hash = hash1
	// rec1.PrevHash 保持 nil

	rec2 := chainRecord{CreatedAt: createdAt.Add(time.Second), ActorType: "platform_admin", ResourceType: "applications", Action: "create", RiskLevel: "medium", Decision: "allow", Reason: "r2"}
	prevHash2 := hash1
	hash2 := computeHash(&prevHash2, createdAt.Add(time.Second), AuditRecord{ActorType: PrincipalPlatformAdmin, ResourceType: "applications", Action: "create", RiskLevel: RiskMedium, Decision: "allow", Reason: "r2"})
	rec2.Hash = hash2
	rec2.PrevHash = &prevHash2

	rec3 := chainRecord{CreatedAt: createdAt.Add(2 * time.Second), ActorType: "platform_admin", ResourceType: "roles", Action: "delete", RiskLevel: "high", Decision: "allow", Reason: "r3"}
	prevHash3 := hash2
	hash3 := computeHash(&prevHash3, createdAt.Add(2*time.Second), AuditRecord{ActorType: PrincipalPlatformAdmin, ResourceType: "roles", Action: "delete", RiskLevel: RiskHigh, Decision: "allow", Reason: "r3"})
	rec3.Hash = hash3
	rec3.PrevHash = &prevHash3

	err := validateChain([]chainRecord{rec1, rec2, rec3})
	require.NoError(t, err, "valid chain must pass validation")
}

// TestValidateChain_Tampered 验证篡改检测。
func TestValidateChain_Tampered(t *testing.T) {
	createdAt := time.Date(2026, 10, 17, 10, 30, 0, 0, time.UTC).Truncate(time.Microsecond)

	// 构造有效链然后篡改
	rec1 := chainRecord{CreatedAt: createdAt, ActorType: "platform_admin", ResourceType: "users", Action: "read", RiskLevel: "low", Decision: "allow", Reason: "r1"}
	hash1 := computeHash(nil, createdAt, AuditRecord{ActorType: PrincipalPlatformAdmin, ResourceType: "users", Action: "read", RiskLevel: RiskLow, Decision: "allow", Reason: "r1"})
	rec1.Hash = hash1

	// rec2 有篡改：hash 不匹配内容
	prevHash2 := hash1
	rec2 := chainRecord{
		CreatedAt:    createdAt.Add(time.Second),
		ActorType:    "platform_admin",
		ResourceType: "users",
		Action:       "impersonate", // 篡改：把 read 改成 impersonate
		RiskLevel:    "critical",
		Decision:     "allow",
		Reason:       "hacked",
		PrevHash:     &prevHash2,
		Hash:         "0000000000000000000000000000000000000000000000000000000000000000", // 篡改 hash
	}

	err := validateChain([]chainRecord{rec1, rec2})
	require.Error(t, err, "tampered chain must fail validation")
	assert.Contains(t, err.Error(), "hash mismatch")
}

// TestValidateChain_BrokenLink 验证链断裂检测（中间 prev_hash 不指向前一条 hash）。
func TestValidateChain_BrokenLink(t *testing.T) {
	createdAt := time.Date(2026, 10, 17, 10, 30, 0, 0, time.UTC).Truncate(time.Microsecond)

	rec1 := chainRecord{CreatedAt: createdAt, ActorType: "platform_admin", ResourceType: "users", Action: "read", RiskLevel: "low", Decision: "allow", Reason: "r1"}
	hash1 := computeHash(nil, createdAt, AuditRecord{ActorType: PrincipalPlatformAdmin, ResourceType: "users", Action: "read", RiskLevel: RiskLow, Decision: "allow", Reason: "r1"})
	rec1.Hash = hash1

	// rec2.prev_hash 被篡改为别的值
	badPrev := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	rec2 := chainRecord{
		CreatedAt:    createdAt.Add(time.Second),
		ActorType:    "platform_admin",
		ResourceType: "users",
		Action:       "read",
		RiskLevel:    "low",
		Decision:     "allow",
		Reason:       "r2",
		PrevHash:     &badPrev, // 断裂！
	}
	// hash 按 badPrev 正确计算（验证函数仍会检测断裂）
	hash2 := computeHash(&badPrev, createdAt.Add(time.Second), AuditRecord{ActorType: PrincipalPlatformAdmin, ResourceType: "users", Action: "read", RiskLevel: RiskLow, Decision: "allow", Reason: "r2"})
	rec2.Hash = hash2

	err := validateChain([]chainRecord{rec1, rec2})
	require.Error(t, err, "broken link must fail validation")
	assert.Contains(t, err.Error(), "expected prev_hash=")
}

// TestValidateChain_SingleRecord 验证链头（prev_hash=NULL）单独有效。
func TestValidateChain_SingleRecord(t *testing.T) {
	createdAt := time.Date(2026, 10, 17, 10, 30, 0, 0, time.UTC).Truncate(time.Microsecond)

	rec := chainRecord{
		CreatedAt:    createdAt,
		ActorType:    "platform_admin",
		ResourceType: "users",
		Action:       "read",
		RiskLevel:    "low",
		Decision:     "allow",
		Reason:       "first record",
		// PrevHash 为空（链首记录）
	}
	rec.Hash = computeHash(nil, createdAt, AuditRecord{ActorType: PrincipalPlatformAdmin, ResourceType: "users", Action: "read", RiskLevel: RiskLow, Decision: "allow", Reason: "first record"})

	err := validateChain([]chainRecord{rec})
	require.NoError(t, err, "single record with prev_hash=nil must be valid")
}

// TestValidateChain_Empty 验证空链。
func TestValidateChain_Empty(t *testing.T) {
	err := validateChain(nil)
	assert.NoError(t, err)

	err = validateChain([]chainRecord{})
	assert.NoError(t, err)
}

// TestValidateChain_ChainHeadMustBeNil 验证链头不能有 prev_hash。
func TestValidateChain_ChainHeadMustBeNil(t *testing.T) {
	prev := "somehash"
	rec := chainRecord{
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
		ActorType:    "user",
		ResourceType: "users",
		Action:       "read",
		RiskLevel:    "low",
		Decision:     "allow",
		PrevHash:     &prev, // 链头不该有 prev_hash
	}
	rec.Hash = computeHash(&prev, rec.CreatedAt, AuditRecord{ActorType: PrincipalUser, ResourceType: "users", Action: "read", RiskLevel: RiskLow, Decision: "allow"})

	err := validateChain([]chainRecord{rec})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prev_hash=nil")
}
