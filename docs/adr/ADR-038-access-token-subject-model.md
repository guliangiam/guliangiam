# ADR-038: access_tokens 的主体模型（Principal Model）

- 状态：[Target] — 决策方向已定；落地路径步骤 1 已完成：`client_credentials` 两个主体分支与 `jwt-bearer` 均已在认证后、写入前显式返回 400 `unsupported_grant_type`（`internal/app/auth/client_credentials.go` 的 `ExchangeClientCredentials` 与 `internal/app/auth/jwt_bearer_token.go` 的 `GetJwtBearerToken` 封堵分支；jwt-bearer 的 refresh 签发分支已删除，R1 违反关闭）；Schema 与其余代码未按本 ADR 改造；出口条件见「验证证据」
- 日期：2026-11-06
- 影响面：`contracts/schema/40_session.yaml`、`contracts/schema/60_audit.yaml`、`internal/domain/tokens`、`internal/app/auth`、`internal/app/tokens`、`internal/app/workload`、`internal/infra/{postgres,mysql}/tokens`、`internal/infra/postgres/{dashboard,records}`、`internal/infra/jwt`

## 背景

### 当前模型

以下主体模型描述以声明层与 token 读写路径为准（核验 2026-11-21 仍成立）；`client_credentials` 与 `jwt-bearer` 签发分支的行级证据为决策时（2026-11-06）快照，两个 grant 现均已被封堵（见下「五类签发路径的主体」后的现状说明）。

`access_tokens` 把 OAuth 的 `sub` 与 IAM 的 `users` 实体等同，租户归属也寄生在 subject 上：

- `user_id UUID NOT NULL`，`FK → guliangiam.users(id) ON DELETE CASCADE`（`contracts/schema/40_session.yaml:156-159`、`:275-277`）
- RLS 谓词：
  `guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.users u WHERE u.id = user_id AND u.organization_id = guliangiam.current_organization_id())`
  （声明层：`contracts/schema/40_session.yaml:269-270`；生成 DDL 定位：`grep -n 'CREATE TABLE IF NOT EXISTS guliangiam.access_tokens' db/postgresql/V1__guliangiam_schema_init.sql` 及其后的 POLICY 段）
- token 读路径统一靠 `JOIN guliangiam.users` 推组织：`internal/infra/postgres/tokens/sql.go:17-213`（`:17`、`:46`、`:73`、`:99`、`:125`、`:150`、`:182` 为读查询，`:202-213` 为 `UPDATE ... FROM users`）
- 领域校验只认 `UserID`：`internal/domain/tokens/token.go:42-45` 在 `Validate()` 中要求 `userId is required`

### 五类签发路径的主体（`client_credentials` 含两个主体分支）

| 流程 | RFC 语义中的 `sub` | 当前模型能否表达 | 证据 |
|---|---|---|---|
| `authorization_code` | 终端用户 | 成立 | `internal/app/auth/token_exchange.go:128-169`（先 `Users().GetByID`，再写 `UserID: user.ID`） |
| `client_credentials`（application） | 客户端自己 | 不能：`UserID` 为空，`$2::uuid` 解析失败 | `internal/app/auth/client_credentials.go:199-206`、`internal/infra/postgres/tokens/sql.go:4-5` |
| `client_credentials`（service account） | 客户端自己 | 不能：SA id 不在 `users`，外键失败 | `internal/app/auth/client_credentials.go:184-198`、`contracts/schema/40_session.yaml:275-277` |
| `token_exchange`（RFC 8693） | `subject_token` 的 sub，另有 `act` 链 | 部分：只有 `subjectToken.UserID`，`act` 只存在于 `metadata` | `internal/app/auth/rfc8693_token_exchange.go:231-258`、`:415-430` |
| JWT bearer（RFC 7523） | 客户端自己 | 不能：字面空 `UserID` | `internal/app/auth/jwt_bearer_token.go:96-118` |
| workload / federation | 外部工作负载标识 | 不能：合成内存 Token，`UserID: sub`（如 `fed:<sub>`），从不落库 | `internal/app/auth/rfc8693_federation.go:189-223` |

`client_credentials` 的两个分支若直接写入，在真实库上的失败是确定性的（以下为决策时的失败机理与行级快照；现行代码已重构为「认证解析 principal → 写入前封堵」，不再尝试落库）：

- application 分支不设置 `UserID`，`$2::uuid` 收到空串 → `22P02`（决策时 `internal/app/auth/client_credentials.go:199-206`）。
- service account 分支把 `service_accounts.id` 写入 `UserID`，外键指向 `users(id)` → `23503`（决策时 `:184-198`）。
- `ApplicationID` 只有 service account 分支没有设置，因此空 `$3::uuid` 只发生在 service account 分支。
- `jwt-bearer` 决策时同样写入空 `UserID`（`internal/app/auth/jwt_bearer_token.go`，`UserID: ""` 的 access 与 refresh 构造）并签发 refresh token；该路径现已封堵，refresh 签发分支已删除（见下）。

现行封堵事实（核验 2026-11-21）：`ExchangeClientCredentials` 在 `authenticateClientCredentials` 解析出 application/service account 主体后，统一返回 400 `unsupported_grant_type`（"token subjects are limited to users"），认证失败仍返回 401 `invalid_client`；不再构造 token、不再写 `metadata["principalType"]`。`GetJwtBearerToken` 镜像同一模式：断言通过客户端认证（含 jti 消费提交，重放保护保留）后统一返回 400 `unsupported_grant_type`，认证失败仍 401；access/refresh 构造与签发代码已整体删除。决策时「代码把主体类型放在 `metadata`」的事实已随封堵重构消失；「多态主体」由本 ADR 提升为正式列的决策不变。

