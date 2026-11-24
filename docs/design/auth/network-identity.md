# 国家网络身份认证接入设计

> 设计锚点：[DESIGN:auth/network-identity]

> 目标态规范「国家网络身份认证接入」；写作约束见 [docs/README.md](../../README.md)。

---

## 1. 定位、规划态与边界

国家网络身份认证（下称 NCI）作为身份提供方接入，承担两类互斥职责：

| 职责 | 入口 | 结果 |
|------|------|------|
| 登录 | 登录页 NCI 方法 | 建立会话，签发与密码登录同级的会话 Cookie |
| 实名核验 | 账户安全页实名入口 | 只写实名标记，不建立会话、不下发 Cookie 或令牌 |

NCI 网关返回的**去标识化网络身份应用标识**是平台可持有的唯一身份材料。平台不接收、不存储、不转发姓名、证件号、人脸图像或任何可还原自然人的明文数据。

**规划态**：NCI 接入以外部准入为前提，本文定义目标态端口契约、适配器边界与验收口径。运行期启用依赖外部 NCI 网关准入、机构审批与联调材料（网关地址、SP 标识、国密双证书、测试回调通道）；外部依赖不改变本文的 fail-close 与去标识化约束。适配器、`contracts/registries/provider-kinds.yaml` 条目、声明式 Schema 与验收测试在同一变更交付；在外部依赖满足前，NCI 不进入运行期路由与注册表，管理面不得暴露不可解析的类型。

**红线**：NCI 为规划态；未获外部依赖与同批批准前，禁止在 `internal/`、`web/src/`、`db/`、`contracts/schema/`、`openapi/`、`contracts/registries/` 落地任何端口、适配器、表、路由、注册表条目或前端入口。

NCI 是独立认证方法，与密码登录并列展示。网关或加密链路失败时，NCI 方法停留在不可用状态，**不得自动回退密码登录**，也不得复用上一次成功结果；失败姿态见 §5.3。

## 2. 领域模型与端口

### 2.1 领域端口

端口定义在 `internal/domain/networkidentity`，不依赖 `app/infra/transport`：

```go
type NetworkIdentityProvider interface {
	InitiateAuth(ctx context.Context, request InitiateRequest) (InitiateResult, error)
	VerifyAuth(ctx context.Context, request VerifyRequest) (VerifyResult, error)
}

type AuthMode string

const (
	AuthModeLogin    AuthMode = "login"
	AuthModeRealName AuthMode = "real_name"
)

type InitiateRequest struct {
	OrganizationID string
	ApplicationID  string
	BizSeq         string   // 32 位十六进制，服务端生成，128 bit 熵
	Mode           AuthMode // login | real_name
}

type InitiateResult struct {
	RedirectURL string
	BizSeq      string
	ExpiresAt   time.Time
}

type VerifyRequest struct {
	BizSeq string
	Payload []byte // 网关回调原文，仅由 infra 适配器解释
}

type VerifyResult struct {
	SubjectRef  string // 去标识化网络身份应用标识
	Verified    bool
	GatewayCode string
}
```

约束：

1. `OrganizationID`/`ApplicationID`/`BizSeq` 由服务端从已认证上下文与会话行生成，**不接受**前端或回调参数直接提供。
2. `BizSeq` 长度固定 32 位十六进制，一次性消费；服务端保存的绑定关系是唯一可信来源。
3. `VerifyResult` 只包含 `SubjectRef`、`Verified`、`GatewayCode` 三个字段，不携带姓名、证件号、人脸或原始报文。
4. 适配器无状态：`InitiateAuth`/`VerifyAuth` 不保存登录或实名上下文；模式、组织、应用来自 App 注入的一次性绑定行，适配器不得跨请求复用上一次成功结果。

### 2.2 身份类型

`user_identities.identity_type` 的目标值增加 `network_identity`。该类型的 `Subject` 存去标识化网络身份应用标识；同一组织内同一标识唯一，复用既有 `uq_identity_provider_subject` 约束收敛重复绑定。`network_identity` 与适配器、`contracts/registries/provider-kinds.yaml` 条目在同一变更进入运行期闭集，管理面不得提前暴露无法解析的类型。

## 3. 基础设施适配器边界

`internal/infra/identity/ncidas` 是唯一与 NCI 网关通信的组件，职责边界如下：

