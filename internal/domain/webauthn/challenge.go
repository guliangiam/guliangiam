package webauthn

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Ceremony 类型：注册与认证共用一张 webauthn_challenges 表，但 finish 时不得互换
// （拿认证挑战去完成注册就是跨 ceremony 重放）。
const (
	CeremonyTypeRegistration   = "registration"
	CeremonyTypeAuthentication = "authentication"
)

// Challenge 是一次 WebAuthn ceremony 在服务端的挑战记录（《开发设计》31.4）。
//
// 明文挑战不落库：行里存的是 SHA-256（hex）。finish 用客户端回传的 challenge 重算哈希
// 与本行比对，因此即使数据库被读走，也无法据此伪造一次合法断言。
//
// 挑战存在表里而不是进程内存：begin/finish 是两个请求，多副本部署时会落到不同进程，
// 进程内 map 必然失败；表化后天然共享，重启也不丢（一次性由条件 UPDATE 保证）。
type Challenge struct {
	ID             string
	ChallengeHash  string
	CeremonyType   string
	UserID         string
	OrganizationID string
	RelyingPartyID string
	ExpiresAt      time.Time
	ConsumedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// HashChallenge 返回挑战明文的 SHA-256 十六进制摘要——存库与比对的唯一口径。
func HashChallenge(challenge string) string {
	sum := sha256.Sum256([]byte(challenge))
	return hex.EncodeToString(sum[:])
}

// Consumed 报告该挑战是否已被消费（一次性）。
func (c *Challenge) Consumed() bool { return c.ConsumedAt != nil }

// Expired 报告在给定时刻该挑战是否已过期。
func (c *Challenge) Expired(now time.Time) bool { return now.After(c.ExpiresAt) }

// MatchesChallenge 判断客户端回传的挑战明文是否就是本行签发的那一个。
func (c *Challenge) MatchesChallenge(challenge string) bool {
	return c.ChallengeHash != "" && c.ChallengeHash == HashChallenge(challenge)
}