### 门禁为什么没有暴露该缺陷

`client_credentials` 的测试覆盖全部在 fake 仓储上，没有一层在真实库执行写入：

1. **单测全链路 fake**：`internal/app/auth/wli_auth_test.go:91-120` 用 `authWLITestUOW` 注入 fake 的 workload / token / records / outbox 仓储；`GetAccountByClientID` 在内存切片匹配（`:30-37`）；`Create` 只写 `uow.tokenRepo.tokens`，不经过 `internal/infra/postgres/tokens/sql.go`。
2. **断言建立在错误模型上**：`wli_auth_test.go:180` 断言 `records.created[0].ActorID == account.ID`（SA id）。真实 `records.actor_id` 外键指向 `users`（`contracts/schema/60_audit.yaml:173-175`），该断言在真实库同样不成立，fake 使其通过。
3. **没有真实库用例**：提及 `client_credentials` 的测试文件均为 app/transport 层单测；仓内没有针对该 grant 的隔离库集成测试或 E2E 用例（外部流水线是否覆盖待核实）。
4. **门禁职责不重叠**：`go test` 跑 fake；`check-migrations.sh` / `check-schema-parity.py` 只比对声明与生成物；`check-openapi-contract.sh` 只校验路由声明；`contracts/registries/known-red.yaml` 未登记该缺陷。没有门禁会把「插入违反外键」暴露出来。

把缺口归因于「service account 创建路径不存在」不成立：创建链路完整接线，`internal/transport/http/api/v1/identitypools/routes.go:93-103` → `chi.go:122-135` → `handler.go:65-67` → `internal/app/workload/service.go:410-449` → `internal/infra/postgres/workload/repository.go:177` + `sql.go:18`，且 `internal/bootstrap/chi.go:45` 已注入该 transport。真正缺口是「SA 能创建，但令牌写不进 `access_tokens`」：`user_id` 的外键指向 `users`，而 SA id 不属于该表。

## 决策

### D1. 显式租户 + 多态主体

`access_tokens` 改为「显式租户列 + 多态主体列」，不保留 `user_id`：

```
access_tokens
  id                    UUID PK
  organization_id       UUID NOT NULL        -- 签发时确定，RLS 唯一锚点
  principal_type        VARCHAR(32) NOT NULL -- user | service_account | client | workload
  principal_id          UUID NOT NULL        -- 指向对应主体表的内部主键
  actor_type            VARCHAR(32) NULL     -- token_exchange 的第一层委托
  actor_id              UUID NULL
  application_id        UUID NULL            -- credential-only 主体可为空
  issued_via_client_id  TEXT NOT NULL        -- 本次实际认证使用的 OAuth client_id
  scopes                TEXT[] NOT NULL DEFAULT '{}'
  audience              TEXT[] NOT NULL DEFAULT '{}'
  token_hash            TEXT NOT NULL
  token_prefix          TEXT NOT NULL
  metadata              JSONB NOT NULL DEFAULT '{}'
  expires_at            TIMESTAMPTZ NOT NULL
  revoked_at            TIMESTAMPTZ NULL
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
  revision              BIGINT NOT NULL DEFAULT 1
```

约束与索引：

- `CHECK (principal_type IN ('user','service_account','client','workload'))`
- `CHECK ((actor_type IS NULL AND actor_id IS NULL) OR (actor_type IS NOT NULL AND actor_id IS NOT NULL))`
- `UNIQUE (token_hash)`
- `(organization_id, principal_type, principal_id, expires_at)`：按主体查活跃令牌
- `(organization_id, expires_at)`、`(organization_id, application_id, created_at DESC)`：管理面列表
- `(expires_at) WHERE revoked_at IS NULL`：清理任务
- `(organization_id, created_at DESC)`：分页列表

不变量 P1：`principal_id` 所指向行的 `organization_id` 必须等于 `access_tokens.organization_id`。该不变量在签发路径一次索引命中裁决，不落数据库约束（跨表 CHECK 不可行，主体也可能在签发后迁移）。

不变量 P2：`principal_type='user'` 时，`principal_id` 必须存在于 `users(id)` 且 `users.deleted_at IS NULL`。

### D2. organization_id 作为 RLS 唯一锚点

四种流程的租户来源不同（用户所在组织、client 所属组织、`subject_token` 的组织、federation trust 所属组织），从 subject 反推只在 user 场景成立。RLS 谓词改为：

```sql
USING (
  guliangiam.is_super_admin()
  OR organization_id = guliangiam.current_organization_id()
)
```

收益：

- 读查询不再 `JOIN users`，租户归属与主体存亡解耦（用户软删、client 禁用不影响已签发令牌的租户判定）。
- `token_exchange` 跨租户时，`subject_token.organization_id = 目标 organization_id` 是一次列比较，不依赖 join 正确性。
- 索引直达 `(organization_id, ...)`。

写入路径已有可复用的租户解析：`internal/app/auth/client_credentials.go` 的 `clientCredentialsPrincipal.organizationID()` 从主体（application/service account）解析租户；`internal/domain/tokens/token.go` 的 `Token.OrganizationID` 已有字段，但未落到 `access_tokens` 列。

### D3. 放弃单表外键与三层补偿

`principal_id` 不设数据库外键，`application_id` 允许为空。理由：单一外键只能覆盖一类主体；四类主体意味着四列可空外键加互斥 CHECK，新增类型即改表。多态引用在仓内已有先例：`records.resource_id` 是多态引用（`contracts/schema/60_audit.yaml:26-30`），无外键。主体表存在软删/禁用（`applications.deleted_at`：`contracts/schema/20_identity.yaml:766-768`；`service_accounts.status`：`contracts/schema/85_workload.yaml:65`），外键的 `CASCADE` 会静默删除审计关联，`SET NULL` 又破坏 `principal_id NOT NULL`。

