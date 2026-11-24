# 应用域详细设计

> 设计锚点：[DESIGN:iam/applications]

> 目标态规范「应用域详细设计」；写作约束见 [docs/README.md](../../README.md)。

---

## 定位

应用（Application）是 GuliangIAM 中被保护资源的注册实体。应用可以是 OAuth2 Client、SAML SP、OIDC RP、CAS Client、RADIUS Client、SCIM Client 等。

## 应用类型

| 类型 | grant_types | 协议 |
|------|------------|------|
| SPA（public） | authorization_code（强制 PKCE） | OAuth2/OIDC |
| 机密客户端（confidential） | authorization_code + client_credentials + refresh_token | OAuth2/OIDC |
| SAML SP | — | SAML 2.0 HTTP-Redirect/POST |
| CAS Client | — | CAS 2.0/3.0 |
| RADIUS Client | — | RADIUS PAP/CHAP |
| SCIM Client | — | SCIM 2.0（Bearer Token） |

## OAuth Client 属性

| 属性 | 类型 | 说明 |
|------|------|------|
| client_id | TEXT | 唯一标识符 |
| client_secret | TEXT（argon2id 哈希存储） | 机密客户端凭据 |
| redirect_uris | TEXT[] | **精确等值匹配**，禁通配 |
| grant_types | TEXT[] | 白名单，服务端统一判定 |
| response_types | TEXT[] | 与 grant_types 独立元数据（禁从 grant_types 派生） |
| token_endpoint_auth_method | TEXT | client_secret_basic / client_secret_post / none / private_key_jwt / tls_client_auth / self_signed_tls_client_auth |
| app_oauth_config.token_signing_method | TEXT | ID Token 签名算法：`RS256`（默认）/ `SM2-SM3`；同一应用不得混签；JWKS 按算法族分端点发布（标准端点仅国际算法族，国密 SM2-SM3 由 `/.well-known/jwks-gm.json` 独立发布，Discovery 以 `gm_jwks_uri` 宣告），验签方按 `kid` 前缀在对应通道选择；事实源 `contracts/schema/20_identity.yaml`；边界见《[security-model.md](../../architecture/security-model.md)》§2.21 |
| pkce_required | BOOLEAN | **兼容列，不作门控**——全部客户端强制 S256 |
| custom_scopes | JSONB | 自定义 scope 集合 |
| code_expire | INTEGER | 授权码 TTL（默认 5min） |
| logout_url / logout_redirect_url | TEXT | RP-Initiated Logout 注册 |
| backchannel_logout_uri | TEXT | Back-Channel Logout 投递地址 |
| frontchannel_logout_uri | TEXT | Front-Channel Logout iframe 目标地址（OP 登出时加载） |
| frontchannel_logout_session_required | BOOLEAN | iframe 是否携带 `sid`（true 时带会话标识） |

**private_key_jwt 凭据**：内嵌 `jwks`（`app_client_jwks`）或远端 `jwks_uri`（`app_client_jwks_uris`）二选一或并存；`jwks_uri` 只接受 HTTPS，验签时经 SSRF-safe fetcher 拉取（5s 超时、64KiB 上限、成功缓存 5min、失败 fail-closed）。DCR 与控制台 Create/PATCH 均支持 `jwksUri`；控制台 UI 与字段说明同步见前端契约生成物。

## OAuth2 协议强制基线

- **PKCE**：全部客户端强制 S256（含机密客户端），`plain` 一律拒绝
- **redirect_uri**：精确等值匹配（scheme/host/port/path 逐字节相等），禁通配/子串/后缀
- **禁用 Implicit**：只接受 `code`，拒绝 `response_type=token`/`id_token`
- **禁用 Password Grant**：永不实现 Resource Owner Password Credentials
- **服务端 state Cookie**：`__Host-ars`（一次性消费、与 redirect_uri + nonce + code_challenge 绑定）

## Provider 绑定

应用可绑定外部身份提供者：

| Provider 类型 | 绑定方式 |
|---------------|---------|
| Google/GitHub/OIDC | `infra/idp/goth.go` + Goth 框架 |
| SAML IdP | `infra/saml/` + etree + dsig |
| LDAP | `infra/ldap/` + go-ldap/v3 |
| RADIUS | `infra/radius/` + layeh.com/radius |

## DCR（动态客户端注册）

OIDC Dynamic Client Registration（RFC 7591/7592）：

