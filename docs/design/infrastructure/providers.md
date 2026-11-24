# 统一 Provider 注册与配置中心

> 设计锚点：[DESIGN:infrastructure/providers]

> 目标态设计：把身份、AI、支付、通知四类 Provider 收敛为「一个注册中心 + 族内子类型表」，
> 控制面字段唯一，配置与密钥按族内语义存放。身份族内由专用子系统托管的 kind
> （LDAP、验证码、Face ID）不进入统一注册层，边界与 422 契约见
> [ADR-037](../../adr/ADR-037-provider-registry-boundary.md)。
> 声明式 Schema 为唯一事实源（[ADR-022](../../adr/ADR-022-declarative-schema-versioned-migration.md)），
> 租户隔离遵循 [ADR-016](../../adr/ADR-016-rls-tenant-isolation.md)，支付渠道 SPI 见
> [ADR-035](../../adr/ADR-035-payment-channel-spi.md)。
> 复现命令：`python3 scripts/gen_schema.py --dialect postgresql --check && python3 scripts/check-schema-parity.py`。

---

## 1. 适用范围

| 族 | 存储位置 | 控制台入口 | 写权限点 |
|----|----------|-----------|----------|
| identity | [20_identity.yaml](../../../contracts/schema/20_identity.yaml) 的 `providers` / `provider_configs` / `provider_secrets` | 身份提供商 | `providers:write` |
| ai | [75_ai_gateway.yaml](../../../contracts/schema/75_ai_gateway.yaml) 的 `llm_providers` | AI 网关 · 供应商 | `aigateway:providers` |
| payment | [71_payment.yaml](../../../contracts/schema/v3/71_payment.yaml) 的 `pay_channel_configs` | 支付渠道配置 | `payments:write` |
| notification | [80_governance.yaml](../../../contracts/schema/80_governance.yaml) 的 `notification_providers` | 通知提供商 | `notification-providers:write` |

鉴权面登记见 [openapi.yaml](../../../openapi/openapi.yaml)（`/api/v1/providers`、`/api/v1/ai/providers`、`/api/v1/pay-channel-configs`、`/api/v1/notification-providers`）；
复现命令：`grep -n 'x-permission:.*providers' openapi/openapi.yaml`。

两轨边界：统一注册中心不承载由专用子系统托管的 kind。LDAP、验证码族、Face ID 族由
专用控制台唯一创建与管理；存储 Provider 保持既有独立资源。kind → 归属的单一事实源是
`contracts/registries/provider-kinds.yaml`（`managed_via` / `consumer_table`），复现与
校验：`python3 scripts/check-provider-kinds.py`。

| 子系统 | 专用表 | 专用控制台（唯一创建入口） |
|--------|--------|---------------------------|
| LDAP | `ldaps` | `/api/v1/ldaps` |
| 验证码 | `captcha_providers` | `/api/v1/captcha-providers` |
| Face ID | `face_id_providers` | `/api/v1/face-id-providers` |

统一入口对这些 kind 的 create、kind/providerType 变更、activate、rotate secret、
test-connection 返回 422（`field=kind`、`value=<kind>`、`managedBy=<专用控制台路径>`）；
list / get / disable / delete 保留，delete 仍受「先停用再删除」保护。完整决策见
[ADR-037](../../adr/ADR-037-provider-registry-boundary.md)。

---

## 2. 设计约束

1. **基表不可另起**。`guliangiam.providers` 已被 `provider_configs`、`provider_secrets`、
   `application_provider_bindings`（应用与 Provider 的绑定）以及 `secret_rotation_audit`
   的 RLS 谓词引用；新建独立注册表会复制这些引用关系并让 V1 冻结基线的语义分叉。
2. **四方言可表达**。结构变更只能使用 PostgreSQL / MySQL / GaussDB / KingBaseES 共同可表达的能力：
   不使用触发器维持双写、不使用函数生成派生列；派生关系放在应用层或视图之外。
3. **密钥不落明文**。读接口不得回显明文凭据；密文列必须配指纹列，明文只在创建/轮换响应中出现一次。
4. **租户边界不变**。所有族表都带 `organization_id` 且启用 `RLS ENABLE + FORCE`；
   收敛不得引入跨租户可读的注册表。
5. **迁移可回滚**。每个结构迁移必须有配对回滚脚本；数据迁移必须幂等（`ON CONFLICT DO NOTHING` / 条件更新）。
6. **托管 kind 不进统一注册层**。LDAP、验证码族、Face ID 族的运行期消费者读专用表，
   统一 `providers` 行对这些 kind 不产生运行期效果；统一入口不得为其提供 create、
   kind 变更、activate、密钥轮换或连接测试。边界以 [ADR-037](../../adr/ADR-037-provider-registry-boundary.md) 为准。