补偿三层，缺一不可：

1. **签发前校验**：按 `principal_type` 路由到对应表做一次主键命中，并比对 `organization_id`（P1）。失败返回协议错误，不产生令牌。
2. **审计同构**：每次签发写 `records`，`actor_type/actor_id` 与 token 的主体同源（D8）。篡改令牌行必须同时篡改审计链才能自洽，而 `records` 有哈希链（`internal/platform/outbox/providers/audit/chain.go:145-196`）。
3. **孤儿令牌巡检**：按 `principal_type` 分组做反连接，输出 `orphaned_tokens_total{principal_type=...}`；发现即吊销并写 `token.orphaned` 审计。巡检与显示层共用 D4 的只读视图。

### D4. 只读视图 v_principals

不建实体主体注册表。用视图聚合主体事实，供显示层与巡检共用：

```sql
CREATE VIEW guliangiam.v_principals AS
SELECT 'user'::text AS principal_type, u.id AS principal_id, u.organization_id,
       u.username AS display_name, u.status
FROM guliangiam.users u
WHERE u.deleted_at IS NULL
UNION ALL
SELECT 'service_account', sa.id, sa.organization_id, sa.name, sa.status
FROM guliangiam.service_accounts sa
UNION ALL
SELECT 'client', a.id, a.organization_id, COALESCE(NULLIF(a.display_name,''), a.slug), a.status
FROM guliangiam.applications a
WHERE a.deleted_at IS NULL
UNION ALL
SELECT 'workload', w.id, w.organization_id, w.display_name, w.status
FROM guliangiam.workload_identities w;
```

边界：`workload_identities` 目前不存在，`contracts/schema/85_workload.yaml:5-52` 只有 `identity_pools` / `federation_trusts` / `service_accounts` 三张表；federation 主体是内存合成 Token（`internal/app/auth/rfc8693_federation.go:189-223`）。因此首版视图只 UNION 三张真实存在的表，`workload` 分支以 `WHERE false` 占位；落地 `workload_identities` 后去掉占位（见开放问题 Q3）。

视图只授予 `SELECT`。RLS 视图需 `security_invoker=on`（PG 15+）或显式 `organization_id = current_organization_id()` 过滤，否则视图会绕过基表 RLS。`scripts/gen_schema.py` 是否支持声明视图待核实；不支持时以手写迁移引入并登记生成器缺口。

### D5. sub 编码规则

表内一律内部 UUID：`principal_id` 永远是对应主体表的主键，不混存外部字符串。

外部 `sub` 通过单点函数转换，规则与 `docs/design/protocol/oauth2.md` 的客户端模式语义对齐：

| `principal_type` | 外部 `sub` | 附加 claim |
|---|---|---|
| `user` | `users.id`（UUID 字符串；现状即如此，`internal/app/auth/handle_logged_in.go:243`） | 无 |
| `service_account` | `client:{service_accounts.client_id}` | `principal_type=service_account`、`client_id=<sa.client_id>` |
| `client` | `client:{applications.client_id}` | `principal_type=client`、`client_id=<app.client_id>` |
| `workload` | 外部 IdP 的 `sub` 原文（现状 `internal/app/auth/rfc8693_federation.go:217` 把外部 sub 放进 `UserID`） | `principal_type=workload`、`identity_pool_id`、`iss` |

`client:{client_id}` 与 `docs/design/protocol/oauth2.md` §14.7「无用户主体：sub/用户名为 `client:{client_id}`」一致；它与 RFC 9068 §2.2 的裸 `client_id` 不同，采用前缀是为了避免与用户 UUID 形状冲突，并与 `client_id` 的两个命名空间问题（D9）解耦。

转换函数只允许一处：新建 `internal/domain/tokens/subject.go`（待建），导出 `ExternalSubject(principalType, principalID, externalClientID string) (string, error)`。调用点限定：

- `internal/infra/jwt/issuer.go:42-60`（claim 组装；`claims.go:1-13` 只是类型别名，不存在 `platform/jwt/claims.go`）；
- `internal/app/tokens/introspect_token.go:53-63` 与 `:87-97`（内省 `sub`）；
- `internal/app/auth/handle_logged_in.go:243`（ID token 的 `sub`，user 场景语义不变）。

禁止把 `client_id` 字符串写进 `principal_id`，也禁止把内部 UUID 直接当客户端可见 `sub`。

### D6. actor 只存第一层委托

`token_exchange` 的 `act` 当前只存在于 `metadata["actor"]`（`internal/app/auth/rfc8693_token_exchange.go:239`、`tokenExchangeActor.actChainMetadata()` `:415-430`）。目标：`actor_type/actor_id` 存第一层，便于 SQL 审计查询；更深层链按 RFC 8693 嵌套在 JWT `act.act...`，已有 `maxActChainDepth` 保护（`:425-427`）。`actor_id` 同样为内部 UUID，外部化由 D5 的同一函数负责。

### D7. principal 与 OAuth client 的区分

- `application_id` / `issued_via_client_id` 表示本次认证使用的 OAuth 客户端（application，或 SA 自己的 client_id）。
- `principal_*` 表示令牌代表的主体。

`client_credentials` 下二者可以重合（application 代表自己，SA 代表自己），但不能互相替代。这样 `token_exchange`（client A 代表 user U）与 `client_credentials`（client A 代表 client A）是同一套列的不同取值，不需要分支特判。