- 受保护端点（需管理员认证或初始 Bearer Token）
- 注册期校验 `ValidateRedirectURI`（拒绝通配/fragment/userinfo/非 https）
- DCR 走同一 `MatchRedirectURI` 校验器

## 密钥管理

应用的 OIDC 签名密钥按密钥版本与轮换状态机管理，支持滚动窗口（多证书并列）；轮换流程与 JWKS 算法族边界见《[oauth2.md](../protocol/oauth2.md)》§14.14，客户端密钥版本结构以 `contracts/schema/20_identity.yaml` 的 `app_client_secrets` 声明为准。应用专属 JWKS 端点：`/.well-known/{application}/jwks.json`。

## 配置保存与合并语义

应用详情页以锚点导航分区呈现（分区事实源：`web/src/pages/applications/ApplicationDetailPage.tsx` 的 `sectionNavItems` 与各 `section-*` 内容区），保存时**一次提交完整配置**，后端按「nil 未提交 / 显式空值清空」区分：

- 嵌套配置（注册项、登录项、登录方式、Provider items、SAML/Token attributes、SAML 单源字段）必须随 PATCH 持久化；只解码不写回被视为缺陷，回归见 `internal/app/applications/service_test.go`。
- Provider 绑定并入统一保存；SAML 为单一事实源，不再保留独立双写入口。
- 注册项编辑器只接受列内字段（name/visible/required/prompted/label），非法行在提交前拒绝。
- 保存成功后以详情读回值为准，禁止“提示成功但配置未生效”。

## 生命周期操作

### 复制（Duplicate）

ApplicationsPage 行内操作：拉取目标应用完整详情 → 前端清空 `clientId` / `clientSecret`（后端重新生成）→ 生成新 slug（`{原slug}-copy-{timestamp36}`）→ `POST /api/v1/applications` → 跳转至新应用详情页。

**安全约束**：后端 `getApplication` 响应含 `clientSecret` 哈希副本，复制路径必须前端显式置空。

### 导出（Export）

ApplicationsPage 行内操作 → `GET /api/v1/applications/{id}/export` → `ApplicationExportDTO`（**不含 clientSecret**）→ Blob 下载为 `{name}.json`。

导出字段白名单以 `internal/app/applications/export_import.go` 的 `ApplicationExportDTO` 定义为唯一事实源（backend 强制排除敏感字段；回归 `export_import_coverage_test.go`），文档不复制字段清单。

### 导入（Import）

ApplicationsPage 工具栏 Upload（`accept=".json"`）→ 浏览器端 `JSON.parse` → `POST /api/v1/applications/import`（body 含 `organizationId` + 上传 payload）。

**前置条件**：会话组织上下文存在（否则前置拦截提示"请先选择组织"）。后端校验 JSON schema + 组织归属权限。

| 操作 | 前端位置 | 权限 | 后端路径 |
|------|---------|------|---------|
| Duplicate | 行内 | `app:write` | 复用 `POST /api/v1/applications` |
| Export | 行内 | `app:read` | `GET /api/v1/applications/{id}/export` |
| Import | 工具栏 Upload | `app:write` | `POST /api/v1/applications/import` |

## Client Secret 与 DCR 管理凭据

| 项 | 规范 |
|---|---|
| 存储 | `client_secret` 仅存 Argon2id 哈希（`applications.client_secret_hash`），明文只在创建/轮换响应出现一次 |
| 轮换 | 轮换写 `application.client_secret.rotated` 审计；旧 secret 立即失效；并发轮换以 revision 乐观锁仲裁 |
| 认证方式 | 支持 `client_secret_basic` / `client_secret_post`；`client_secret_jwt` 不支持（OPEN-API-60 已关闭），DCR 传该值返回 400 `invalid_client_metadata` |
| DCR | `registration_access_token` 只存哈希；RFC 7592 管理端点必须携带该凭据；创建响应一次性返回 secret |
| 导入/导出 | 导入不得接收 secret 字段；导出不含 secret；`client_secret` 读面一律 `XxxSet` + `XxxMasked` |

## 公开投影与登录/注册 UI 定制

### 匿名公开投影