| 维度 | 目标态约束 |
|------|------------|
| 传输 | 国密 TLS 双证书双向认证；客户端证书与服务端 CA 校验均不可关闭；`gm_tls` 能力位不可用时 NCI 拒绝启用，不降级标准 TLS |
| 数字信封 | 每个请求动态生成 SM4 会话密钥；SM2 公钥加密会话密钥，密文按 **C1C3C2** 排序；业务报文使用 SM4-ECB + PKCS#5 填充（实现按 PKCS#7 填充）；会话密钥单次使用、只驻留内存 |
| 响应解封 | 使用平台 SM2 私钥解封会话密钥，校验 PKCS#5 填充（实现按 PKCS#7 填充）后再解业务报文；填充或摘要校验失败一律拒绝 |
| 时钟 | 适配器启用前执行一次网关与本地时钟自检，并在每次 `InitiateAuth` 前复核；偏移绝对值 > **3 秒** 即返回 `unavailable`，不发起业务请求 |
| 失败分类 | TLS、信封、时钟、网络、网关 5xx 统一映射为结构化 `unavailable`；本地请求字段缺失才映射 `invalid_argument`；业务拒绝由 `Verified=false` + `GatewayCode` 表达 |
| 密钥生命周期 | 私钥与证书经 KMS/Vault 引用加载；密钥、密文、回调原文不写日志；不缓存上一次成功结果 |
| 回退 | 禁止降级到标准 TLS、明文 HTTP、软件模拟证书或密码登录；禁止缓存上一次成功结果 |

`VerifyAuth` 的 `Verified=false` 是业务结果而非错误：调用方据此拒绝登录或拒绝实名标记，同时记录 `GatewayCode`。

国密原语的能力位、缺失策略与传输层降级边界统一由 `contracts/registries/crypto.yaml` 与《[security-model](../../architecture/security-model.md)》§2.21 定义；本文不复制能力位清单。`gm_tls` 的启用开关不等于可用：签名证书、加密证书、对应私钥、SM2 公钥与 SM2-SM3 签名、私钥匹配、有效期与 CA 链全部通过材料自检后才可用；未通过只降级，不影响 SM2/SM3/SM4 业务密码学。

**填充口径**：SM4 分组长度为 16 字节，PKCS#5 与 PKCS#7 在该分组长度下的填充字节布局等价；需求原文保留 PKCS#5，实现按 PKCS#7 填充，本文三处表述以此为准。

## 4. 持久化模型

### 4.1 `network_identity_sessions`

目标表 `guliangiam.network_identity_sessions` 保存发起与回调的一次性绑定关系。表、列、约束与索引以声明式 Schema `contracts/schema/20_identity.yaml` 为唯一事实源，随 NCI 适配器同批引入；本文只定义语义，不复制列清单。

约束：

- 一行对应一次发起，承载去标识化网络身份应用标识、组织、应用、模式、核验状态与一次性 `biz_seq`；`biz_seq` 有效期上界 5 分钟。
- RLS 启用，租户隔离谓词按 `organization_id = current_organization_id()`；跨租户读写与「资源不存在」不可区分。
- 唯一约束保证同一组织内 `biz_seq` 不重复；身份绑定的重复收敛复用 `uq_identity_provider_subject`。
- **禁止**出现姓名、证件号、手机号、人脸图像、原始报文或可还原自然人的列；`subject_ref` 只存去标识化网络身份应用标识。
- 生命周期登记在 `contracts/registries/lifecycle.yaml`；会话行在 `expires_at` 后由清理任务删除；实名标记的保存期跟随用户身份记录。

### 4.2 实名标记

实名模式核验成功后，只更新用户身份绑定记录的 `subject_ref` 与核验时间戳，并记录核验来源。实名标记不改变 `user_identities` 的唯一键语义；同一 `subject_ref` 重复核验按 §5.2 的幂等语义收敛，不重复写标记。

## 5. 流程设计

登录与实名共用同一 `NetworkIdentityProvider` 适配器，但两条流程的绑定行、写入目标与会话副作用互不共享：

| 维度 | `mode=login` | `mode=real_name` |
|------|--------------|------------------|
| 入口 | 登录页 | 账户安全页 |
| 核验成功写入 | 身份绑定 + 安全会话 | 仅实名标记 |
| 会话/Cookie/令牌 | 建立并下发 | 不建立、不下发 |
| `biz_seq` | 一次性消费 | 一次性消费 |
| 幂等 | 同一标识收敛为一条身份记录、一个会话 | 收敛为一次标记 |

### 5.1 登录流程

1. 登录页选择 NCI 方法，前端向应用服务发起登录初始化。
2. App 层校验组织/应用已启用 NCI、生成 32 位 `biz_seq`，写入 `mode='login'` 的 pending 行。
3. App 层调用 `InitiateAuth`，将网关授权地址返回前端；前端只做跳转，不接触证书、密钥或 `biz_seq` 之外的绑定信息。
4. 网关回调进入协议回调端点；服务端按 `biz_seq` 查行并校验 `org_id`/`app_id`/`mode` 一致，随后调用 `VerifyAuth`。
5. `Verified=true` 时绑定/复用 `identity_type='network_identity'` 的用户身份，消费会话行，建立安全会话；会话 `acr` 为 `ncidas`，`amr` 为 `network_identity`。
6. `Verified=false`、`biz_seq` 过期或已消费时拒绝登录，按 AC-NCIDAS-001/004 返回确定错误码。

