# 设计总纲（Design）

> 目标态规范「设计总纲（Design）」；写作约束见 [docs/README.md](../README.md)。

---

## 定位

`design/` 承载 GuliangIAM 的**目标态详细设计**——当从零构建系统时，每个子域应如何设计与实现。架构层基准见《architecture/system-overview.md》，数据模型见《architecture/data-model.md》，安全模型见《architecture/security-model.md》。

与 `adr/`（架构决策）、`registries/`（运行时事实注册表）的分工：

| 目录 | 内容 |
|------|------|
| `architecture/` | 架构层：系统概览、分层、数据模型、安全模型、多区域、可观测性 |
| `design/` | **本目录**：子域详细设计、协议规格、前端规范、基础设施组件 |
| `adr/` | 架构决策记录：为什么选 X 不选 Y |
| `registries/` | 运行时事实注册表：状态机、错误码、Outbox 参数、SLO、限流等 |

---

## 目录索引

### `auth/` — 认证与登录

| 文件 | 内容 |
|------|------|
| [login.md](auth/login.md) | 登录认证流程（用户名密码路径、Transport → App → Domain 调用链） |
| [mfa.md](auth/mfa.md) | MFA Challenge 状态机 + 生命周期参数（TTL、最大验证次数、过期策略） |
| [oauth2.md](auth/oauth2.md) | OAuth2 授权码流程（SP → IdP → code → token 时序） |
| [qrcode.md](auth/qrcode.md) | QR Code 登录时序（创建 → 扫码 → 确认 → 换取） |
| [saml.md](auth/saml.md) | SAML/CAS 协议规格（双角色 IdP/SP、SSO/SLO、元数据、票据模型） |
| [webauthn.md](auth/webauthn.md) | WebAuthn/Passkey 完整 ceremony（注册 14 项校验、认证 6 项、挑战持久化 + Liquid Auth/Passkey 登录替换设计） |
| [faceid.md](auth/faceid.md) | Face ID 活体检测登录（init/verify/complete 三步 Ceremony、Provider SPI、TTL 与尝试次数收敛、一次性消费、与 MFA/Session/Audit 集成、等保 2.0 三级对齐） |
| [network-identity.md](auth/network-identity.md) | 国家网络身份认证接入（规划态设计；NCI 领域端口、国密 TLS/数字信封/时钟自检、去标识化身份绑定、登录与实名分离、会话表与审计口径、前端入口；落地边界见该文件 §1） |

### `iam/` — 身份与访问管理域

| 文件 | 内容 |
|------|------|
| [users.md](iam/users.md) | 用户域（创建/更新/状态/身份绑定/批量导入） |
| [organizations.md](iam/organizations.md) | 组织域（创建/安全策略/套餐/域名联邦） |
| [applications.md](iam/applications.md) | 应用域（OAuth Client/SAML SP/Provider 绑定/密钥） |
| [roles.md](iam/roles.md) | 角色域（RBAC 绑定/批量导入、Casbin+ReBAC 求值分工） |
| [permissions.md](iam/permissions.md) | 权限域：权限标识/模型、Casbin Enforcer、跨租户 scope 授权与 RLS 边界 |

### `protocol/` — 协议规格

| 文件 | 内容 |
|------|------|
| [oauth2.md](protocol/oauth2.md) | OAuth2/OIDC 协议扩展（RFC 7009/7662 等 Token 语义、OIDC Discovery、授权码强制基线、ID Token claims、Back-Channel/RP-Initiated Logout 等集中存放；具体条目以文件章节为准，不在此处计数） |
| [scim.md](protocol/scim.md) | SCIM 2.0 完整设计（Schema 映射/filter/pagination/Bulk/ETag/出站 Client） |
| [ldap.md](protocol/ldap.md) | LDAP 接入与同步规格 |

### `commerce/` — 商业域

| 文件 | 内容 |
|------|------|
| [order.md](commerce/order.md) | 订单状态机（created → payment_pending → paid → completed → refunded/closed） |
| [payment.md](commerce/payment.md) | 支付域完整设计：ChannelGateway SPI、五阶段回调管线、并发退款控制、UNKNOWN 收敛、对账机制、渠道适配器、Schema 与独立菜单树分组 |
| [metering.md](commerce/metering.md) | 计量与配额管道（事件 → 聚合 → 配额扣减 → 计费导出） |

