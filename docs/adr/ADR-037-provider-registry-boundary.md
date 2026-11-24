# ADR-037: Provider 双轨边界与统一注册入口封口

- 状态：[Current] — 统一 `/providers` 不承载「由专用子系统托管」的 identity kind；LDAP、验证码族、Face ID 族的创建与管理只由各自专用控制台负责。后端 422 契约（`delegatedIdentityKinds`）与注册表 `managed_via` / `consumer_table` 已随 provider 封口实现（P1）合并；前端创建/编辑收敛、列表筛选与历史行保留已落库。
- 日期：2026-11-03

## 背景

统一 Provider 注册中心把身份、AI、支付、通知四族收敛为「`providers` 注册层 + 族内明细表」；身份族的 LDAP、验证码、Face ID 在运行期分别由 `ldaps`、`captcha_providers`、`face_id_providers` 专用表承载，登录、验证码校验与 Face ID 验活链路直接读取专用表。设计文档已把这些子系统列为统一收敛范围之外。

实现仍允许统一 `/providers` 创建这些 kind：写入落在 `providers` 注册层，但没有任何运行期消费者读取该行；专用控制台又各自维护同一业务对象的另一行。结果是「统一控制台可以配置，但配置不生效」，同时用户会误以为统一入口与专用控制台管理的是同一对象。

本决策需要同时满足：

1. 不改变 LDAP 登录、验证码校验、Face ID 验活的运行期读取路径。
2. 不删除既有 kind 闭集；历史统一行仍可读、可停用、可删除。
3. 让统一入口在契约上明确拒绝托管 kind，而不是静默写入无效行。
4. 用注册表而不是 Go 常量或文档副本维护 kind → 归属。

## 决策

### D1. 统一 Provider 中心不承载托管 kind

统一 Provider 中心（`/providers`）不承载「由专用子系统托管」的 kind。以下 kind 的创建与管理只由对应专用控制台负责：

| kind | 专用控制台 | 运行期消费表 |
| --- | --- | --- |
| `ldap` | `/api/v1/ldaps` | `ldaps` |
| `recaptcha`、`recaptcha_v3`、`hcaptcha`、`turnstile`、`geetest`、`aliyun_captcha` | `/api/v1/captcha-providers` | `captcha_providers` |
| `faceid`、`aliyun_face_body` | `/api/v1/face-id-providers` | `face_id_providers` |

专用控制台是这些 kind 的唯一创建、编辑、激活、密钥轮换与连接测试入口。统一入口只保留读取、停用与删除。

### D2. 统一入口的 422 契约

统一入口对托管 kind 的 create、kind/providerType 变更、activate、rotate secret（含 PATCH 中携带 secret 字段的路径）与 test-connection 返回 422。

- 元数据固定为 `field=kind`、`value=<kind>`、`managedBy=<专用控制台路径>`。
- list、get、disable、delete 保留；delete 仍受「先停用再删除」保护：`active` 或仍可路由的行拒绝删除。
- 非 kind 的既有字段编辑不扩大拒绝面（重命名等仍按原语义处理）。

### D3. kind → 归属的单一事实源

kind → 归属登记在 `contracts/registries/provider-kinds.yaml` 的 `managed_via` 与 `consumer_table`。Go 侧拒绝集合 `internal/app/providers/delegated.go` 的 `delegatedIdentityKinds` 只保存 kind → 控制台路径；`scripts/check-provider-kinds.py` 校验注册表字段、Go 拒绝集合与运行时 pending 路由表一一对应。任一缺项、多项或归属不一致即门禁失败；禁止在 Go、YAML 或文档中各写一份副本。

`managed_via` / `consumer_table` 是由 provider 封口实现（P1）引入的注册表字段，单一事实源地位由 `scripts/check-provider-kinds.py` 固定；本 ADR 不另写归属副本。

### D4. 前端面

统一控制台的创建/编辑表单不提供托管 kind；列表筛选与历史行展示保留这些类目，历史行仍可查看、停用、删除。前端清单落点 `web/src/shared/hooks/useProviderCategories.ts`（`MANAGED_PROVIDER_KINDS`、`CREATABLE_PROVIDER_CATEGORIES`、`FILTERABLE_PROVIDER_CATEGORIES`），与注册表 `managed_via` 对齐；后端拒绝集合由 P1 引入。专用控制台继续作为 LDAP、验证码、Face ID 的唯一创建入口。