登录前发现读只暴露公开投影 `ApplicationPublicDetail`——固定 **35 个 JSON 键**的匿名契约（键数与字段清单以 `internal/app/applications/dto.go` 的 struct 定义为唯一事实源，复现：`sed -n '/^type ApplicationPublicDetail struct/,/^}/p' internal/app/applications/dto.go | grep -c 'json:'`；映射 `mapper.go::toPublicDetail`；回归 `internal/app/applications/public_projection_test.go`）：

- `GET /api/v1/applications/by-client-id/{clientId}` 与 `GET /api/v1/applications/resolve`（`orgSlug` + `appSlug` query）均声明 Permission `public`（路由声明事实源：`internal/transport/http/api/v1/applications/routes.go`）；resolve 与 by-client-id 收敛为同一公开投影，管理面继续使用 `ApplicationDetail`。
- resolve 的租户解析：只读事务内先经组织公开投影（`GetBySlugPublic`）解析组织 ID，再按 `organization_id` 显式过滤应用查询，不依赖匿名请求的租户上下文（用例 `internal/app/applications/get_application_by_slug.go`）。
- 投影只携带登录/注册壳实际消费的字段与公开品牌、UI 定制内容；SAML 材料、回调白名单、内部覆盖 URL、传输配置、令牌策略、IP/CORS 策略与凭据哈希一律不得进入公开投影——修复方式是字段缺席，而不是置空值。字段清单以 DTO 定义为准，文档不复制。

### 表单配置与定制列

- 应用表单配置存于 `app_form_config`（声明事实源：`contracts/schema/20_identity.yaml`；DDL 与迁移为生成物，复现 `python3 scripts/gen_schema.py --check`）。CSS/HTML 定制内容列为 TEXT、可空、无默认值（生成迁移 `db/postgresql/V17__app_form_config.sql` 建 TEXT 列、`V18__app_form_config.sql` 收敛为 nullable 无默认）：NULL 表达「未定制」，避免大文本默认值随每行复制。
- 脚本定制为废弃契约：`customJs` 是只读遗留列，PATCH 请求体携带 `customJs` 时被 `DisallowUnknownFields` 拒绝（400；回归 `internal/transport/http/api/v1/applications/customjs_deprecation_test.go`）；相邻 `customCss` / `customHeader` 仍是合法可写字段。公开投影不输出脚本内容，前端不支持执行应用提交的脚本。

### 控制台定制编辑

应用详情页 UI 定制 Tab（`web/src/pages/applications/tabs/UiCustomizationTab.tsx`）提供定制字段的代码编辑与预览，内建撤销/重做/重置（`web/src/pages/applications/useUiConfigHistory.ts`）：撤销/重做按字段级历史回放；重置分「恢复已保存值」与「恢复系统默认」两档，均需确认后生效。

### 登录/注册壳渲染链

LoginPage 与 SignupPage（`web/src/pages/auth/`）共用 `LoginShellLayout.tsx` 消费公开投影的定制内容：

- 自定义整页 HTML（signinHtml）优先级最高：非空时替代整个登录页，其余装饰不叠加。
- HTML 定制（headerHtml / footerHtml / signupHtml / formSideHtml 等）经 `sanitizeHtml`（DOMPurify）净化后渲染；CSS 定制经 `sanitizeCss` 净化并强制作用域：选择器统一加登录壳 scope 前缀（`LOGIN_SHELL_CSS_SCOPE`，`.login-content`），`:root`/`html`/`body` 映射到 scope 元素自身，应用 CSS 不波及壳外（portal/body）。净化事实源：`web/src/shared/lib/dom-purify.ts`。
- CSS 叠加顺序（同特异性时后者覆盖前者）：customCss（整壳通用）→ formCss（桌面表单）→ formCssMobile（移动覆盖）；真实页面按窗口宽度以 `@media` 生效，预览容器按断点模拟直接输出对应规则。
- 移动背景图走内联样式而非 `<style>` 注入：`sanitizeCss` 的 url() 值过滤会丢弃含 url() 的 background 声明，内联 background 由 React 经 CSSOM 赋值，无 HTML 注入面。
- 注册入口由 `enableSignup` 开关控制（`useSignupPageController.ts`）；组织选择模式由 `orgChoiceMode` 决定（登录路由上下文 `web/src/shared/lib/auth-route-context.ts`）；隐私政策/服务条款链接来自公开投影的 `privacyPolicy` / `termsOfUse`。

## API 事实源

应用域 API 端点清单以 `internal/transport/http/api/v1/applications/routes.go` 为准。
