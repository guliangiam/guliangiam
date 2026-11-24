package webauthn

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// 挑战记录的纯函数语义：哈希口径、一次性判定、过期判定。

func TestHashChallenge_IsStableSHA256(t *testing.T) {
	// 固定向量：sha256("challenge-001")——哈希口径一旦变化，存量挑战行全部失配。
	assert.Equal(t, "a3dfdc18751f92616149d87f098b82c852be2bb92e8c4fe442b3af4804c0d575", HashChallenge("challenge-001"))
	assert.Len(t, HashChallenge("challenge-001"), 64)
	assert.NotEqual(t, HashChallenge("challenge-001"), HashChallenge("challenge-002"))
}

func TestChallenge_MatchesChallenge(t *testing.T) {
	challenge := "server-issued-challenge-001"
	stored := &Challenge{ChallengeHash: HashChallenge(challenge)}

	assert.True(t, stored.MatchesChallenge(challenge), "同一明文必须匹配")
	assert.False(t, stored.MatchesChallenge("another-challenge"), "换一个明文必须不匹配")
	assert.False(t, (&Challenge{}).MatchesChallenge(""), "空哈希不得被空明文匹配")
}

func TestChallenge_Expired(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	fresh := &Challenge{ExpiresAt: now.Add(time.Minute)}
	stale := &Challenge{ExpiresAt: now.Add(-time.Second)}

	assert.False(t, fresh.Expired(now))
	assert.True(t, stale.Expired(now))
}

func TestChallenge_Consumed(t *testing.T) {
	consumedAt := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	assert.False(t, (&Challenge{}).Consumed(), "consumed_at 为空即待消费")
	assert.True(t, (&Challenge{ConsumedAt: &consumedAt}).Consumed())
}