### D8. 审计枚举归一

`records.actor_id` 是单列 UUID，无 `actor_type`（`contracts/schema/60_audit.yaml:12-16`、外键 `:173-175`）；`authorization_audit_records` 已有 `actor_type`，取值为 `user | platform_admin | service_account | system`（`:323-324`、`:339`）。两者必须共用同一份封闭枚举：`user | service_account | client | workload | platform_admin | system`。

- `records` 新增 `actor_type VARCHAR(32) NOT NULL`，`actor_id UUID NULL`；去掉指向 `users` 的外键，改为与 token 相同的多态语义。
- `authorization_audit_records` 的 CHECK 扩为上述六值。
- `actor_id IS NULL` 仅允许 `actor_type='system'`。
- 枚举事实源放 `contracts/schema/`，生成器支持程度待核实。

哈希影响：`records` 规范串当前包含 `actor_id`（`internal/platform/outbox/providers/audit/chain.go:145-196`），插入 `actor_type` 会改变规范串。开发期删库重建时直接定义新规范串；存在历史数据时新增 `records.chain_version SMALLINT NOT NULL DEFAULT 1`，新行写 v2，验证器按版本选择规范串，禁止原地改算法。`authorization_audit` 的哈希公式注释已包含 `actor_type`（`internal/domain/authorization_audit/entity.go:11`、`repository.go:18`；字段在 `entity.go:27`、`:52`），扩枚举不改变哈希格式。

### D9. client_id 命名空间归一

当前 `applications.client_id` 可空且全局部分唯一（`contracts/schema/20_identity.yaml:719-721`、`:785-789`），`service_accounts.client_id` 非空且表内唯一（`contracts/schema/85_workload.yaml:60`、`:71`），跨表无唯一约束。认证解析是隐式的「服务账号优先」：`internal/app/auth/client_credentials.go` 的 `authenticateClientCredentials` 先查 SA（`GetAccountByClientID`），未命中才查 application；`ExchangeClientCredentials` 的注释却声明 client_id 全局唯一。

推荐方案 A：统一客户端注册层 + 分表承载。

```
oauth_clients
  id                UUID PK
  organization_id   UUID NOT NULL
  client_id         TEXT NOT NULL
  client_kind       VARCHAR(32) NOT NULL  -- application | service_account
  subject_table     VARCHAR(32) NOT NULL  -- applications | service_accounts
  subject_id        UUID NOT NULL
  status            VARCHAR(64) NOT NULL
  UNIQUE (client_id)
  UNIQUE (subject_table, subject_id)
```

认证入口只查 `oauth_clients`，一次命中得到 `(client_kind, subject_id, organization_id)`，再按 `subject_table` 取凭据细节，不再有隐式优先级。`applications.client_id` 与 `service_accounts.client_id` 保留为业务属性，不再作为认证键。签发时 `principal_type` 由 `client_kind` 决定，`issued_via_client_id = oauth_clients.client_id`。

备选 B（只建 `oauth_client_ids` 共享命名空间表）：改动较小，但两份凭据表仍需各自查询，`owner_type` 与业务表状态可能漂移。备选 C（application 与 SA 合并为一张 `clients` 表）：命名空间天然统一，但 `applications` 承载门户/菜单/品牌语义（`contracts/schema/20_identity.yaml:692-757`），合并会绑死两类生命周期。推荐 A：认证键唯一、类型显式、不动 application 的 UI 语义，并与 `principal_type` 对齐。

`oauth_clients` 只解决认证键唯一，不参与主体身份；`principal_type='client'` 时 `principal_id` 仍指向 `applications.id`。

## 理由

- **主体语义与存储解耦**：`principal_type/principal_id` 让五类签发路径共用一张 token 表，新增主体类型只影响签发路径与主体表，不影响读查询、RLS、审计。
- **RLS 单一锚点**：显式 `organization_id` 使策略退化为一次列比较，去掉对 `users` 的 join 依赖，租户判定不再随主体存亡变化。
- **审计可追溯**：token 主体与 audits actor 同构，`client_credentials` 与 workload 令牌都有真实 actor，而不是空列或借用用户 id。
- **代价可补偿**：失去单表外键的完整性由签发校验、审计同构、孤儿巡检三层覆盖；多态引用在本仓已有先例。

## 后果

正面：

- `access_tokens` 能表达 `authorization_code`、`client_credentials`、`token_exchange`、JWT bearer、workload 五类签发路径，不再出现空 uuid 或外键 500。
- token 读路径去掉 `JOIN users`，RLS 与列表查询可用 `organization_id` 复合索引直达。
- 审计 actor 与令牌主体一致，`principal_type=client` 的令牌可双向追溯。

负面与代价：

- 失去数据库级主体外键；完整性依赖应用层校验与巡检，写入路径必须集中，绕过签发器的直插会破坏 P1。
- `records` 规范串变化需要链版本化，审计历史验证必须按版本分支。
- `client_id` 归一需要认证入口切换，涉及 `oauth_clients` 新表与双写/回填（开发期删库重建可跳过回填）。

运维影响：新增 `orphaned_tokens_total` 指标与巡检任务；`token.orphaned` 审计事件需登记 `contracts/registries/events.yaml`。

安全影响：`principal_id` 不再是外键，必须保证所有 token 写入都经过 D3 的签发前校验；否则可构造指向不存在主体的令牌。

## 边界

### refresh_tokens 保持 user_id NOT NULL

`refresh_tokens.user_id UUID NOT NULL`（`contracts/schema/40_session.yaml:286-289`、外键 `:412-414`、RLS `:403-404`）不改。

