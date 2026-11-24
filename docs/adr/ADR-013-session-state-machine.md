# ADR-013: 会话状态机与固定防护

- 状态：[Current]（会话状态列与 CHECK、闲置超时（last_seen_at + 组织级 idle_timeout）、会话固定防护、撤销 Outbox、OIDC Back-Channel/Front-Channel Logout、可配置客户端 IP/UA 绑定（变化即 401 重新认证）与跨实例撤销生效均已落地；控制台登出页接入 front-channel 页面为后续 UX 优化，不改变服务端能力状态）
- 日期：2026-09-14

## 背景

会话需要显式的状态迁移与生命周期语义：闲置/绝对双超时、登录后重新生成会话 ID 的固定防护、设备指纹/IP 变化的劫持检测、多设备撤销通道，以及 OIDC Front-Channel/Back-Channel Logout（RFC 7007 风格的 `sid` 关联）。这些都要求会话先成为一个有明确状态迁移的实体。

## 决策

会话从“Cookie + SessionID”升级为显式状态机（`active` / `idle` / `expired` / `revoked`，非法迁移拒绝），并引入三类防护：登录成功后重新生成会话 ID（会话固定防护）、基于设备指纹与 IP 变化的劫持检测、OIDC Front/Back-Channel Logout（携带 `sid`）；撤销必须能实时传播到所有应用实例。

## 理由与后果

- 正面：过期、闲置、撤销语义可审计、可测试；闲置超时与绝对超时分别覆盖“离开未登出”与“最长存活期”两类风险；Back-Channel Logout 让下游 RP 可凭 `sid` 即时清理会话；多设备管理与“下线其他设备”有了数据基础。
- 负面：会话表需要状态列、绝对超时（`expires_at`）与索引；闲置超时由 `last_seen_at` + 组织级 `idle_timeout` 在读取时计算，不落冗余 `idle_expires_at` 列（避免策略调整后留下陈旧 deadline，代价是每次读取多一次策略解析）；撤销的多实例实时传播需要共享失效通道（依赖 ADR-010 Redis 或 PG LISTEN/NOTIFY）；指纹/IP 策略过严会误伤移动网络用户，需要可配置阈值与重新认证而非直接注销。

## 替代方案

- **维持 Cookie + 单条记录、仅靠 TTL 过期（不采用）**：无法防会话固定、无法区分闲置与绝对超时、撤销只能等 TTL 自然到期，也没有 `sid` 支持 Back-Channel Logout。
- **无状态 JWT 会话（不存服务端记录）**：放弃理由——IAM 最核心的“即时注销/强制下线/管理员撤销”在无状态下无法实现，仍需撤销名单，等于把状态以黑名单形式请回来，还丧失了状态机的可审计性。
- **有状态但只做超时、不做指纹/登出通道**：放弃理由——只解决超时问题，会话固定与联邦注销风险仍在；三者同属会话安全升级包，分批实施但设计一次定型。

## 验证证据

**实现证据（`[Current]` 子事实）**：

- `db/postgresql/V1__guliangiam_schema_init.sql` 的 `guliangiam.sessions` 表：`session_state TEXT NOT NULL DEFAULT 'active'`、`expires_at`、`revoked_at`、`last_seen_at`（用于会话恢复/闲置超时/设备管理）与 `CONSTRAINT uq_sessions_sid UNIQUE (sid)`、`ck_sessions_state CHECK (session_state IN ('active','idle','expired','revoked'))`。
- 闲置超时不在库中落 `idle_expires_at` 列：`internal/app/sessionpolicy.ResolvePolicy` 一次解析组织策略，`IsIdleExpired(now, last_seen_at, idle_timeout)` 在读取时判定；登录路径统一执行 S9（撤销请求携带的旧会话，`internal/app/auth/login_flow.go` 等）防会话固定。
- 客户端绑定：`OrganizationSecurityPolicy.SessionPolicy` 的 `bindIp`/`bindUserAgent`（默认 false）开启后，`ResolveSessionContextForClient` 比较会话与请求的 IP/User-Agent；不一致返回 401 要求重新认证（不静默注销），策略解析与比较函数有单元测试。
- 跨实例撤销：认证解析每请求经由事务内仓储读取 `sessions`（DB 为事实源，不经过进程内缓存），撤销写入后其他实例下一次请求即生效；可选缓存装饰器在单条撤销时删除共享缓存键、批量撤销/清理时 `InvalidateAll`，`cached_repository_revocation_test.go` 用两个实例共享同一缓存验证「A 撤销后 B 不再从缓存返回已撤销会话」。
- Front-Channel Logout 已落地：声明式 Schema V3 新增 `app_oauth_config.frontchannel_logout_uri` 与 `frontchannel_logout_session_required`（生成 V3 前滚/回滚迁移，Application DTO/更新/导入导出与双方言仓储同步）；服务端 `BuildFrontChannelLogout` 按用户活跃会话收集 RP URI，`GET /api/v1/auth/frontchannel-logout` 先构建目标、再撤销 OP 会话/清 Cookie 并渲染隐藏 iframe 页（`iss` 必带；`sid` 按应用 `session_required` 携带）；discovery 宣告 `frontchannel_logout_supported`/`frontchannel_logout_session_supported`。控制台登出沿用既有 ssoLogout 隐藏 iframe 流程；OIDC Front-Channel 页面供 OIDC RP 登出链路使用。
- 撤销事件经 `internal/app/sessions/revoke_*.go` 写入 Outbox；OIDC Back-Channel Logout 已实现（`internal/app/auth/backchannel_logout.go` + `POST /api/v1/auth/backchannel-logout`）；认证解析直读 DB，撤销跨实例即时生效；共享缓存失效由 `cached_repository_revocation_test.go` 覆盖。

## 复审条件

会话存储迁移到专用高吞吐存储（超出 PG 会话表承载）时重审；双超时模型无法满足业务分级（如按信任等级给不同绝对寿命）时复审；OIDC 会话管理/注销规范更新；ADR-010 的共享失效通道选型确定后联动复核。