### 5.2 实名流程

1. 账户安全页发起实名核验，App 层生成 `mode='real_name'` 的 pending 行并调用 `InitiateAuth`。
2. 回调校验绑定关系后调用 `VerifyAuth`。
3. `Verified=true` 时只写实名标记并审计；**不建立会话、不签发 Cookie/令牌、不改变登录态**。
4. 同一 `subject_ref` 重复或并发核验按 §4.2 幂等收敛：返回已核验状态，不重复写标记、不重复建立身份绑定。

### 5.3 失败处置

| 场景 | 返回 | 副作用 |
|------|------|--------|
| 网关不可达 / GM TLS 失败 / 时钟偏移 >3 秒 | 503 `unavailable` | 无会话、无令牌、无实名标记 |
| 数字信封或 PKCS#5 填充（实现按 PKCS#7 填充）校验失败 | 503 `unavailable` | 同上；不记录密文或密钥 |
| `biz_seq` 不存在、跨组织或跨应用 | 404 `not_found` | 与资源不存在不可区分 |
| `biz_seq` 已消费或并发重放 | 409 `conflict` | 不产生第二个会话或第二次标记 |
| `Verified=false` | 401 `unauthorized`（与凭证错误形态一致，防枚举） | 不建立会话、不写实名标记 |

任何失败路径都不得自动提交密码表单、不得复用缓存结果、不得降级到非国密传输。

## 6. 审计口径

NCI 审计事件走统一审计写入器；除统一记录字段外，NCI 相关载荷只追加以下四项（`records` 行结构以 `contracts/schema/60_audit.yaml` 为准）：

| 字段 | 内容 |
|------|------|
| `biz_seq` | 本次一次性绑定标识 |
| `subject_ref` | 去标识化网络身份应用标识（核验成功时） |
| `verify_result` | `verified` / `rejected` / `expired` |
| `gateway_code` | 网关响应码（不记录原始报文） |

审计不写姓名、证件号、手机号、人脸、证书、会话密钥或密文。登录成功、登录失败、实名成功、实名失败、跨租户拒绝与重放拒绝均落审计；拒绝与越权同样留痕。

## 7. 前端入口

| 入口 | 事实源 | 约束 |
|------|--------|------|
| 登录页 NCI 方法 | `applications.signin_items` + 已启用的 `network_identity` 提供商行 | 未配置时隐藏方法；禁用条件上收为统一的 `disabled` 判定，不在渲染层按 UA/OS 判断 |
| 提供商配置卡片 | `providers` 行（`kind=network_identity`）与组织/应用绑定 | 网关地址、SP 标识、证书/私钥引用、登录/实名开关；密钥只引用 KMS/Vault，不回显明文 |
| 实名入口 | 账户安全页 + 用户身份绑定状态 | 只展示已核验/未核验与去标识化标识，不展示姓名、证件号或人脸 |

失败状态：NCI 方法保持选中并展示 `unavailable` 提示；页面可并列展示其他已启用登录方法，但**不得自动切换或自动提交密码**。

## 8. 配置与事实源

| 配置面 | 唯一事实源 | 本文约定 |
|--------|------------|----------|
| 加密能力位、缺失策略、`default`/`gm` 档位 | `contracts/registries/crypto.yaml`（渲染层 `docs/generated/crypto-registry.md`，生成命令 `python3 scripts/gen_registry_md.py`） | 只引用能力位 ID 与 `failure_mode`，不复写能力位清单 |
| 国密档位开关与 TLS 材料变量 | `contracts/registries/env.yaml` | 只引用变量登记，不复写变量名、默认值或材料路径 |
| 错误码 | `contracts/registries/errors.yaml` | `unavailable` / `not_found` / `conflict` / `unauthorized` 只引用码名，不新增未登记码 |
| 提供商行、登录方法可见性、身份类型与会话表 | `contracts/schema/20_identity.yaml` | 结构只改声明层，随适配器同批生成迁移 |
| 身份提供商 kind 闭集 | `contracts/registries/provider-kinds.yaml` | `network_identity` 与适配器、消费方同批进入闭集 |
| 审计行结构 | `contracts/schema/60_audit.yaml` | 本文 §6 只定义 NCI 载荷语义 |
| 会话生命周期 | `contracts/registries/lifecycle.yaml` | 会话行清理与实名标记保存期按注册表执行 |

## 9. 安全不变量