显式不变量 R1：refresh token 仅由用户绑定流程签发，允许的签发路径只有：

- `authorization_code`（`internal/app/auth/token_exchange.go:233-251`，用户已认证）；
- `device_code`（`internal/app/auth/device_auth.go:200-217`，用户已认证）；
- `token_exchange` 且 subject 是本地 user（`internal/app/auth/rfc8693_token_exchange.go:288-318`，已有 `!isFederatedSubject` 守卫）。

不属于同一缺陷的理由：

- refresh token 的语义是「长期代表一个可交互用户继续换发」，天然绑定用户会话；非人主体不应获得长期刷新能力。
- `access_tokens` 是任意主体的短期凭证，`refresh_tokens` 是用户会话的续期凭证，两者主体域不同；把 refresh 也改成多态会把长期凭据扩展到机器主体。

`jwt-bearer` 决策时签发 refresh token 且 `UserID: ""`（决策时 `internal/app/auth/jwt_bearer_token.go:140-171`），违反 R1。现已显式拒绝：refresh 签发分支随封堵整体删除，认证通过后统一返回 400 `unsupported_grant_type`（`TestGetJwtBearerToken_BlockedAfterClientAuthentication` 断言零 access/refresh 落库）。

### 与 docs/design/protocol/oauth2.md 的关系

- `oauth2.md` §14.7 已定义 `client_credentials` 为「无用户主体：sub/用户名为 `client:{client_id}`；TTL 1h」；本 ADR 只解决「这种令牌如何在 `access_tokens` 中存储与隔离租户」，不改变该节的 grant 语义、TTL、客户端资格与 scope 规则。
- `oauth2.md` §14.13 的 `grant_types_supported` 已宣告 `client_credentials`；在 D1/D2 落地前，该宣告与真实签发能力不一致。封堵已落地：`ExchangeClientCredentials` 对已认证调用显式返回 400 `unsupported_grant_type`，不用 500 冒充支持；宣告与能力的收敛仍待 D1/D2。`jwt-bearer` 同构：discovery 经 `contracts/registries/grant-types.yaml` 宣告该 grant，而 `GetJwtBearerToken` 已封堵为 400 `unsupported_grant_type`；宣告收敛同样待 D1/D2，本 ADR 不改注册表与 `oauth2.md` 正文。
- `oauth2.md` §14.8 的 ID Token `sub` 规则（用户主键）不变；D5 只对 access token 与内省生效。
- 本 ADR 不修改 `oauth2.md` 正文；落地时以本 ADR 的存储模型为准，协议语义仍以 `oauth2.md` 为准。

### 与 OPEN-API-60 / OPEN-API-64 / OPEN-API-65 的关系

- **OPEN-API-60**（`client_secret_jwt` 不实现，状态 open）：缺口是原始 secret 的可逆存储与 HS256 验签设施缺失（`contracts/registries/open-items.yaml` 的 `OPEN-API-60` 条目）。本 ADR 不解决 secret 存储，也不改变「不实现 `client_secret_jwt`」的结论；关闭条件仍由 OPEN-API-60 持有。
- **OPEN-API-64**（revoke/introspect 客户端认证，已关闭）：两条端点已复用 token endpoint 的统一客户端认证裁决（含 `private_key_jwt` 与 mTLS），认证失败统一 401 `invalid_client`（闭环记录见 `contracts/registries/open-items.yaml`）。本 ADR 不扩展这两条端点的客户端认证方式，只改令牌主体存储。
- **OPEN-API-65**（revoke 认证失败响应，已关闭）：认证失败已统一为 401 `invalid_client`（三种失败消息逐字一致，防枚举）；令牌存在性语义（有效/无效均 200）不变。与令牌主体模型无关，本 ADR 不改变该行为。

## 替代方案

### 甲：user_id 改 nullable + 补 organization_id

做法：`access_tokens.user_id` 允许为空，新增 `organization_id`；机器主体留空 `user_id`，在 `metadata` 记 SA/client id。

否决理由：

- 主体仍然只能是 user，`client_credentials` 的 `sub` 语义靠「空 user_id + metadata 约定」。
- `token_exchange` 的 `act` 仍只能塞 `metadata`（`rfc8693_token_exchange.go:239`），无法作为一等列查询。
- workload 的外部 sub 是字符串，塞不进 `user_id`。
- nullable 外键制造三态：NULL 同时表示系统令牌与机器令牌，类型信息埋进 NULL。
- 只要 `user_id` 非空，显示名与组织仍倾向 `JOIN users`，读路径不会真正简化。

### 乙：应用绑定服务账号 + 影子用户

做法：每个 application 在 `users` 建一条不可登录的影子账号，`client_credentials` 把 `sub` 写成影子 user id，以维持外键。

否决理由：

- `users` 同时承担「可登录账号」与「身份目录」，所有用户查询都必须额外过滤影子用户，漏一处即越权或脏数据。
- application 与影子用户双生命周期，删除/禁用需要双向同步，状态漂移会产生僵尸用户或可登录的已删应用。
- 外部看到的 `sub` 是不可登录、无凭据的 user UUID，语义失真。
- RLS 与读查询仍经 `users`，没有切断租户归属与主体的耦合。
- workload 的外部 sub 没有稳定 UUID，只能哈希造影子用户，审计语义进一步失真。

### 丙：建 principals 实体注册表代替多态

做法：所有主体签发时向 `principals` 插一行，`access_tokens.principal_id` 单列外键指向它。

否决理由：多态变成伪单态（`principal_type` 仍要存在）；注册表与主体表双写；存量主体需要回填；外键 `CASCADE` 语义对审计与令牌生命周期并不合适；相比 D3 的三层补偿多一张表与一条写路径，收益仅是恢复外键。