### `ai-gateway/` — AI 网关域

| 文件 | 内容 |
|------|------|
| [README.md](ai-gateway/README.md) | **AI 网关完整设计**：三协议面（OpenAI + Anthropic + Gemini）+ Canonical 内部规范 + 路由引擎 + 熔断健康 + API Key + 配额治理 + Schema 表 + 独立菜单树分组 |

### `infrastructure/` — 基础设施组件

| 文件 | 内容 |
|------|------|
| [outbox.md](infrastructure/outbox.md) | Outbox 一致性投递（Worker/状态机/指数退避/死信/统一幂等） |
| [audit.md](infrastructure/audit.md) | 审计哈希证据链（链列/锚点/验证算法/断链处置） |
| [integration.md](infrastructure/integration.md) | 外部系统接口适配（IdP/通知/存储/支付 + SSE 实时事件流） |
| [providers.md](infrastructure/providers.md) | 统一 Provider 注册与配置中心（身份/AI/支付/通知四族收敛、双轨边界、生命周期、密钥与迁移路径） |
| [agent.md](infrastructure/agent.md) | Agent 运行时与注册中心 |

### `frontend/` — 控制台（前端）规范

| 文件 | 内容 |
|------|------|
| [README.md](frontend/README.md) | 前端设计总纲 + 技术栈 + 组件目录结构 + 设计系统（内联）+ a11y（内联） |
| [interaction.md](frontend/interaction.md) | 页面设计规范 + 响应式 + 字段显示规范 + 交互简化 + CrudListPage 列表页抽象 |
| [security.md](frontend/security.md) | 前端安全（Cookie/CSRF/SameSite 矩阵/DPoP/令牌绑定） |
| [registry.md](frontend/registry.md) | 路由/页面/菜单注册表（生成物，禁手改；`scripts/gen_frontend_registry.py`） |
| [testing.md](frontend/testing.md) | 测试规范（Vitest + RTL + Playwright E2E） |
| [workbenches.md](frontend/workbenches.md) | IGA/PAM、Risk、WLI 工作台的页面范围、操作面与验收锚点 |
| [casdoor-parity-checklist.md](frontend/casdoor-parity-checklist.md) | Casdoor 管理页 → GLIAM 对齐决策台账（KEEP/ADD/ADAPT/REJECT/NO_MATCH） |

> 原 design-system.md / navigation.md / data-layer.md / performance.md / a11y.md / i18n.md 子文档中，design-system 与 a11y 已内联至 frontend/README.md；navigation / data-layer / performance / i18n 内容亦覆盖在 frontend/README.md 与 architecture/technology-selection.md 中，故不再作为独立条目。

### 治理域（rebac / iga / risk / b2b / workload / actions）

| 文件 | 内容 |
|------|------|
| [rebac/README.md](rebac/README.md) | ReBAC 关系元组授权（命名空间、元组、Zookie 一致性、Why 溯源、求值流程） |
| [iga/README.md](iga/README.md) | IGA/PAM（访问活动、SoD、JIT 提权与到期回收） |
| [risk/README.md](risk/README.md) | 身份风险与持续访问评估（信号、分级、会话治理、申诉） |
| [b2b/README.md](b2b/README.md) | 多级组织树、策略继承、域名认领与 SSO 发现 |
| [workload/README.md](workload/README.md) | Workload 身份（身份池、OIDC 联邦信任、Service Account） |
| [actions/README.md](actions/README.md) | Identity Actions 沙箱与执行管控 |

### 合规域（privacy）

| 文件 | 内容 |
|------|------|
| [privacy/README.md](privacy/README.md) | 隐私与数据主体权利（DSAR 六类请求、SLA、同意版本、可携导出、擦除边界） |

---

## 跨文档引用约定

- 协议域集中于《protocol/oauth2.md》，auth/ 下各文件的「协议域声明」节（位于文件尾部）指向 design/protocol/oauth2.md
- 跨域引用使用《文档名》§N.M 锚点格式，如《architecture/data-model.md》§3.2
- 同一事实只在唯一位置定义，禁止重复定义

---

旧锚点映射见 `scripts/check-doc-anchors.py`。
