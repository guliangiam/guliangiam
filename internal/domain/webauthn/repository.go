package webauthn

import (
	"context"
	"time"
)

// Repository defines the persistence interface for WebAuthn credentials and
// ceremony challenges (both live in the webauthn domain; see 《开发设计》31.4).
type Repository interface {
	Create(ctx context.Context, credential *Credential) error
	GetByCredentialID(ctx context.Context, credentialID []byte) (*Credential, error)
	ListByUserID(ctx context.Context, userID string) ([]*Credential, error)
	UpdateSignCount(ctx context.Context, id string, signCount int64) error
	// Update 持久化凭证的可变元数据（显示名与 updated_at）。状态与签名计数走各自的
	// 专用方法，避免绕过状态机与单调计数保护。
	Update(ctx context.Context, credential *Credential) error
	// UpdateStatus 迁移凭证生命周期状态（active/revoked/deactivated）。克隆嫌疑、
	// 管理员吊销与用户停用都走这条路径——状态是留痕，删行不是。
	UpdateStatus(ctx context.Context, id string, status CredentialStatus) error
	Delete(ctx context.Context, id string) error

	// CreateChallenge 落一条待消费的挑战（只存哈希，不存明文）。
	CreateChallenge(ctx context.Context, challenge *Challenge) error
	// ConsumeChallenge 原子消费：仅当该行存在、未消费且未过期时置 consumed_at 并返回它；
	// 其余情况（不存在 / 已消费 / 已过期）一律返回 NotFound。一次性保证来自条件 UPDATE 的
	// RowsAffected=0（或 RETURNING 无行），不是「先查后写」——后者在并发下会双放行。
	ConsumeChallenge(ctx context.Context, id string, now time.Time) (*Challenge, error)
	// DeleteExpiredChallenges 批量删除 expires_at 早于 cutoff 的挑战行，返回删除条数。
	// 过期行不影响正确性（finish 的消费谓词自带 expires_at > now()），只占空间，
	// 因此由周期性清理作业调用，而不是在请求路径上顺带删。
	DeleteExpiredChallenges(ctx context.Context, cutoff time.Time) (int64, error)
}