## 读路径改写清单

### PostgreSQL token SQL（`internal/infra/postgres/tokens/sql.go`）

| 语句 | 位置 | 当前依赖 | 改写 |
|---|---|---|---|
| `queryInsertToken` | `:4-5` | `user_id`、`$2::uuid` | 改为 `organization_id, principal_type, principal_id, actor_type, actor_id, application_id, issued_via_client_id` |
| `queryGetTokenByID` | `:17-41`（join `:38`） | `JOIN users` 取 `username`、`organization_id` | 直接取 `t.organization_id`；显示名走 `v_principals` 或按主体类型左连 |
| `queryGetTokenByIDForOrganization` | `:46-71`（join `:67`） | `u.organization_id = resolve_org_id($2)` | `t.organization_id = resolve_org_id($2)` |
| `queryGetTokenByHash` | `:73-97`（join `:94`） | `JOIN users` | 同上；内省与资源服务器主路径 |
| `queryGetTokenByDeviceSecretHash` | `:99-123`（join `:120`） | `JOIN users` | 同上 |
| `queryGetRefreshTokenByHash` | `:125-148`（join `:144`） | `JOIN users`（refresh 仍 user-only） | 保留 join，但可用显式 `organization_id` 过滤 |
| `queryListTokens` | `:150-180`（join `:171`） | `u.organization_id`、`u.username`、`t.user_id` | 改为 `t.organization_id`、`t.principal_type/principal_id`；搜索名走 `v_principals` |
| `queryCountTokens` | `:182-192`（join `:185`） | 同上 | 同上 |
| `queryRevokeTokensByQuery` | `:202-213`（`FROM users` `:207-208`） | `UPDATE ... FROM users` 推组织 | 去 `FROM users`，直接按 `t.organization_id` 过滤 |
| `queryRevokeRefreshByUser` | `:257-276` | `refresh_tokens.user_id` | 不改（R1） |
| `queryRevokeByAuthorizationCodeID` | `:282-301` | `metadata ->> 'authorizationCodeId'` | 提升为正式列 `authorization_code_id UUID NULL`，不再依赖 JSONB |

### MySQL 镜像（`internal/infra/mysql/tokens/sql.go`）

同构改写，遵守 OPEN-API-37（MySQL 无 RLS，必须显式 `organization_id` 过滤）：

- insert `:23`：列改造同 PostgreSQL。
- 三表 join `:60-61`、`:101-102`、`:190` 去掉 `JOIN users`。
- 列表/计数 `:144-145`、`:159-160`：`u.organization_id` → `organization_id` 列。
- `:214-229` 的 `RevokeByFamilyID` 与 `:251` 的 `JOIN refresh_tokens ... AND rt.user_id = ?` 保留（R1）。

### 跨域读路径

| 文件:行 | 查询 | 影响 | 改写 |
|---|---|---|---|
| `internal/infra/postgres/dashboard/sql.go:57-62` | token 计数 | `JOIN users` 推组织 | 直接按 `organization_id`；总数语义变为所有主体令牌，是否只统计 `principal_type='user'` 需产品确认 |
| `internal/infra/postgres/authorizationcodes/sql.go:34` | `JOIN users` | 授权码仍 user-only | 不改 |
| `internal/infra/postgres/sessions/sql.go:29,61,92,107` | session 查询 | `sessions.user_id` 仍 user-only | 不在本 ADR 范围 |
| `internal/infra/postgres/scim/sql.go` | `JOIN users` | SCIM 面向用户 | 不改 |

### 应用层读/判定

| 文件:行 | 当前 | 目标 |
|---|---|---|
| `internal/app/tokens/introspect_token.go:60`、`:94` | `Sub: token.UserID` / `refreshToken.UserID` | access 用 `tokens.ExternalSubject(...)`；refresh 仍 user |
| `internal/app/auth/oidc_discovery.go:307` | UserInfo 用 `token.UserID` 查 `users` | 先判 `principal_type='user'`，非 user 按 scope 语义处理 |
| `internal/app/auth/rfc8693_token_exchange.go:244`、`:301` | 继承 `subjectToken.UserID` | 继承 `principal_type/principal_id`；`actor_*` 取第一层 |
| `internal/app/auth/rfc8693_token_exchange.go:485` | `actor.Subject = actor.UserID` | `actor.Subject = ExternalSubject(...)` |
| `internal/app/auth/rfc8693_federation.go:215-223` | 合成 Token `UserID: sub` | 映射 `principal_type='workload'`，外部 sub 放 `external_subject`（Q3） |
| `internal/app/workload/service.go:701`、`:709` | 按 `UserID=SA.ID` 统计/吊销 | 按 `(principal_type='service_account', principal_id=SA.ID)` |
| `internal/app/applications/get_delete_summary.go:25` | 按 `application_id` 统计 | 列仍存在；删除前置检查加「是否存在以该 client 为 `issued_via_client_id` 的活跃令牌」 |
| `internal/app/users/impersonate_user.go:98-100` | `UserID: user.ID` | 加 `PrincipalType='user'`、`OrganizationID` |
| `internal/app/auth/device_auth.go:200-217` | 同上 | 同上 |
| `internal/app/auth/token_exchange.go:152-169` | 同上 | 同上 |
| `internal/app/auth/client_credentials.go`（`ExchangeClientCredentials` 封堵分支） | 已封堵：认证后统一 400 `unsupported_grant_type`，不落库 | 解除封堵，按 principal 设置 `PrincipalType/PrincipalID/OrganizationID/IssuedViaClientID` |
| `internal/app/auth/jwt_bearer_token.go`（`GetJwtBearerToken` 封堵分支） | 已封堵：认证后统一 400 `unsupported_grant_type`，refresh 分支已删除（R1） | 解除封堵，按 principal 设置 `PrincipalType='client'` 等字段；refresh 不恢复（R1） |
| `internal/domain/tokens/token.go:42-45` | `Validate` 强制 `UserID` | 校验 `PrincipalType/PrincipalID/OrganizationID/ApplicationID` 组合 |