---

## 3. 目标模型

### 3.1 分类学

控制面用两级分类替代四套互不相干的类型枚举：

| 层级 | 取值 | 说明 |
|------|------|------|
| `family` | `identity` / `ai` / `payment` / `notification` | 决定使用哪张子类型表、哪个适配器工厂、哪个权限点 |
| `kind` | 族内闭集（如 `oauth`、`saml`、`openai`、`alipay`、`smtp`） | 决定配置字段集合与连接测试实现；闭集以声明式 Schema 的 CHECK 为事实源 |
| `variant` | 空串或族内区分维度 | 仅支付族使用（取值为 `sandbox` / `production`），其余族固定空串 |

`kind` 的闭集与前端 `@enumset` 清单必须逐值一致，由 `python3 scripts/check-enums.py` 校验。

身份族历史字段 `category` 仅作为 `kind` 的展示标签保留在视图/响应中，不参与新写入。

### 3.2 三层存储

```
provider 注册（控制面，1 行 = 1 个逻辑 Provider）
   └── provider 明细（族内配置，1:1 或 1:N）
          └── provider 密钥（密文 + 指纹，1:N 按 secret_type 轮换）
```

**注册层**：`guliangiam.providers` 增补 `family`、`kind`、`variant` 三列，成为控制面唯一事实源。
承载：租户、机器名、显示名、分类、状态、乐观锁 `revision`、软删除、审计主体。

**明细层**：

| 族 | 明细表 | 基数 | 承载 |
|----|--------|------|------|
| identity | `provider_configs`（`config_type` + JSONB） | 1:N | 统一注册层内的 OAuth/OIDC/SAML 端点、映射、模板等非密配置；`_provider_settings_v1` 承载 Casdoor 兼容的 Provider 选项（`emailRegex`/`enablePkce`/`enableProxy`/`httpHeaders`/SAML 开关/`cert`/`customLogo`/`customLogoutUrl`/`state`） |
| ai | `llm_providers` | 1:1 | 协议、模型路由、限流、超时 |
| payment | `pay_channel_configs` | 1:N（按 `version_no` 版本链） | 渠道配置密文、环境、测试/激活时间 |
| notification | `notification_providers` | 1:1 | 渠道类型、Webhook、非密配置 |

托管 kind 的配置不在 `provider_configs`：LDAP 使用 `ldaps`，验证码使用
`captcha_providers`，Face ID 使用 `face_id_providers`；这些专用表由各自控制台写入，
统一注册层不为其保存明细。

明细表统一新增 `provider_id` 指向注册层，删除语义随注册层软删除；支付族按
`(provider_id, version_no)` 唯一，并以部分唯一索引保证同一 Provider 只有一个 `active` 版本。

**密钥层**：目标态是所有族的密文字段落入 `provider_secrets`（`secret_type` + 密文 + 指纹 + 轮换时间 + 过期时间 + 版本）。
加密方案为原生列 envelope 加密（`client_secret` / `client_secret2` / `password` / `app_secret` 等列写 `vN-aes|vN-sm4` 令牌，读侧 fail-closed），
`provider_secrets` 仅记录轮换状态/指纹/版本，不保存密文；明细表的密文列迁移到密钥层仍按 §5 的收口条件推进。
托管表的凭据由各自仓储按专用控制台的语义处理，统一 `provider_secrets` 不承载它们，也不参与其轮换。

### 3.3 生命周期

统一状态机：`draft → testing → active → disabled`，允许 `disabled → testing` 回退重新启用；显式 enable 对 `draft/disabled` 先内部桥接 testing 再进入 active。
`active` 是唯一可被业务引用的状态；`testing` 仅供连接测试与试运行。

| 动作 | 前置状态 | 结果 | 审计事件 |
|------|----------|------|----------|
| 创建 | — | `draft` | `provider.<family>.created` |
| 连接测试 | `draft` / `testing` / `disabled` | 只读探测，不改状态、不写审计；失败在响应中返回错误码 | — |
| 激活 | `draft` / `testing` / `disabled` | `active`（draft/disabled 先内部桥接 testing；族内引用立即生效） | `provider.<family>.activated` |
| 停用 | `active` | `disabled`（新请求不再路由，在途请求按族内姿态收敛） | `provider.<family>.disabled` |
| 轮换密钥 | 任意非删除态 | 新增密钥版本，旧版本按宽限期失效 | `provider.<family>.secret.rotated` |
| 删除 | 非 `active` | 软删除（作者提供服务已绑定时拒绝） | `provider.<family>.deleted` |