1. 去标识化：`SubjectRef` 是唯一身份材料；禁止姓名、证件号、手机号、人脸图像与原始回调报文（§2、§6）。
2. 绑定可信：`OrganizationID`/`ApplicationID`/`BizSeq` 只来自服务端上下文与会话行；跨组织、跨应用不得复用同一绑定行（§2.1、§4）。
3. fail-close：国密 TLS、数字信封、时钟自检任一失败即 `unavailable`；禁止降级到标准 TLS/明文 HTTP/密码登录，禁止缓存回退（§3、§5.3）。
4. 职责分离：实名路径永不建立会话或签发令牌；登录与实名共用适配器但状态互不污染（§5）。
5. 同批交付：适配器、`provider-kinds.yaml` 条目、声明式 Schema 与验收测试同一变更引入；管理面不得提前暴露不可解析类型（§1、§8）。
6. 结构只改声明层：`network_identity_sessions` 与身份类型闭集变更通过 `contracts/schema/*.yaml` 生成迁移，禁止手写 DDL。

## 10. 落地顺序、外部依赖与验收映射

| 阶段 | 范围 | 同批约束 |
|------|------|----------|
| 建模 | 领域端口、`identity_type=network_identity` 目标值、`network_identity_sessions` 声明与迁移 | 不引入运行期注册表条目；不暴露管理面类型 |
| 适配器 | `internal/infra/identity/ncidas`、国密 TLS/信封/时钟自检、结构化失败、审计接入 | 适配器、`provider-kinds.yaml` 条目、Schema 迁移与验收测试同批合并；缺一不得上线 |
| 前端 | 登录页方法、提供商配置卡片、实名入口、失败态与 i18n | 方法与配置项从注册表/配置读取，不在组件内硬编码类型或字段清单 |

外部强依赖：

- 外部 NCI 网关的生产准入、测试环境、回调白名单与联调窗口。
- 机构/监管审批与网络身份应用标识（SP）登记。
- 国密双证书、私钥与受信 CA 材料（KMS/Vault 引用）。
- 平台侧适配器、Schema、provider kind、审计与验收测试同批交付。

任一外部依赖未满足时，NCI 保持不可用并停留在本方法，不回退密码登录；不得以软件模拟证书、标准 TLS 或缓存结果替代。

### 10.1 验收映射

| AC 编号 | 设计条款 | 验证锚点 |
|---------|----------|----------|
| AC-NCIDAS-001 | §3 失败分类、§5.3 | 错误码 `unavailable`（`contracts/registries/errors.yaml`）；同批交付测试（当前不存在）：`internal/infra/identity/ncidas/*_test.go`（失败注入） |
| AC-NCIDAS-002 | §2.1、§4.1、§6 | `contracts/schema/20_identity.yaml`、`contracts/schema/60_audit.yaml`（现有声明不含 `network_identity_sessions`，目标表随适配器同批引入）；同批交付测试（当前不存在）：`internal/infra/identity/ncidas/*_test.go`（载荷断言） |
| AC-NCIDAS-003 | §3 时钟 | `gm_tls`（`contracts/registries/crypto.yaml`；通用材料自检见 `internal/platform/crypto/gm_tls_test.go`，仅证明通用能力）；同批交付测试（当前不存在）：`internal/infra/identity/ncidas/*_test.go`（时钟自检） |
| AC-NCIDAS-004 | §2.2、§4.2、§5 | `uq_identity_provider_subject`（`contracts/schema/20_identity.yaml`；仅证明通用身份绑定唯一性）；同批交付测试（当前不存在）：`internal/app/networkidentity/*_test.go`（并发重放） |
| AC-NCIDAS-005 | §5 流程隔离 | 同批交付测试（当前不存在）：`internal/app/networkidentity/*_test.go`（登录/实名分离）；失败路径复用 AC-NCIDAS-001 |
| AC-NCIDAS-006 | §5.1 登录流程、§4.1 `network_identity_sessions` | 同批交付测试（当前不存在）：会话建立、`biz_seq` 一次性消费与单会话计数 |
| AC-NCIDAS-007 | §5.3 失败处置、§4.1 绑定行 | 同批交付测试（当前不存在）：租户与应用绑定、404 防枚举与响应形态对比 |
| AC-NCIDAS-008 | §6 审计口径 | 同批交付测试（当前不存在）：审计字段最小化断言 |

> 规划态备注：NCI 适配器、`network_identity_sessions` 声明、`provider-kinds.yaml` 条目与上表「同批交付测试（当前不存在）」随外部依赖满足同批引入；Schema、注册表、`gm_tls` 与设计条款锚点只能证明通用能力，不能证明 NCI 已实现。

验收条件见 [acceptance-criteria.md](../../acceptance-criteria.md) 的 `AC-NCIDAS-001` ~ `AC-NCIDAS-008`；登录页集成位置见 [login.md](login.md) 的「国家网络身份认证」小节。
