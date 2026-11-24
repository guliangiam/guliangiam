# ADR-002: PostgreSQL 作为身份事实源

- 状态：[Current]
- 日期：2026-09-14

## 背景

IAM 的核心数据——用户、组织、应用、策略、会话、令牌以及一次性认证状态（授权码、Nonce、Ticket、MFA challenge）——既要支持强一致写入，又要可审计、可备份恢复。若让缓存、会话存储或外部目录各自成为一部分事实的权威源，丢数、漂移与审计断链会直接转化为认证事故。项目初期即须明确“哪个存储说了算”，其余存储只能派生。

## 决策

用户、组织、应用、策略、会话、令牌和一次性认证状态的关键事实以 PostgreSQL 为唯一权威；缓存不可替代事务和审计。结构事实源为 `contracts/schema/*.yaml`（声明式 Schema，见 ADR-022），访问路径为 pgx 连接池（见 ADR-006）。

## 理由与后果

- 正面：单库 ACID 事务覆盖核心写入；备份、恢复、审计链统一；一次性状态可用条件更新原子消费；多租户隔离还可叠加 RLS 兜底层（ADR-016）。
- 负面：必须持续治理连接池、备份恢复、索引与结构演进（ADR-022）；PostgreSQL 成为关键路径依赖，需要 HA 与容量规划（ADR-011、ADR-020）；Redis 等其他存储的角色被严格限定为可重建缓存或限流状态（ADR-010、ADR-019），不得越界承载事实。

## 替代方案

- **多存储并列权威（文档/关系入 PostgreSQL、会话与令牌入 Redis、画像入文档库）**：放弃理由——跨存储没有事务，注册/注销/撤销等动作无法原子收敛，备份恢复与审计必须跨系统拼接，运维与故障面成倍扩大。
- **以外部 IdP / 企业目录为事实源、本系统仅做镜像**：放弃理由——产品定位即自建多租户 IAM，协议面与管理面需要自有事实才能签发令牌、承载租户策略；外部 IdP 只作为登录联邦入口（Identity Provider）存在，不充当权威库。

## 验证证据（Current）

- `grep -E 'jackc/pgx|go-sql-driver/mysql|redis/go-redis' go.mod` → 数据库驱动：PostgreSQL 用 `github.com/jackc/pgx/v5`（默认基线）、MySQL 用 `github.com/go-sql-driver/mysql`（第二基线，ADR-023）；无 ORM（ADR-006）。`github.com/redis/go-redis/v9` 仅用于缓存/限流等派生状态（ADR-010/ADR-019），测试用 miniredis 替身。
- 核对 `contracts/schema/*.yaml`（声明式 Schema 唯一事实源，见 ADR-022）：表/列/索引数量以 `gen_schema.py` 输出为准（不在文档手写计数），全部位于 `guliangiam` schema；一次性状态落库并以条件更新原子消费，例：`internal/infra/postgres/web3identity/sql.go` 的 `UPDATE ... WHERE nonce = $1 AND consumed = false ... RETURNING`；授权码以 `used_at` + 过期时间判定一次性（见 ADR-004 证据）。
- Redis 的存在不改变「PostgreSQL 为身份事实源」：缓存/限流状态全部可重建，ADR-010/ADR-019 已从纯规划推进为「适配器已实现、部署驱动可选（默认内存，Redis 可选）」的混合状态。

## 复审条件

引入多区域多活（需要重定义事实源与复制拓扑）、事实源外移到外部托管 IdP、或单库容量/可用性触及上限时复审；ADR-023 多方言落地后，“关系数据库为事实源”不变，但“PostgreSQL 唯一”的表述需要按双基线重新界定。