上述状态机只作用于统一注册层内的 kind。托管 kind 由专用控制台管理；统一入口对它们
保留 list / get / disable / delete，对其 create、kind/providerType 变更、activate、
rotate secret、test-connection 返回 422，元数据为 `field=kind`、`value=<kind>`、
`managedBy=<专用控制台路径>`。delete 仍受「先停用再删除」保护：`active` 或仍可路由的行
拒绝删除。完整契约见 [ADR-037](../../adr/ADR-037-provider-registry-boundary.md)。

身份族的 `enabled` 布尔仅作为兼容响应字段由状态派生，不再作为独立写入口。

### 3.4 唯一性与索引

| 约束 | 定义 |
|------|------|
| 注册唯一 | `(organization_id, family, kind, name, variant)`，条件 `deleted_at IS NULL` |
| 明细锚点 | `(provider_id)` 唯一（支付为 `(provider_id, version_no)`） |
| 单活版本 | `(provider_id)` 唯一，条件 `status = 'active' AND deleted_at IS NULL`（支付族） |

新增唯一索引后必须同步删除被其最左前缀覆盖的普通索引，否则
`python3 scripts/check-redundant-index.py` 会判定冗余。`TEXT` 列不得携带默认值
（`python3 scripts/check-schema-defaults.py`），新增枚举列使用 `VARCHAR` + CHECK。

### 3.5 密钥与脱敏

- 密文以信封加密存储（`config_sealed` / 密钥层），密钥由 `ENCRYPTION_KEY_CURRENT` 派生，轮换按既有域分离约定。
- 指纹列用于去重与审计比对，不可逆。
- 读响应只返回 `XxxSet` 布尔 + `XxxMasked` 字符串；由
  `python3 scripts/check-credential-masking.py` 校验，不再为每族维护例外清单。
- 明文只在创建与轮换的响应体中出现一次，且不进入审计载荷。

---

## 4. 应用层收敛

### 4.1 领域与应用服务

- 领域根：`internal/domain/providers` 扩展 `Family` / `Kind` / `Variant` 与统一 `Status`，现有身份族枚举降为 `identity` 族的 `kind` 集合。闭集不因双轨边界收缩：托管 kind 仍可读，只是不进入统一生命周期。
- 族内策略：每个族实现同一 `ProviderStrategy` 接口（校验、连接测试、配置脱敏、激活副作用），
  由 `internal/app/providers` 按 `family` 选择实现；AI、支付、通知的现有工厂
  （`internal/infra/aiupstream`、`internal/infra/payment`、`internal/infra/notification`）保持独立，不被合并。
- 统一用例：`CreateProvider` / `UpdateProvider` / `ChangeProviderStatus` / `RotateProviderSecret` /
  `TestProviderConnection` / `DeleteProvider`：Create/Update/ChangeStatus/Rotate/Delete 在 `uow` 单事务内写注册层、明细层、审计与 Outbox；`TestProviderConnection` 是只读探测，不推进状态、不写审计/Outbox，激活由显式 enable 触发。

### 4.2 接口面

| 接口 | 语义 |
|------|------|
| `GET/POST /api/v1/providers?family=` | 跨族列表与创建（`family` 必填参与创建，列表可省略表示全部） |
| `GET/PATCH/DELETE /api/v1/providers/{id}` | 详情、局部更新、软删除 |
| `POST /api/v1/providers/{id}/test-connection` | 连接测试（只读探测，不写状态/审计） |
| `POST /api/v1/providers/{id}/enable` | 显式启用（探测通过后由操作者触发，`draft→testing→active`） |
| `GET/PATCH /api/v1/providers/{id}/config` | 族配置读取/更新；PATCH 提交 secret 字段即触发密钥轮换用例（`RotateProviderSecretCommand`，明文仅创建/轮换响应一次性返回） |
| `GET /api/v1/providers/{id}/secret-status` | 密钥版本与指纹状态 |

端点、方法与权限的唯一事实源为 `internal/transport/http/api/v1/providers/routes.go` 的 `route.Definition` 声明（不存在独立的 `:rotate-secret` 端点；轮换经 PATCH secret 字段收敛，见 `internal/app/providers/unified_mutation.go`）。

托管 kind 在上表的 create、kind 变更、enable、secret 轮换（PATCH secret 字段）、test-connection 上返回
422；list / get / disable / delete 保留，delete 仍受「先停用再删除」保护。`PATCH` 中
kind/providerType 变更与 secret 字段同样返回 422，其余既有字段编辑保持原行为。元数据
契约见 [ADR-037](../../adr/ADR-037-provider-registry-boundary.md)。