### RLS 与索引

- 声明层：`contracts/schema/40_session.yaml:269-270` 的 `access_tokens.rls_predicate` 替换为 D2 谓词。
- 生成 DDL：重跑 schema 生成器；禁止手改 `db/postgresql/V1__guliangiam_schema_init.sql`（`access_tokens` 表与策略段以 `grep -n 'access_tokens' db/postgresql/V1__guliangiam_schema_init.sql` 定位）。
- 索引替换（`40_session.yaml:234-266`）：删 `idx_access_tokens_user_active`、`idx_access_tokens_user_app`、`idx_access_tokens_user_id_created`；增 `(organization_id, principal_type, principal_id, expires_at) WHERE revoked_at IS NULL`；保留 `idx_tokens_app_expires` 与 application_id 相关索引。
- `sessions`、`refresh_tokens` 的 user-based RLS 不改。

## 落地路径

开发期直接删库重建，不写兼容迁移；结构只改 `contracts/schema/*.yaml` 后生成。

步骤 1：封堵（无 Schema 变更）——已完成

- ✅ 已落地：`client_credentials` 的两个主体分支（application 与 service account）在认证后统一显式返回 400 `unsupported_grant_type`（RFC 6749 §5.2），不再 500（`internal/app/auth/client_credentials.go` 的 `ExchangeClientCredentials` 封堵分支）。事实是「该 grant 当前不支持」，不是「客户端未授权」，因此不使用 `unauthorized_client`；认证失败仍为 401 `invalid_client`。
- ✅ 已落地：`jwt-bearer` 镜像同一封堵模式（`internal/app/auth/jwt_bearer_token.go` 的 `GetJwtBearerToken`）：断言验证事务提交（jti 消费保留，重放保护不放松）后统一返回 400 `unsupported_grant_type`；空 `UserID` 的 access 构造与 refresh 签发分支整体删除（R1 违反关闭）；认证失败仍 401。回归：`TestGetJwtBearerToken_BlockedAfterClientAuthentication`。
- 出口：`client_credentials`（两个分支）与 `jwt-bearer` 均稳定返回 400 + `unsupported_grant_type`，无 500、无空 uuid 日志（已达成）。

步骤 2：Schema 流

- `contracts/schema/40_session.yaml`：`access_tokens` 列/约束/索引/RLS 改造。
- `contracts/schema/60_audit.yaml`：`records.actor_type` + CHECK + 索引；`authorization_audit_records` CHECK 扩枚举。
- 客户端命名空间声明（D9 方案 A 的 `oauth_clients`，若采纳）。
- `v_principals` 视图声明（生成器支持情况待核实；不支持则以手写迁移引入并登记缺口）。
- 出口：`check-migrations.sh`、`check-schema-parity.py`、`check-generated-zero-diff.sh` 全绿。

步骤 3：写路径代码

- `internal/domain/tokens/token.go` 字段与 `Validate`。
- `internal/app/auth/{client_credentials,jwt_bearer_token,token_exchange,device_auth,rfc8693_token_exchange}.go` 设置新字段。
- `internal/infra/{postgres,mysql}/tokens/` 的 insert SQL 与参数。
- 新增 `internal/domain/tokens/subject.go`（待建）。

步骤 4：读路径代码

- 本文「读路径改写清单」全部条目。
- `internal/app/tokens/introspect_token.go`、`internal/app/auth/oidc_discovery.go`。
- `internal/app/workload/service.go`、`internal/app/applications/get_delete_summary.go`。

步骤 5：审计同构

- `internal/infra/postgres/records/sql.go`、`audit/embed.go`、`chain.go` 规范串版本化。
- `authorization_audit` 枚举常量与校验。
- `orphaned_tokens_total` 指标与巡检任务（`token.orphaned` 审计）。

步骤 6：client_id 命名空间（D9）

- `oauth_clients` 落地、认证入口切换。
- 删除「服务账号优先」隐式分支。

测试重写（必须重写，不允许放宽）：

| 测试 | 位置 | 原因 |
|---|---|---|
| token SQL mock 列期望 | `internal/infra/postgres/tokens/repository_create_test.go`、`repository_extra_test.go` | 插入列与参数个数变化 |
| 租户伪造 org | `internal/transport/http/api/v1/tokens/tenant_forged_org_test.go` | 语义从 users join 改为显式 `organization_id` 过滤，拒绝断言保留 |
| OIDC claim 一致性 | `internal/app/auth/oidc_claims_conformance_test.go` | `sub` 编码规则变化 |
| RFC 8693 短时主体 | `internal/app/auth/rfc8693_shortlived_test.go` | `fed:` 合成主体映射为 workload |
| workload 删除预检 | `internal/app/workload/service_test.go` | 当前假设按 `UserID=SA.ID` 查询 |
| 内省 | `internal/app/tokens/service_test.go` | `Sub` 来源变化 |

不得放宽：跨租户返回 not found / inactive 的拒绝路径；RLS 隔离测试；凭据脱敏断言；审计链完整性测试（`internal/platform/outbox/providers/audit/*_test.go`）。

## 开放问题（待定）

