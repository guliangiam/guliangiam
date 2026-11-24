package crypto

import encryptionkeys "github.com/guliangiam/guliangiam/internal/domain/encryption_keys"

// KeyStatus 是 encryption_keys 表的状态枚举（ADR-015 + 架构计划 Step 4.7）。
//
// 生命周期：
//
//	pending ──(审批+激活)──▶ active ──(轮换开始)──▶ retiring ──(重加密完成)──▶ retired
//	   │                                                                │
//	   └──(紧急撤销)──────────────────────────────────────────────────▶ revoked
//
// retired 与 revoked 的区别：
//   - retired   — 正常退役，密钥还在加密历史数据的重加密过程中，待所有密文升级后可 destroy
//   - revoked   — 紧急撤销（疑似泄露 / 操作事故），立即停止使用，不允许解密新数据
//
// 这两个都是终态（不能回到 active），区别在于 retired 还在服务重加密作业，
// revoked 立即停服。销毁双通道审批（KeyDestructionApprover）只接受 retired
// 状态的密钥 — revoked 必须走安全事件流程。
//
// DB 层 CHECK 约束：
//
//	ck_encryption_keys_status CHECK (status IN ('pending','active','retiring','retired','revoked'))
//
// Domain 层状态机约束更严格：非法迁移显式拒绝。
// KeyStatus 是生成状态机枚举的别名：contracts/registries/state-machines.yaml
// 是唯一事实源，避免平台层手写枚举与生成物漂移（OPEN-API-41）。
type KeyStatus = encryptionkeys.EncryptionKeysState

const (
	KeyStatusPending  KeyStatus = encryptionkeys.EncryptionKeysStatePending_State
	KeyStatusActive   KeyStatus = encryptionkeys.EncryptionKeysStateActive_State
	KeyStatusRetiring KeyStatus = encryptionkeys.EncryptionKeysStateRetiring_State
	KeyStatusRetired  KeyStatus = encryptionkeys.EncryptionKeysStateRetired_State
	KeyStatusRevoked  KeyStatus = encryptionkeys.EncryptionKeysStateRevoked_State
)

// AllKeyStates 列出所有合法状态（schema CHECK 对齐）。
var AllKeyStates = encryptionkeys.AllEncryptionKeysState

// IsValidKeyStatus 报告 s 是否属于已知状态集合。
// 匹配 DB CHECK 约束，提前在 Domain 层拦截非法值。
func IsValidKeyStatus(s string) bool {
	switch KeyStatus(s) {
	case KeyStatusPending, KeyStatusActive, KeyStatusRetiring,
		KeyStatusRetired, KeyStatusRevoked:
		return true
	case "":
		return false
	default:
		return false
	}
}

// IsTerminalKeyStatus 报告状态是否终态（retired / revoked）。
// 终态密钥不再参与正常加密流程，只能被安全销毁。
func IsTerminalKeyStatus(s string) bool {
	st := KeyStatus(s)
	return st == KeyStatusRetired || st == KeyStatusRevoked
}

// validKeyTransitions 是显式白名单迁移表。任何不在表中的 from→to 对都是非法。
//
// 设计原则：
//   - pending → active 必须走审批流程（Admin API 不允许裸激活）—— TryTransition
//     只管状态合法性，审批是 app 层职责，但状态机确保"未审批不能直接从 pending 跳 retired"。
//   - retiring 是过渡态：只能来自 active，终止于 retired 或 revoked。
//   - revoked 是紧急出口：从 pending / active / retiring 都能进（安全事件不讲究流程）。
//   - retired 是正常退役：只能从 retiring 进（active 不能直接 retired，必须先 retiring 走完重加密）。
//   - 终态无出边：retired / revoked 不能回到 active 或其他状态。
var validKeyTransitions = map[KeyStatus]map[KeyStatus]bool{
	KeyStatusPending: {
		KeyStatusActive:  true, // 审批通过 → 激活
		KeyStatusRevoked: true, // 激活前发现问题 → 撤销
	},
	KeyStatusActive: {
		KeyStatusRetiring: true, // 轮换计划启动
		KeyStatusRevoked:  true, // 紧急撤销（泄露）
	},
	KeyStatusRetiring: {
		KeyStatusRetired: true, // 重加密完成 → 正常退役
		KeyStatusRevoked: true, // 轮换中发现问题 → 紧急撤销
	},
	// KeyStatusRetired: 空 map → 终态，只能被 KeyDestructionApprover destroy
	// KeyStatusRevoked: 空 map → 终态
}

// TryKeyTransition 尝试把密钥状态从 from 迁移到 to。
//
// 返回 true 表示迁移合法；false 表示非法且调用方必须拒绝。
// 不 panic，纯函数，不依赖 encryption_keys Repository。
func TryKeyTransition(from, to KeyStatus) bool {
	if !IsValidKeyStatus(string(from)) || !IsValidKeyStatus(string(to)) {
		return false
	}
	next, ok := validKeyTransitions[from]
	if !ok {
		return false // from 是终态或非法
	}
	return next[to]
}