族内既有端点（`/api/v1/ai/providers`、`/api/v1/pay-channel-configs`、`/api/v1/notification-providers`）
在收敛期保留为适配器：请求转换为统一用例，响应字段与权限点不变；下线前必须在 CHANGELOG 与
`openapi/openapi.yaml` 同步标注替代端点。

### 4.3 权限

新资源使用 `providers:read` / `providers:write`。适配器端点继续使用各族既有权限点，
映射关系写进路由注册表；同一请求不得同时要求两个权限点。跨族批量操作（如全部停用）
不提供——它会让单一权限点覆盖全部族。

### 4.4 控制台

统一「Provider 中心」承载统一注册层 kind 的创建、编辑、停用与密钥轮换；四族页面保留
各自菜单与表单，并共享同一 Provider 状态控件、密钥轮换弹窗与连接测试交互。
创建/编辑表单不提供托管 kind；列表筛选与历史行展示保留这些类目，历史行只提供查看、
停用、删除。LDAP、验证码、Face ID 的创建与编辑只在各自专用控制台进行。
页面清单与通用验收口径见 [frontend/workbenches.md](../frontend/workbenches.md) 的通用骨架约定。

---

## 5. 存储收敛路径

以下阶段只覆盖统一注册层内的 kind；托管 kind 的专用表、专用控制台与登录链路不参与
回填、双写或收口。

### 阶段 A：应用层收敛（无 DDL）

统一领域模型、用例与 `/api/v1/providers`，读写仍落在四张既有表上；适配器端点转调统一用例。
出口条件：新端点四族全部可创建/更新/测试/停用，旧端点行为与权限不变，审计事件含统一前缀。

### 阶段 B：注册层增量（声明式 Schema 迁移，ADR-022）

1. `family` / `kind` / `variant` 为 `providers` 基线列（`contracts/schema/20_identity.yaml` 声明，nullable）；既有行按 `identity` 归一（`kind` 由 `provider_type` 归一）。
2. 为 AI、支付、通知三族在 `providers` 插入注册行（同租户、同机器名），并在明细表回填 `provider_id`。
3. 明细表增加 `provider_id` 列、唯一约束与外键；支付族按 `(channel, environment)` 合并为单一注册行，
   `version_no` 保留在明细层。
4. 新增注册唯一索引，删除被覆盖的冗余索引。

结构变更只改 `contracts/schema/*.yaml` 声明，由 `scripts/gen_schema.py` 生成 `db/postgresql/V{N}` 增量迁移与 `db/rollback/` 配对回滚；kind 闭集扩展表现为 `ck_providers_kind` CHECK 约束变更的生成迁移（如 `V10__ck_providers_kind.sql`），不是新增列迁移。数据回填走 `db/seeds/V1__guliangiam_seed_init.sql` 单文件幂等追加（禁止新增种子版本）。`bash scripts/check-migrations.sh` 校验配对与版本连续性。

### 阶段 C：双写与一致性校验

写路径在同事务内写注册层与明细层重复字段；夜间校验任务比对注册层 `name/status/deleted_at`
与明细层同名列，差异写入告警而不自动"修复"。出口条件：连续校验无差异后进入阶段 D。

### 阶段 D：收口

1. 读路径全部切到注册层；明细层重复列降级为只读。
2. 统一注册层内的身份族专有列分批迁入 `provider_configs`，基表只保留控制面字段；
   托管 kind 的专用列不迁入。
3. 各明细表密文列迁入 `provider_secrets`，删除密文列。
4. 删除重复列与适配器端点，`openapi/openapi.yaml` 移除旧路径。

---

## 6. 回滚与兼容

| 阶段 | 回滚动作 |
|------|----------|
| A | 关闭新端点开关，旧端点仍读写原表 |
| B | 执行 `db/rollback/` 配对脚本删除新增列与注册行；数据迁移幂等可重放 |
| C | 停止双写，读路径回退到明细层 |
| D | 不提供向前回滚；收口前必须以阶段 C 的一致性证据与应用切流结果作为准入 |

双轨边界的回滚不涉及数据迁移：移除统一入口的 422 拒绝面与注册表归属字段即可恢复
旧行为；专用表、专用控制台与历史统一行保持不变。回滚不得删除历史行，也不得把专用
表重新并入统一注册层。

兼容期约束：明细层新增的 `provider_id` 允许为空（回填未完成的行），应用读取时以注册层缺失视为
"未收敛"并走旧路径；注册层存在而明细缺失视为脏数据，写入失败并告警。