### Q1. workload / jwt-bearer 场景 application_id 是否可空（待定）

倾向：可空，并新增 `issued_via_client_id TEXT NOT NULL`。

- `client_credentials` 下签发者就是主体自身，不存在第二个 application；强制非空会逼代码把主体 id 塞进 `application_id`。
- `jwt-bearer` 的客户端是 assertion 的 `iss`（`internal/app/auth/jwt_bearer_token.go:113`、应用查找 `:67`），可能是 application，也可能不是；`issued_via_client_id` 更准确。
- 需产品确认：管理面列表是否要求每个令牌都显示应用；若要求，可用 `issued_via_client_id` 回退展示。

### Q2. principal_type=client 时 principal_id 指向 applications 还是中间表（待定）

倾向：直接指向 `applications.id`，不建中间表。

- `applications` 是 client 的唯一业务实体（`contracts/schema/20_identity.yaml:683-809`），1:1 映射不需要中间层。
- `oauth_clients` 只解决认证键唯一，不承担主体身份。
- 若未来一个 application 需要多个客户端人格，再引入 `application_clients` 子表，`principal_id` 仍指向 application，`issued_via_client_id` 区分人格。
- 风险：`applications` 软删后主体不存在；删除应用时吊销其作为主体/签发者的令牌，巡检将 `client` 孤儿计入指标，不级联删除审计。

### Q3. workload 主体的 principal_id 指向什么（待定）

倾向：新增 `workload_identities` 表（`id, organization_id, pool_id, issuer, subject, status`），首次联邦成功时物化。

- `identity_pools` / `federation_trusts` 是配置，不是某个外部 workload 的身份（`contracts/schema/85_workload.yaml:5-49`）；指向 pool 会让同池所有 workload 共用一个主体。
- 外部 `subject` 是字符串，不能直接做 `principal_id UUID`；物化后才有内部 UUID。
- 若暂不物化，`principal_type='workload'` 的令牌无法写入（`principal_id NOT NULL`），该类型只是预留枚举。
- 待核实：`identity_pools` / `federation_trusts` 的 RLS 与唯一键是否足以支撑 `(pool_id, issuer, subject)` 唯一约束（`contracts/schema/85_workload.yaml:16-22`、`:40-49`）。

### Q4. authorization_code_id 是否提升为正式列（待定）

倾向：是。当前依赖 `metadata ->> 'authorizationCodeId'` 与部分索引（`contracts/schema/40_session.yaml:227-230`）。提升为 `authorization_code_id UUID NULL` 正式列加外键（`ON DELETE SET NULL`），让「代码泄露 → 令牌吊销」可索引、可审计。不影响主体模型，但同批改动成本最低。

### Q5. records.actor_id 是否保留对 users 的外键（待定）

倾向：去掉，改为多态 + `actor_type`。保留外键会让 client / service_account / workload 审计写不进去，与 token 表同一缺陷。若坚持保留，则 `actor_id` 必须允许为空并把真实 actor 放进 `metadata`，会重演甲案的三态问题。

## 验证证据

本 ADR 为 `[Target]`，以下为当前事实的可复现证据与落地出口条件。

当前事实（可复现）：

```bash
rg -n "user_id|rls_predicate" contracts/schema/40_session.yaml
rg -n "JOIN guliangiam.users|user_id" internal/infra/postgres/tokens/sql.go
rg -n "UserID|ApplicationID|principalType" internal/app/auth/client_credentials.go
rg -n "CreateServiceAccount|CreateAccount" internal/transport/http/api/v1/identitypools internal/app/workload
go test ./internal/app/auth/... ./internal/infra/postgres/tokens/...
```

结论：`access_tokens.user_id` 非空且外键指向 `users`；token 读路径依赖 `users` join；`client_credentials` 与 `jwt-bearer` 均已在写入前封堵（400 `unsupported_grant_type`），解除封堵后真实库写入将因空 uuid 或外键失败；jwt-bearer 的 refresh 签发分支已删除（R1）。

出口条件：

1. `contracts/schema/40_session.yaml` 与 `60_audit.yaml` 按 D1/D2/D8 生成，`check-generated-zero-diff.sh` 零差异。
2. `client_credentials` 两个分支在隔离库真实签发成功：application 与 service account 各写入一条 `access_tokens`，`organization_id` 与主体组织一致，RLS 跨租户不可见。
3. `token_exchange` 的 `actor_type/actor_id` 落列，JWT `act` 保留深链。
4. `records` 审计 actor 与 token 主体一致；`chain_version` 验证通过。
5. `orphaned_tokens_total` 指标可观测，孤儿巡检能吊销并写 `token.orphaned`。
6. `v_principals` 对三张真实主体表返回一致的主体视图，`workload` 分支按 Q3 结论处理。

待取证项：

- 外部 CI 是否已有 `client_credentials` 的真实库覆盖。
- `scripts/gen_schema.py` 是否支持声明视图。
- `workload_identities` 的唯一键与 RLS 设计（Q3）。
- 管理面列表是否需要强制展示 `application_id`（Q1）。

## 复审条件

- 任一流程新增主体类型（例如 delegation 之外的机器身份），或主体表生命周期变化（新增软删/禁用语义）时复审 D1/D3/D4。
- `oauth2.md` §14.7 的客户端模式 `sub` 规则变更时复审 D5。
- `records` 哈希链引入新规范串或 `chain_version` 时复审 D8。
- `client_id` 命名空间方案（D9）从 A 切换到 B/C，或 `applications` / `service_accounts` 任一调整唯一约束时复审。
- 开放问题 Q1–Q5 任一作出决定时，在本 ADR 内更新对应小节并重新评审状态。