### D5. 取舍

| 决策 | 内容 | 理由 |
| --- | --- | --- |
| **KEEP** | 既有专用控制台、专用表与登录链路不变；Schema 闭集不删；历史统一行不迁移 | 这些子系统不在统一收敛范围；改读或迁移会触碰认证关键路径与外键历史数据 |
| **ADD** | 统一入口 422 契约；注册表 `managed_via` / `consumer_table`；门禁一一对应校验 | 让边界可执行、可发现、可回归，而不是只写在文档 |
| **ADAPT** | 统一入口对托管 kind 降级为「只读 / 停用」面；创建/编辑表单移除托管 kind | 统一控制台不再提供会写入无效行的入口，历史行仍可处置 |
| **REJECT** | 用统一 `providers` 表承载托管 kind（运行期改读统一表或写入时双写）；删除历史行或收缩 Schema 闭集 | 统一表缺少 LDAP 的 `base_dn` / `filter` / 属性列，验证码密钥来自环境工厂而非统一行，双写无法兑现「配了生效」；删除历史行会破坏闭集读取与审计追溯 |
| **NO_MATCH** | Casdoor 以单一 Provider 页面覆盖 LDAP、验证码、Face ID；GuliangIAM 使用专用表 + 专用控制台，两者不是同一模型，不做页面级对齐 | 按决策备忘的 MO-C 结论：保留双轨，统一入口只封口，不追求 Casdoor 结构同构 |

## what / why / how / trade_off

| 维度 | 内容 |
| --- | --- |
| **what** | 统一 `/providers` 不承载 LDAP、验证码族、Face ID 族；这些 kind 只由专用控制台创建与管理。统一入口对其 create、kind 变更、activate、rotate secret、test-connection 返回 422，并保留 list / get / disable / delete。 |
| **why** | 现行统一行没有运行期消费者，属于「可配置但不生效」的实现漂移；封口把实现拉回既有设计边界，零鉴权链路风险、零数据迁移。 |
| **how** | Go 侧新增 `delegatedIdentityKinds` 与四处生命周期拒绝点；注册表新增 `managed_via` / `consumer_table`；`scripts/check-provider-kinds.py` 做一一对应校验；前端创建/编辑选项过滤托管 kind，筛选与历史行保留。 |
| **trade_off** | 统一 Provider 中心不覆盖这些 kind，习惯统一入口的客户端会收到 422；历史统一行仍是惰性数据，是否清理需产品另行裁决；注册表字段要求 Go 与 YAML 同步演进，靠门禁而不是人工记性。 |

## 理由与后果

- 正面：不触碰 LDAP 登录、验证码校验、Face ID 验活与专用表结构；不新增 Schema 迁移；统一入口不再产生无效注册行；422 元数据把用户导向正确的专用控制台；历史行保留可读与可停用/删除路径。
- 负面：统一控制台的能力边界变窄；已有 API 客户端若仍向统一入口创建托管 kind，会收到 422；历史统一行继续存在但不参与运行期，需要产品决定是否清理。
- 约束：不得以「统一注册」为名删除 `ldaps`、`captcha_providers`、`face_id_providers`；不得让专用登录链路改读统一 `providers` 行；不得为了通过门禁在 Go 或 YAML 中放宽归属集合；不得为托管 kind 新增统一注册层字段或迁移。

## 替代方案

| 方案 | 内容 | 否决/暂缓理由 |
| --- | --- | --- |
| **KEEP：维持现状** | 统一入口继续接受托管 kind，只改文档说明 | 统一行仍无运行期消费者，用户仍会配置到不生效的对象；实现漂移继续存在 |
| **MO-A：运行期改读统一表** | LDAP 登录、验证码校验、Face ID 验活改读 `providers` + `provider_configs` | LDAP 缺 `base_dn` / `filter` / 属性列；验证码密钥模型是环境工厂；Face ID 类型命名与历史 `user_identities.provider_id` 需要迁移；触碰认证关键路径，风险不成比例 |
| **MO-B：统一表写入时双写专用表** | `createNative` / Patch / Rotate / Delete 扩展 identity native 分支，同事务写三张专用表 | 双写漂移、唯一名冲突、状态语义分叉；验证码 `secret_key` 仍不被工厂消费，交付的是「双写成功但不生效」 |
| **MO-C：保留双轨 + 统一入口封口（本 ADR）** | 专用控制台继续唯一管理；统一入口拒绝 create/kind 变更/activate/rotate/test，保留 list/get/disable/delete | 采纳：零数据迁移、零鉴权链路改动，把实现拉回既有设计边界 |
| **REJECT：删除历史行或收缩闭集** | 迁移删除统一表中的托管 kind 行，或从 Schema 闭集移除 kind | 破坏历史读取与审计追溯；需要数据迁移与回滚设计，且没有产品承诺 |

