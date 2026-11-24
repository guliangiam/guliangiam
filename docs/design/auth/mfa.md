# MFA Challenge 状态机

> 设计锚点：[DESIGN:auth/mfa]

> 目标态规范「MFA Challenge 状态机」；写作约束见 [docs/README.md](../../README.md)。

---

## 状态机

MFA Challenge 领域常量定义于 `internal/domain/mfachallenges/challenge.go`，DDL 列 `login_challenges.status VARCHAR(64) NOT NULL DEFAULT 'pending'`（配套 `ck_login_challenges_status` CHECK 约束与 COMMENT 列举同 4 态；结构事实源 `contracts/schema/40_session.yaml`，复现：`grep -n 'ck_login_challenges_status' contracts/schema/40_session.yaml`）。

```
                  +-----------+
                  |  pending  |  密码校验通过后创建（TTL 10min）
                  +-----+-----+
                        |
           +------------+------------+
           |                         |
   +-------v-------+        +-------v--------+
   | push_pending  |        |   approved     |  TOTP/SMS/Email/WebAuthn 验证通过
   | (Push等待确认) |        +----------------+
   +-------+-------+
           |
   +-------v--------+
   |   rejected     |  用户拒绝
   +----------------+
```

## 状态转换规则

| 转换 | 触发条件 |
|------|---------|
| `pending → approved` | 用户完成 TOTP/SMS/Email/WebAuthn 验证 |
| `pending → push_pending` | Push MFA 等待移动端确认 |
| `push_pending → approved` | 移动端确认 |
| `push_pending → rejected` | 移动端拒绝 |
| 逆向转换 | **不允许** |

## 生命周期参数

| 参数 | 规范值 | 代码位置 |
|------|--------|---------|
| Challenge TTL | **10 分钟**（`mfaChallengeTTL`，创建时 `expires_at = now+TTL`） | `internal/app/auth/login.go` 的 `mfaChallengeTTL`；复现：`grep -n 'mfaChallengeTTL' internal/app/auth/login.go` |
| 最大验证失败次数 | **5 次**（`maxMfaAttempts`），超限**物理删除 challenge 行**而非置态 | `internal/app/auth/login_mfa.go` 的 `maxMfaAttempts`；复现：`grep -n 'maxMfaAttempts' internal/app/auth/login_mfa.go` |
| 过期处理 | `expiresAt` 已过 → **物理删除** challenge 行；批准/拒绝成功后同样删除 | — |

**终态表达（目标态）**：「验证成功被消费」「超时过期」须以置态表达——领域常量与 DDL COMMENT 同步扩为 6 态（补 `consumed`/`expired`），终结时置态并写 `resolved_at`，DDL 约束 `ck_login_challenges_status` 同步扩集合；超限失败仍物理删除 challenge 行。审计以 `records` 侧事件与终态列共同还原。

**注意**：`login_challenges.status` 与 `sessions.session_state` 是两张表的两个独立列，引用时不得混用。


## 恢复码（Recovery Code）

| 项 | 规范 |
|----|------|
| 存储 | **仅存哈希**（Argon2id）；明文只在生成响应中出现一次，禁止以明文 JSONB 落库 |
| 生成 | 一次性生成一组高熵随机码，互不相同；生成新组即作废旧组 |
| 消费 | 原子条件更新（未用 → 已用）；同一码重复提交一律拒绝并触发安全通知 |
| 耗尽 | 全部用尽后强制进入重新生成流程，未重新生成前不得再以恢复码登录 |
| 审计 | `mfa.recovery_code.used` / `mfa.recovery_code.exhausted`；只记 actor、来源 IP、结果，不记码值 |

结构承载：`user_mfa_config.recovery_code_hashes`（`TEXT[]`，Argon2id 哈希）与 `recovery_generated_at` 已随声明式 Schema 落地；**明文恢复码列不存在**。

## TOTP 失步与重同步

重同步是 **challenge 内自动门控**，不是独立端点或登录页入口（实现 `internal/app/auth/login_mfa.go`，复现：`grep -n 'totpResyncRequired\|totpResyncAfterFailures\|emitTotpResyncAudit' internal/app/auth/login_mfa.go`）：

1. 验证窗口为前后各 1 个时间步（30s，`internal/platform/mfatotp/mfatotp.go` 的 `MatchStep`）；窗口外的码一律拒绝，不因连续失败而放宽窗口。同一时间步的码重复提交按重放拒绝（`mfatotp.Replayed` 对照 `last_used_at`）。
2. challenge 累计失败满 **3 次**（`totpResyncAfterFailures`）后进入受控重同步门控：只接受**晚于最后一次失败时间步**的码（`totpResyncRequired`），强制客户端时钟/窗口前进，而不是重放同一个漂移码。
3. 进入门控时写 **`mfa.totp.resync_required`** 审计（`emitTotpResyncAudit`）；该事件在登录事务回滚后由 `persistMfaChallengeFailure` 的独立事务提交（事务内嵌入会随回滚丢弃）。payload 只含 challengeId/methodType，不含码值与密钥。事件登记以 `contracts/registries/events.yaml` 为唯一事实源。
4. 验证成功更新 `user_mfa_config` 对应方法的 `last_used_at`；失败计入 challenge 失败次数（上限见 §生命周期参数）。
5. 管理员「重绑 TOTP」须重新走注册 ceremony，并写 `mfa.credential.rebound` 审计。
6. 页面入口与交互以 `docs/design/frontend/registry.md` 与 `design/frontend/interaction.md` 为准。

## 协议域声明

完整 OAuth2/OIDC 协议扩展见《design/protocol/oauth2.md》。
