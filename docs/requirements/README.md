# 需求总纲

> 目标态规范「需求总纲」；写作约束见 [docs/README.md](../README.md)。

## 1. 引言

GuliangIAM 是面向企业级场景的统一身份认证与访问管理（IAM）平台，覆盖身份、认证、授权、协议接入、网关与集成能力。本套文档按「全新建设、目标最优方案」描述目标态需求与设计，不继承任何既有实现的架构约束。

### 1.1 术语与缩写

术语表唯一登记于《规范手册》附录 E。跨文档引用时直接使用术语名称。

### 1.2 参考文献

RFC 与标准引用按协议分布登记在 [design/protocol/](../design/protocol/) 与 [design/auth/](../design/auth/) 的对应文档（oauth2.md、scim.md、ldap.md、saml.md）。

---

## 2. 总体描述

### 2.1 产品拓扑

GuliangIAM 在系统拓扑中处于身份认证的核心枢纽位置。与外部系统的关系：

| 外部系统 | 交互方式 | 用途 |
|----------|---------|------|
| 第三方 IdP（原生构造器 + Goth Provider） | OAuth 2.0 / SAML | 社交登录、企业 SSO |
| LDAP/AD | LDAP/LDAPS | 目录同步、认证 |
| 支付渠道 | HTTPS API | 订单支付 |
| 存储适配器 | S3 API | 文件上传 |
| 通知渠道 | SMTP/API | 验证码、告警通知 |
| Kubernetes | K8s API | 容器化部署 |

### 2.2 用户特征

| 画像 | 职责 | 核心诉求 |
|------|------|---------|
| 超级管理员 | 全局运维与配置 | 统一租户管理、安全策略、审计追踪 |
| 组织管理员 | 本组织用户与应用 | 独立管理、细粒度权限、自定义登录体验 |
| 普通用户 | 使用应用服务 | 便捷登录、MFA 保护、自助密码管理 |
| 开发者 | 集成 IAM 到自有应用 | 标准协议、清晰 API、便捷 SDK |

### 2.3 假设与依赖

- 部署环境为 Linux 服务器、Docker 容器化或 Kubernetes 集群
- PostgreSQL 为唯一运行期数据库；MySQL 8.4+ / MariaDB 12.3+ 仅用于 DDL 与仓储兼容性验证（ADR-023、`OPEN-API-37`）
- 网络环境支持 HTTPS (TLS 1.3)
- 外部 IdP 服务可用且网络可达

---

## 3. 业务需求

### 3.1 业务背景

企业数字化转型中 IAM 是核心基础设施。GuliangIAM 以多租户、多协议、可审计、默认安全为第一性原则设计，避免把历史架构约束带入目标方案。

### 3.2 业务目标

| 目标 | 量化指标 |
|------|---------|
| 完整的多协议 IAM 能力 | 认证/授权/协议/网关能力全覆盖（清单见功能需求） |
| 企业级安全合规 | PKCE/DPoP/Token Rotation/密码历史/过期策略 |
| 亚秒级认证响应 | P99 < 500ms（不含外部 IdP 调用） |
| 多租户数据隔离 | 组织间数据零交叉，权限策略域隔离 |
| 高可用部署 | Docker/K8s 部署，优雅关闭，0 宕机更新 |

### 3.3 项目范围与边界

**包含的功能模块**：认证（密码/OAuth/SAML/CAS/WebAuthn/Passkey/MFA/QR Code/Face ID/Device Flow）、组织（多租户隔离/安全策略）、用户（CRUD/身份/凭证/MFA/角色/组）、应用（OAuth 客户端/Provider 绑定）、Provider（IdP/验证码/存储/支付/通知）、授权（Casbin v3 RBAC/ABAC）、商业（产品/订单/支付/订阅）、基础设施（Webhook/LDAP/网关/MCP）、安全（IP 黑白名单/速率限制/WAF/RADIUS/Key 管理）、协议（SAML/CAS/Kerberos/WebAuthn/SCIM/RADIUS/DCR）、前端管理控制台。

**不在此次开发范围**：移动端原生 SDK、桌面端客户端应用、BI 数据分析平台、AI/ML 驱动的风险引擎（规则引擎先行）。

---

## 4. 需求编号治理

### 4.1 编号规则

需求编号一经分配不得复用、不得改义。各域编号前缀：

| 前缀 | 功能域 | 登记位置 |
|------|--------|---------|
| AUTH | 认证与登录 | functional.md §1 |
| ORG | 组织与多租户 | functional.md §2 |
| USER | 用户管理 | functional.md §3 |
| APP | 应用与 OAuth 客户端 | functional.md §4 |
| PROV | Provider 与 IdP 集成 | functional.md §5 |
| PERM | 授权与权限 | functional.md §6 |
| COMM | 商业与支付 | functional.md §7 |
| METER | 用量计量与配额 | functional.md §7.4 |
| INFRA | 基础设施与集成 | functional.md §8 |
| SEC | 安全与防护 | functional.md §9 |
| FE | 前端管理控制台 | functional.md §10 |
| REBAC | 关系与属性访问控制 | functional.md §12 |
| IGA | 身份治理与管理 | functional.md §13 |
| RISK | 身份风险与零信任 | functional.md §14 |
| B2B | B2B 组织与域名治理 | functional.md §15 |
| WLI | 工作负载身份 | functional.md §16 |
| EXT | 扩展与开发者体验 | functional.md §17 |
| DATA | 数据链路 | data.md §1.1 |
| DB-REQ | 多数据库支持 | non-functional.md §4 |
| PRIV | 合规与隐私 | non-functional.md §5 |
| DR | 容量与灾备 | non-functional.md §6 |

**定义源纪律**：

1. 需求编号的逐号定义唯一载体是本目录各文档的表格行；ADR、架构、验收条件等文档只能引用，不得定义。
2. 未列出条目的编号为保留号；引用保留号即缺陷，由 `python3 scripts/gen_traceability.py --check` 阻断。
3. 新增编号必须先落表格行，再在 ADR/验收条件中引用；章节标题的 `[REQ:… ~ …]` 仅声明编号空间，不产生逐号定义。

### 4.2 优先级定义

| 优先级 | 含义 |
|--------|------|
| P0 | Must Have — 阻塞发布 |
| P1 | Should Have — 本期交付 |
| P2 | Could Have — 计划中 |

### 4.3 编号分配细节

编号空间分配与未取用号段以本文件 §4 与 [functional.md](functional.md) 的 REQ 编号为准；验收条件唯一登记于 [acceptance-criteria.md](../acceptance-criteria.md)。

---

## 5. 关键用户旅程

### 5.1 新用户注册与 MFA

邀请验证 → 注册资料和密码校验 → 邮箱/手机号验证 → WebAuthn/TOTP 绑定 → 首次登录 → 会话创建。每一步都必须支持过期、重复提交、限流、审计和安全错误提示。

### 5.2 管理员排查用户问题

管理员按组织范围搜索用户，查看脱敏详情。模拟登录（Impersonate，USER-005）属于高风险操作，必须重新授权、设置时限、显示醒目状态、记录开始/结束事件，并在退出后撤销临时凭证。

### 5.3 开发者接入 OAuth/OIDC

创建应用 → 保存一次性 client secret → 精确配置 redirect URI → 使用 PKCE S256 → 兑换短期 access token → 旋转 refresh token → 撤销和审计。错误码必须稳定，不得泄露客户端密钥或用户存在性。

---

## 6. 外部接口参考

完整功能需求见 [functional.md](functional.md)；非功能需求见 [non-functional.md](non-functional.md)；数据需求见 [data.md](data.md)。