---

## 7. 验收口径

| 维度 | 判定 |
|------|------|
| Schema | `python3 scripts/gen_schema.py --dialect <dialect> --check` 四方言全部零差异；`python3 scripts/check-schema-parity.py` 通过 |
| 迁移 | `bash scripts/check-migrations.sh` 配对与版本连续；全新库迁移 + 种子可用 |
| 租户隔离 | `bash scripts/check-rls.sh` 覆盖新增列与注册行；跨租户读取返回空集 |
| 引用完整 | `python3 scripts/check-fk-coverage.py`：明细表 `provider_id` 为物理外键 |
| 生命周期 | `python3 scripts/check-lifecycle-coverage.py`：新增 `expires_at` / `deleted_at` 的列被策略覆盖 |
| 数据访问 | `python3 scripts/check-sql-columns.py`、`python3 scripts/check-sql-runtime.py`、`python3 scripts/check-sql-args.py` 全绿 |
| 契约 | `python3 scripts/check-api-params.py`、`python3 scripts/check-http-methods.py`、`python3 scripts/check-enums.py` 全绿 |
| 凭据 | `python3 scripts/check-credential-masking.py` 无需为 Provider 新增例外 |
| 双轨边界 | `python3 scripts/check-provider-kinds.py`：注册表 `managed_via` / `consumer_table`、Go 拒绝集合与运行时 pending 路由一一对应；统一入口对托管 kind 的 create / kind 变更 / activate / rotate secret / test-connection 返回 422，list / get / disable / delete 保留 |
| 控制台 | 统一控制台创建/编辑不提供托管 kind；列表筛选与历史行保留；专用控制台仍是 LDAP、验证码、Face ID 的唯一创建入口 |
| 行为 | 同一 Provider 在族内唯一；`active` 才被路由；停用后新请求不再命中；轮换后旧密钥在宽限期后失效；统一注册层四族连接测试均返回结构化结果，托管 kind 的连接测试只在专用控制台执行 |

---

## 8. 非目标

- 不合并四族的适配器工厂与协议客户端：各族协议差异大于共性。
- 不引入通用 "Provider 插件 SDK"：`kind` 闭集由 Schema 与代码枚举共同约束，动态加载推迟到独立 ADR。
- 不把托管 kind（LDAP、验证码、Face ID）纳入统一注册层；专用表、专用控制台与登录链路
  保持不变（边界见 §1 与 [ADR-037](../../adr/ADR-037-provider-registry-boundary.md)）。
- 不删除托管 kind 的 Schema 闭集，不迁移或批量删除历史统一行；存储 Provider 保持既有独立资源。

---

## 9. 能力边界与不支持语义

「当前部署不支持某能力」必须返回 503 `unavailable`（`apperrors.NewUnavailable`），
不得 panic 或静默返回空数据；开放项与能力边界以
[`contracts/registries/open-items.yaml`](../../../contracts/registries/open-items.yaml) 为准：

- `OPEN-API-35`：MySQL 下 `records` 审计哈希链不提供（ADR-031 决策）；
- `OPEN-API-36`：MySQL 下 dashboard / LDAP Sync 通过
  `internal/bootstrap/unavailable_capabilities.go` 注入「能力不可用」实现，请求返回 503，
  LDAP Sync scheduler 不启动；SCIM 走 UoW，MySQL 可用；
- `OPEN-API-37`：MySQL 运行期能力边界以注册表 `OPEN-API-37` 条目为准；组合根按
  `DB_DRIVER` 选择 MySQL 运行时（`internal/bootstrap/runtime_factory.go` 的
  `newRuntimeMySQLWithDialect`），启动探针保留，不支持模块按 `OPEN-API-35/36`
  注入 503 降级实现；运维处置见 [runbook](../../operations/runbook.md)。

## 10. 健康探测与降级

| 项 | 规范 |
|---|---|
| 探测 | 按能力探针判定（凭据可读、端点可达、协议握手成功）；探测失败即标记 degraded 并告警 |
| 认证链路 | 认证依赖的渠道不可用 → fail-close（拒绝并 503），不得回退到弱校验 |
| 旁路链路 | 通知/同步等旁路渠道不可用 → 可降级并告警，由 Outbox 重试与死信兜底 |
| 观测 | 健康结果作为可观测指标输出；如需持久化观测缓存，表结构以声明式 Schema 为准，健康权威事实仍来自探测 |
| 禁止 | 不得以健康缓存代替授权/认证判定，也不得因渠道 degraded 放宽任何安全策略 |