## 与其它 ADR 的关系

- **ADR-022**：本 ADR 不改 `contracts/schema/*.yaml`、不新增迁移；`providers` 与三张专用表的既有结构、RLS 与闭集保持。Schema 闭集不因统一入口封口而收缩。
- **ADR-033**：Face ID Provider SPI 仍在 App 层以函数变量注入；本 ADR 只改管理入口边界，不改 SPI 位置或调用链。
- **ADR-035**：支付域 `ChannelGateway` SPI 不受影响；支付 Provider 仍走统一注册层与支付族明细表。
- **ADR-016 / ADR-036**：专用表与统一表继续按既有 RLS 策略隔离；本 ADR 不引入新的租户旁路，也不改可信租户解析。

## 验证证据

以下 Current 证据用于锁定决策所依据的现行事实（核验日期 2026-11-03）：

| 检查点 | 命令 | 当前结论 |
| --- | --- | --- |
| 专用控制台存在 | `rg -n 'Pattern:.*"/api/v1/(ldaps\|captcha-providers\|face-id-providers)"' internal/transport/http/api/v1` | 命中三组专用路由声明 |
| 专用表存在 | `rg -n 'table_name: (ldaps\|captcha_providers\|face_id_providers)' contracts/schema` | 命中三张专用表声明 |
| 前端创建/编辑已收敛 | `rg -n "MANAGED_PROVIDER_KINDS\|creatable: false\|CREATABLE_PROVIDER_CATEGORIES\|FILTERABLE_PROVIDER_CATEGORIES" web/src/shared/hooks/useProviderCategories.ts` | 命中托管 kind 清单、创建闭集与筛选闭集 |
| 闭集仍保留托管 kind（KEEP 证据） | `rg -n "ldap\|recaptcha\|faceid" internal/app/providers/kind.go` | 命中 `identityKinds` 中的托管 kind；闭集未收缩，历史行仍可读 |
| 后端拒绝面 | `rg -n "delegatedIdentityKinds\|rejectDelegated\|managedBy" internal/app/providers` | 命中 kind → 控制台映射与 422 元数据写入 |
| 注册表归属 | `rg -n "managed_via\|consumer_table" contracts/registries/provider-kinds.yaml` | 每个托管 kind 均带 `managed_via` / `consumer_table` |
| 生命周期用例 | `go test ./internal/app/providers -run 'TestUnified.*Delegated\|TestDelegatedIdentityKindsDeclaration'` | PASS：create、kind 变更、activate、rotate secret、test-connection 拒绝；disable / delete / 读取保留 |
| 门禁 | `python3 scripts/check-provider-kinds.py` | PASS：注册表、Go 拒绝集合与运行时 pending 路由一致 |

完整回归：`go test ./internal/app/providers`、`python3 scripts/check-provider-kinds.py`、`python3 scripts/gen_registry_md.py --check`、`pnpm --dir web test --run`。本 ADR 不新增 Schema、迁移或历史行删除；`providers` 闭集与三张专用表结构保持不变。

## 复审条件

出现以下任一情况时复审本 ADR：

1. 产品决定把托管 kind 收敛到统一注册层 → 先立新 ADR，并完成 Schema、认证链路、外键历史数据与回滚设计，不得直接改读统一表。
2. 产品决定清理历史统一行 → 先给出数据迁移、审计追溯与回滚方案，再删除行或收缩闭集。
3. 新增托管子系统或新增 kind → 同步更新 `provider-kinds.yaml`、Go 拒绝集合、门禁与前端清单；三处不一致时以门禁失败为准，不接受人工豁免。
4. 专用控制台的运行期消费表或认证链路变更 → 更新 `consumer_table` 与本文验证证据，必要时另立 ADR。
