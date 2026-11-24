# Casdoor → GuliangIAM 前端页面比对清单

> 设计锚点：[DESIGN:frontend/casdoor-parity-checklist]

> 定位：Casdoor 管理页到 GuliangIAM 页面的对齐决策台账（目标态约束，不含执行记录）。
>
> 事实源：页面、Tab 与字段数量以 `docs/design/frontend/registry.md` 的生成块与 `web/src/pages/**` 为准；页面接线由 `scripts/check-ui-wiring.py` 与 `scripts/check-frontend-coverage.py` 校验。本表只保留对齐决策（KEEP/ADD/ADAPT/REJECT），不登记实施结果；历史变更见仓库根 `CHANGELOG.md`。
> 范围：Casdoor `web/src/pages/` 管理页（排除 auth 登录链路）；逐页决策以 §1~§9 表格行为准。
>
> 决策语义：
> - **KEEP**：GLIAM 已有等价或更优实现，保持现有行为
> - **ADD**：Casdoor 有而 GLIAM 缺失，需新增
> - **ADAPT**：GLIAM 有对应实现，但字段/交互需对齐
> - **REJECT**：明确不做（架构分歧 / 安全 / 产品方向）
> - **NO_MATCH**：Casdoor 无对应项，不纳入对齐范围

***

## §1 Identity & Access Management 核心（32 页）

| # | Casdoor 页面 | 文件 | GLIAM 等价 | GLIAM 文件 | 决策 | 备注 |
|---|---|---|---|---|---|---|
| 1 | UserEditPage | UserEditPage.tsx | UserDetailPage（平铺分区） | users/UserDetailPage.tsx | **KEEP** | 分区 key 与旧链接别名折叠以 `users/user-detail-tabs.ts` 为唯一事实源（URL `?tab=` 定位），本文不复制分区清单。FaceId 内聚 MFA 区、Orders/Payments 为独立页（commerce 目录）、WebAuthn 归 Credentials 分区。Casdoor 仅 SingleEditPage ~30 字段 |
| 2 | UserListPage | UserListPage.tsx | UsersPage (16 列 + isProtected 删除保护 + Import/Export/Impersonate) | users/UsersPage.tsx | **KEEP** | 16 列（avatar/username/orgId/displayName/email/phone/realName/**affiliation**/**registerType**/userType/signupApp/status/isAdmin/isVerified/isForbidden/createdAt；见 design/iam/users.md 列表页规范）。Row Actions: Impersonate self-guard（username===account.name disabled）+ Delete isProtected（self \|\| built-in admin 禁用 + bulkDelete 过滤）。Casdoor 22 列（含 region/tag/balance×3/isDeleted/registerSource 等 GLIAM REJECT）；GLIAM 额外有 UserImportPage |
| 3 | UserCreatePage | UserCreatePage.tsx | UserCreatePage (4 Section ~40 字段) | users/UserCreatePage.tsx | **KEEP** | |
| 4 | OrganizationEditPage | OrganizationEditPage.tsx | OrganizationDetailPage（锚点分区） | organizations/OrganizationDetailPage.tsx | **KEEP** | 分区事实源：`OrganizationDetailPage.tsx` 的 `navItems`（profile/branding/signin-password/mfa/accounts/directory/finance 语义），本文不复制计数 — Casdoor 仅 4 Tab |
| 5 | OrganizationListPage | OrganizationListPage.tsx | OrganizationsPage (10 列 + built-in 删除保护 + Groups/Users quick-links) | organizations/OrganizationsPage.tsx | **ADAPT** | Casdoor **13 列**（name/createdTime/displayName/favicon/website/passwordType/passwordSalt/defaultAvatar/orgBalance/userBalance/balanceCredit/balanceCurrency/enableSoftDeletion）。GLIAM CrudListPage 10 列（slug 替代 name，passwordType/balance/currency 覆盖，passwordSalt/defaultAvatar 因安全/粒度 REJECT）。Row Actions: Edit → Groups→/groups → Users→/users → Delete（4 按钮 width 280；Casdoor 用路径 `/trees/{name}` + `/organizations/{name}/users` 被 GLIAM session+orgFilter 架构替代）。Built-in 删除保护：`slug==="built-in"` 行 Delete 禁用 + bulkDelete 过滤（见 design/iam/organizations.md） |
| 6 | OrganizationListPage (New modal) | OrganizationListPage.tsx | OrganizationCreatePage (4 Section ~15 字段) | organizations/OrganizationCreatePage.tsx | **ADAPT** | Casdoor 无独立创建页，在列表页内联 New（~7 字段）；GLIAM 独立创建页 4 Collapsible Section：Basic（slug regex + 6 基础）/ Branding（4 字段）/ PasswordPolicy（passwordType 默认 argon2id + 7 子表字段对齐 DB DEFAULT）/ SecuritySignin（5 开关，见 design/iam/organizations.md） |
| 7 | ApplicationEditPage | ApplicationEditPage.tsx | ApplicationDetailPage（锚点分区） | applications/ApplicationDetailPage.tsx | **KEEP** | 分区事实源：`ApplicationDetailPage.tsx` 的 `sectionNavItems` 与 `section-*` 内容区。Casdoor 仅 8 Tab，GLIAM 另有 CustomScopes + SigninLog 分区 |
| 8 | ApplicationListPage | ApplicationListPage.tsx | ApplicationsPage | applications/ApplicationsPage.tsx | **ADAPT** | Casdoor 行内 Edit/Delete + 工具栏 Add；GLIAM 行内 Duplicate（前端清空 clientId/clientSecret + 新 slug 规则）/ Export（ApplicationExportDTO 白名单不含敏感字段）+ Delete（异步 summary Popconfirm）；工具栏 Import（JSON Upload + 组织上下文前置校验），见 design/iam/applications.md 生命周期操作 |
| 9 | AppListPage | AppListPage.tsx | AppsPage | apps/AppsPage.tsx | **ADAPT** | 门户卡片应打开应用 `homepageUrl` 而非管理详情；List DTO/OpenAPI 需声明 `homepageUrl`；缺失时管理员回退详情、非管理员禁用；外链走 `isSafeExternalUrl` 校验。 |
| 10 | ProviderEditPage | ProviderEditPage.tsx | ProviderDetailPage (DynamicProviderFields) | providers/ProviderDetailPage.tsx | **ADAPT** | 动态字段桥接保持，绑定字段均由 `ProviderDetail`/`PatchProviderRequest` 支撑；`enableSignUp` 无表列/无 DTO，不提供该开关；开关与新增字段标签统一走 `resource.provider.*`（14 语言包同步）；HTTP Headers 用共享 `HttpHeaderTable` 结构化编辑；托管 kind（LDAP/验证码/Face ID）不在统一入口创建，历史行保留读取/停用/删除；Web3 类目 `creatable: false, filterable: true`（钱包签名端点 410，Passkey 走 WebAuthn 独立通道）。 |
| 11 | ProviderListPage | ProviderListPage.tsx | ProvidersPage | providers/ProvidersPage.tsx | **KEEP** | 保留统一列表与批量删除；筛选包含托管类目（历史行需可定位），创建按钮只提供非托管 kind；与 `provider-kinds.yaml` 的 `managed_via`/`consumer_table` 对齐。 |
| 12 | RoleEditPage | RoleEditPage.tsx | RoleDetailPage (2 Tabs + CasbinPolicyEditor) | roles/RoleDetailPage.tsx | **KEEP** | GLIAM 额外有 Casbin 策略编辑器 + 3 个 SubTable — Casdoor 仅 9 字段 SimpleEditPage |
| 13 | RoleListPage | RoleListPage.tsx | RolesPage | roles/RolesPage.tsx | **KEEP** | 列集差异保持（List DTO 无的字段不造伪列），引用关系由详情承载 |
| 14 | PermissionEditPage | PermissionEditPage.tsx | PermissionDetailPage (2 Tabs) | permissions/PermissionDetailPage.tsx | **KEEP** | Structured + Advanced 分组 |
| 15 | PermissionListPage | PermissionListPage.tsx | PermissionsPage | permissions/PermissionsPage.tsx | **KEEP** | |
| 16 | GroupEditPage | GroupEditPage.tsx | GroupDetailPage | groups/GroupDetailPage.tsx | **KEEP** | |
| 17 | GroupListPage | GroupListPage.tsx | GroupsPage | groups/GroupsPage.tsx | **KEEP** | |
| 18 | GroupTreePage | GroupTreePage.tsx | GroupTreePage | groups/GroupTreePage.tsx | **KEEP** | |
| 19 | RuleEditPage | RuleEditPage.tsx | RuleDetailPage | rules/RuleDetailPage.tsx | **KEEP** | |
| 20 | RuleListPage | RuleListPage.tsx | RulesPage | rules/RulesPage.tsx | **KEEP** | |
| 21 | ModelEditPage | ModelEditPage.tsx | ModelDetailPage | models/ModelDetailPage.tsx | **KEEP** | |
| 22 | ModelListPage | ModelListPage.tsx | ModelsPage | models/ModelsPage.tsx | **KEEP** | |
| 23 | EnforcerEditPage | EnforcerEditPage.tsx | EnforcerDetailPage | enforcers/EnforcerDetailPage.tsx | **KEEP** | |
| 24 | EnforcerListPage | EnforcerListPage.tsx | EnforcersPage | enforcers/EnforcersPage.tsx | **KEEP** | 列表列使用 List DTO 的 modelId/adapterId 链接（名称由 models/adapters options 解析）；不补 createdAt 列，列集差异保持（List DTO 无的字段不造伪列） |
| 25 | SessionListPage | SessionListPage.tsx | SessionsPage | sessions/SessionsPage.tsx | **KEEP** | |
| 26 | ResourceListPage | ResourceListPage.tsx | ResourcesPage | resources/ResourcesPage.tsx | **ADAPT** | **KEEP** + **ADAPT** —— **what**：列表已展示 `tag/fileSize/mimeType` 且不展示内部 `filePath`，但列头 `sorter` 与 `sortField/sortOrder` 透传给不读排序的 List query；**why**：`ResourceListItem` 已有前三列，`filePath` 属内部存储路径不应外露；后端 `ORDER BY created_at DESC` 固定，伪排序会让翻页顺序回退；**how**：保留列集与 Create/Detail，移除列头 `sorter`、默认排序与排序参数透传；**trade_off**：列表固定创建时间倒序，用户不能自定义排序 |
| 27 | RecordListPage | RecordListPage.tsx | RecordsPage | records/RecordsPage.tsx | **KEEP** | |
| 28 | VerificationListPage | VerificationListPage.tsx | VerificationsPage | verifications/VerificationsPage.tsx | **KEEP** | |
| 29 | TokenEditPage | TokenEditPage.tsx | TokenDetailPage + JwtDecodeView | tokens/ | **KEEP** | GLIAM 多一个 JWT 解码查看器；保持服务端签发、一次性结果与解码视图，不提供 raw access/refresh token 编辑 |
| 30 | TokenListPage | TokenListPage.tsx | TokensPage | tokens/TokensPage.tsx | **KEEP** | 只读列表 + 既有 revoke/delete；不新增签发入口 |
| 31 | AccountPage | AccountPage.tsx | AccountPage (5 Tabs) | account/AccountPage.tsx | **KEEP** | 实际 5 Tabs（profile/realname/logins/security/sessions；AccountPage.tsx L30-117），其中 security 下嵌套 4 Section（ContactBinding/Password/MFA/Web3Address）。Casdoor 仅 1 Tab + 动态 accountItems 配置（无 realname/logins/sessions 独立 Tab） |
| 32 | Dashboard | Dashboard.tsx | DashboardPage | dashboard/DashboardPage.tsx | **KEEP** | |

***

## §2 Authentication & Directory（11 页）

| # | Casdoor 页面 | 文件 | GLIAM 等价 | GLIAM 文件 | 决策 | 备注 |
|---|---|---|---|---|---|---|
| 33 | LdapEditPage | LdapEditPage.tsx | LdapDetailPage | ldaps/LdapDetailPage.tsx | **KEEP** | 详情不回显 bind 密码；空口令不随 PATCH 下发，保留已存值 |
| 34 | LdapSyncPage | LdapSyncPage.tsx | LdapSyncPage | ldaps/LdapSyncPage.tsx | **KEEP** | |
| 35 | IntegrationEditPage | IntegrationEditPage.tsx | — | — | **REJECT** | GLIAM 把集成拆为 LDAP/Syncers/Webhooks 三页，更清晰 |
| 36 | IntegrationListPage | IntegrationListPage.tsx | — | — | **REJECT** | 同上 |
| 37 | MarketplacePage | MarketplacePage.tsx | — | — | **REJECT** | 企业 IAM 不做公共应用市场 |
| 38 | InvitationEditPage | InvitationEditPage.tsx | InvitationDetailPage | invitations/InvitationDetailPage.tsx | **KEEP** | 详情页不得提供 `defaultCode`/`application`/`quota`/`usedCount` 配置区：`InvitationDTO`（`internal/app/invitations/dto.go`）与 `InvitationResponse`（`internal/transport/http/api/v1/invitations/response.go`）均无这些字段，前端 Zod schema 不得声明服务端不填充的伪字段；后端不扩字段（`quota`/`usedCount`/`application` 属 NO_MATCH） |
| 39 | InvitationListPage | InvitationListPage.tsx | InvitationsPage | invitations/InvitationsPage.tsx | **ADAPT** | 列表列 email/status/invitedBy/expiresAt/createdAt 均由 `InvitationDTO`（`internal/app/invitations/dto.go`）提供；不补 code/quota/usedCount/application（无 GLIAM List 字段，NO_MATCH）；create 按钮带 `invitation:write` PermissionGuard，revoke/resend 有失败提示与 expired 状态色；create/revoke/resend 能力保持，不扩后端 |
| 40 | FormEditPage | FormEditPage.tsx | FormDetailPage | forms/FormDetailPage.tsx | **ADAPT** | 表单字段需结构化编辑（name/label/visible/width）；`forms.form_items` 与 API/OpenAPI 字段及数据迁移属结构变更，须先立提案；在此之前 `properties` JSON 文本域保留校验与错误提示，长度受列宽限制。 |
| 41 | FormListPage | FormListPage.tsx | FormsPage | forms/FormsPage.tsx | **ADAPT** | **ADAPT** —— **what**：列表列与 Casdoor 一致（name/displayName/status/actions），但列头 `sorter` 与 `sortField/sortOrder` 透传给不读排序的 List query，且 Casdoor 的字段数列因 List DTO 无 `properties/formItems` 无法实现；**why**：后端 `ORDER BY created_at DESC` 固定，伪排序会让翻页顺序回退；字段计数需先有结构化字段契约；**how**：移除列头 `sorter`、默认排序与排序参数透传，保留现有列集；**trade_off**：列表固定创建时间倒序，用户不能自定义排序，暂不展示字段数 |
| 42 | SyncerEditPage | SyncerEditPage.tsx | SyncerDetailPage | syncers/SyncerDetailPage.tsx | **KEEP** | |
| 43 | SyncerListPage | SyncerListPage.tsx | SyncersPage | syncers/SyncersPage.tsx | **ADAPT** | **ADAPT** —— **what**：补 host:port、syncInterval、lastSyncAt 列（`SyncerListItem` 的 port/syncInterval/lastSyncAt）；移除后端 `ListSyncersQuery` 不存在的 type 筛选；删除按 record.id（rowKeyToDeleteKey）；**why**：列表列必须与 `SyncerListItem` 对齐，`ListSyncersRequest` 不读排序参数、SQL 固定 `ORDER BY created_at DESC`，前端透传 `sorter`/sortBy/sortOrder 只会制造伪交互；**how**：`SyncersPage` 去掉 `sorter` 与排序透传；同族 LdapsPage/CaptchaProvidersPage/FaceIdProvidersPage/RadiusConfigsPage 同样禁止排序透传（对应 List DTO 同样不读排序）；**trade_off**：列表固定创建时间倒序，用户不能自定义排序；`databaseType/user/password` 仅 Detail |

***

## §3 Certificate & Key Management（4 页）

| # | Casdoor 页面 | 文件 | GLIAM 等价 | GLIAM 文件 | 决策 | 备注 |
|---|---|---|---|---|---|---|
| 44 | CertEditPage | CertEditPage.tsx | CertDetailPage | certs/CertDetailPage.tsx | **KEEP** | |
| 45 | CertListPage | CertListPage.tsx | CertsPage | certs/CertsPage.tsx | **KEEP** | |
| 46 | KeyEditPage | KeyEditPage.tsx | KeyDetailPage | keys/KeyDetailPage.tsx | **KEEP** | |
| 47 | KeyListPage | KeyListPage.tsx | KeysPage | keys/KeysPage.tsx | **KEEP** | |

***

## §4 Commerce & Billing（22 页）

| # | Casdoor 页面 | 文件 | GLIAM 等价 | GLIAM 文件 | 决策 | 备注 |
|---|---|---|---|---|---|---|
| 48 | ProductEditPage | ProductEditPage.tsx | ProductDetailPage | products/ProductDetailPage.tsx | **KEEP** | |
| 49 | ProductListPage | ProductListPage.tsx | ProductsPage | products/ProductsPage.tsx | **ADAPT** | 列表列只用 List DTO 的 name/displayName/image/currency/price/state/createdAt；isRecharge 仅 Detail，降为服务端筛选、不加伪列；tag/quantity/sold/providers 仅 Detail，不展示（列集保持：仅 Detail 字段不造伪列） |
| 50 | ProductBuyPage | ProductBuyPage.tsx | ProductBuyPage | products/ProductBuyPage.tsx | **KEEP** | |
| 51 | ProductStorePage | ProductStorePage.tsx | ProductStorePage | products/ProductStorePage.tsx | **KEEP** | |
| 52 | CartListPage | CartListPage.tsx | CartPage | products/CartPage.tsx | **KEEP** | |
| 53 | OrderEditPage | OrderEditPage.tsx | OrderDetailPage | orders/OrderDetailPage.tsx | **KEEP** | |
| 54 | OrderListPage | OrderListPage.tsx | OrdersPage | orders/OrdersPage.tsx | **ADAPT** | 移除 OrderListItem 不存在的 type 列；补 user/price/createdAt（List DTO）；products/payment 仅 Detail 或详情承载，不造伪列（仅 Detail 字段） |
| 55 | OrderPayPage | OrderPayPage.tsx | OrderPayPage | orders/OrderPayPage.tsx | **KEEP** | |
| 56 | PaymentEditPage | PaymentEditPage.tsx | PaymentDetailPage | payments/PaymentDetailPage.tsx | **KEEP** | GLIAM 额外有 ChannelConfigsPage + ReconPage |
| 57 | PaymentListPage | PaymentListPage.tsx | PaymentsPage | payments/PaymentsPage.tsx | **ADAPT** | 补 provider/user/price/createdAt（List DTO）；type/state 保持；products/invoice 字段仅 Detail，不造伪列（仅 Detail 字段） |
| 58 | PaymentResultPage | PaymentResultPage.tsx | PaymentResultPage | payments/PaymentResultPage.tsx | **KEEP** | PaymentsCallbackReceiptsPage 为 GLIAM 独有的回单调据页（见 §8） |
| 59 | CouponEditPage | CouponEditPage.tsx | CouponDetailPage | coupons/CouponDetailPage.tsx | **KEEP** | |
| 60 | CouponListPage | CouponListPage.tsx | CouponsPage | coupons/CouponsPage.tsx | **ADAPT** | 补 createdAt；discount 按 discountType 渲染百分比；scope/usedCount/quantity/expireTime/currency 仅 Detail，不加列；discountType 筛选修复透传 |
| 61 | PlanEditPage | PlanEditPage.tsx | PlanDetailPage | plans/PlanDetailPage.tsx | **KEEP** | |
| 62 | PlanListPage | PlanListPage.tsx | PlansPage | plans/PlansPage.tsx | **ADAPT** | 移除 isExclusive 列（仅 Detail），保留 isExclusive 服务端筛选并修复透传；补 price/createdAt（List DTO）；role/product 仅 Detail，不造伪列（仅 Detail 字段） |
| 63 | PricingEditPage | PricingEditPage.tsx | PricingDetailPage | pricings/PricingDetailPage.tsx | **KEEP** | |
| 64 | PricingListPage | PricingListPage.tsx | PricingsPage | pricings/PricingsPage.tsx | **ADAPT** | 移除 application 伪列与后端未读取的 application 筛选；改用 List DTO 的 isEnabled/trialDuration/createdAt；application/plans 仅 Detail（仅 Detail 字段不造伪列） |
| 65 | PricingPage | PricingPage.tsx | PricingSelectPage | pricings/PricingSelectPage.tsx | **KEEP** | |
| 66 | SubscriptionEditPage | SubscriptionEditPage.tsx | SubscriptionDetailPage | subscriptions/SubscriptionDetailPage.tsx | **KEEP** | |
| 67 | SubscriptionListPage | SubscriptionListPage.tsx | SubscriptionsPage | subscriptions/SubscriptionsPage.tsx | **ADAPT** | 补 user/plan/startAt/endAt/createdAt（List DTO）；period/group/payment/autoRenew/pricing/expireAt 无列表字段，不造伪列（仅 Detail 字段） |
| 68 | TransactionEditPage | TransactionEditPage.tsx | TransactionDetailPage | transactions/TransactionDetailPage.tsx | **KEEP** | |
| 69 | TransactionListPage | TransactionListPage.tsx | TransactionsPage | transactions/TransactionsPage.tsx | **ADAPT** | 补 userId/providerType/createdAt（List DTO）；application/tag 无字段，不造伪列（仅 Detail 字段） |

***

## §5 API Infrastructure（9 页）

| # | Casdoor 页面 | 文件 | GLIAM 等价 | GLIAM 文件 | 决策 | 备注 |
|---|---|---|---|---|---|---|
| 70 | ServerEditPage | ServerEditPage.tsx | ServerDetailPage | gateway/ServerDetailPage.tsx | **KEEP** | GLIAM 归入 gateway 模块 |
| 71 | ServerListPage | ServerListPage.tsx | ServersPage | gateway/ServersPage.tsx | **ADAPT** | 补 createdAt 列（`ServerDTO`）；Casdoor 的 displayName/url/application 无 GLIAM List 字段（NO_MATCH），不造列 |
| 72 | SiteEditPage | SiteEditPage.tsx | SiteDetailPage | gateway/SiteDetailPage.tsx | **KEEP** | |
| 73 | SiteListPage | SiteListPage.tsx | SitesPage | gateway/SitesPage.tsx | **ADAPT** | 补 sslMode、lastHealthStatus 列（`SiteDTO`）；Casdoor 的 domain/otherDomains/hosts/nodes/rules/publicIp/casdoorApplication 无 GLIAM List 字段（NO_MATCH），不造列 |
| 74 | EntryEditPage | EntryEditPage.tsx | EntryDetailPage | entries/EntryDetailPage.tsx | **KEEP** | |
| 75 | EntryListPage | EntryListPage.tsx | EntriesPage | entries/EntriesPage.tsx | **ADAPT** | 移除 List DTO 不存在的 clientIp 列（仅 `EntryDetail`），改展示 organizationId（经 useOrganizationNames 解析）；provider/userAgent 仅 Detail，message 仅转录页（42.3.13 条 6），不造列 |
| 76 | WebhookEditPage | WebhookEditPage.tsx | WebhookDetailPage | webhooks/WebhookDetailPage.tsx | **KEEP** | |
| 77 | WebhookListPage | WebhookListPage.tsx | WebhooksPage | webhooks/WebhooksPage.tsx | **ADAPT** | 补 contentType、isError、createdAt 列（`WebhookListItem`）；状态筛选由 status 改为 isEnabled（`ListWebhooksRequest` 只读 isEnabled，原 status 为静默无效）；移除 name 列的 `sorter`（列表请求不发送也不接收 sortField/sortOrder，SQL 固定 `ORDER BY created_at DESC`）；events/isUserExtended 仅 Detail，不造列；`IamWebhookItem.revision` 移至 Detail（List DTO 无 revision） |
| 78 | WebhookEventListPage | WebhookEventListPage.tsx | WebhookEventsPage | webhook-events/WebhookEventsPage.tsx | **KEEP** | |

***

## §6 System & Support（6 页）

| # | Casdoor 页面 | 文件 | GLIAM 等价 | GLIAM 文件 | 决策 | 备注 |
|---|---|---|---|---|---|---|
| 79 | AdapterEditPage | AdapterEditPage.tsx | AdapterDetailPage | adapters/AdapterDetailPage.tsx | **KEEP** | |
| 80 | AdapterListPage | AdapterListPage.tsx | AdaptersPage | adapters/AdaptersPage.tsx | **KEEP** | 列表列只用 List DTO 的 type/server/database/useSameDb；databaseType 仅 Detail DTO，不展示伪列（仅 Detail DTO 字段） |
| 81 | AgentEditPage | AgentEditPage.tsx | AgentDetailPage | agents/AgentDetailPage.tsx | **KEEP** | |
| 82 | AgentListPage | AgentListPage.tsx | AgentsPage | agents/AgentsPage.tsx | **KEEP** | |
| 83 | TicketEditPage | TicketEditPage.tsx | TicketDetailPage | tickets/TicketDetailPage.tsx | **KEEP** | |
| 84 | TicketListPage | TicketListPage.tsx | TicketsPage | tickets/TicketsPage.tsx | **ADAPT** | 补 assignee、creator、createdAt 列（`TicketListItem`）；Casdoor 的 title 无 List 字段（Detail description），不造列；priority 保持 GLIAM 队列排序语义 |

***

## §7 Utility / Special（7 页）

| # | Casdoor 页面 | 文件 | GLIAM 等价 | GLIAM 文件 | 决策 | 备注 |
|---|---|---|---|---|---|---|
| 85 | SystemInfoPage | SystemInfoPage.tsx | SystemInfoPage | system-info/SystemInfoPage.tsx | **KEEP** | |
| 86 | ShortcutsPage | ShortcutsPage.tsx | ShortcutsPage | shortcuts/ShortcutsPage.tsx | **KEEP** | |
| 87 | ThemePreviewPage | ThemePreviewPage.tsx | ThemeEditorPage | theme-editor/ThemeEditorPage.tsx | **KEEP** | GLIAM 合并到独立 ThemeEditor + settings/ThemeTab |
| 88 | OpenClawSessionTranscriptPage | OpenClawSessionTranscriptPage.tsx | EntryTranscriptPage | entries/EntryTranscriptPage.tsx | **KEEP** | 更通用的 Entry 转录页 |
| 89 | ServerStorePage | ServerStorePage.tsx | — | — | **REJECT** | Casdoor 特有 AI Agent Server Store；GLIAM 有独立 ai-gateway/providers |
| 90 | QrCodePage | QrCodePage.tsx | OrderPayPage（内联 QR + 轮询） | orders/OrderPayPage.tsx（`/orders/:id/pay`） | **KEEP** | Casdoor `QrCodePage` 是**支付二维码页**（`/qrcode/:owner/:paymentName`，轮询 `/api/notify-payment`；见 `QrCodePage.tsx:9-95`、`App.tsx:253`），不是登录二维码；GLIAM 在 `OrderPayPage` 内联 QR + 轮询，不提供独立未登录支付落地页 |
| 91 | NotFoundPage | NotFoundPage.tsx | RouteErrorBoundary（router 级 404） | app/router/shared.tsx | **KEEP** | 404 由 router 错误边界统一渲染，不在 pages/ 目录 |

***

## §8 GLIAM 独有页面（Casdoor 无对应）

按功能域分组（页面清单与计数以 `docs/design/frontend/registry.md` 生成块为唯一事实源），体现 GLIAM 在企业 IAM 方向的扩展：

### AI Gateway

| GLIAM 页面 | 说明 |
|---|---|
| ai-gateway/aliases/AIGatewayAliasesPage | 模型别名映射 |
| ai-gateway/api-keys/AIGatewayAPIKeysPage | API Key 管理 |
| ai-gateway/health/AIGatewayHealthPage | 网关健康检查 |
| ai-gateway/models/AIGatewayModelsPage | 模型注册表 |
| ai-gateway/prices/AIGatewayPricesPage | 模型价格表 |
| ai-gateway/providers/* (Create/Detail/Test/List) | LLM Provider 管理 |
| ai-gateway/quotas/AIGatewayQuotasPage | 配额管理 |
| ai-gateway/routes/AIGatewayRoutesPage | 路由策略 |
| ai-gateway/usage/AIGatewayUsagePage | 使用量查询 |
| ai-gateway/usage-stats/AIGatewayUsageStatsPage | 使用统计 |

### Identity Governance & Administration

| GLIAM 页面 | 说明 |
|---|---|
| iga/IgaPage（单页多 Tab，Tab 清单以 `web/src/pages/iga/` 目录为事实源） | 身份治理：访问请求、Campaign、职责分离、生命周期、特权会话 |

### Relationship-Based Access Control

| GLIAM 页面 | 说明 |
|---|---|
| rebac/ (Namespaces / Tuples) | ReBAC 关系元组授权 |

### Workload Identity

| GLIAM 页面 | 说明 |
|---|---|
| workload/ (FederationTrusts / IdentityPools / ServiceAccounts) | Workload 联邦、身份池、SA |

### Risk Engine

| GLIAM 页面 | 说明 |
|---|---|
| risk/ (Decisions / Signals / SessionGovernance) | 风险决策、信号、会话治理 |

### Metering

| GLIAM 页面 | 说明 |
|---|---|
| metering/ (Events / Aggregates) | 用量计量事件与聚合 |

### 安全与认证增强

| GLIAM 页面 | 说明 |
|---|---|
| captcha-providers/ (Create/Detail/List) | Captcha Provider 独立管理 |
| face-id-providers/ (Create/Detail/List) | Face ID Provider 独立管理 |
| notification-providers/ (Create/Detail/List) | 通知 Provider 独立管理 |
| mfa/MfaSetupPage | MFA 设置独立页 |
| device/DeviceVerifyPage | 设备验证 |
| idv/IDVPage | 身份验证 |
| radius-configs/ + radius-accountings/ | RADIUS 配置与计费 |
| mcp-servers/ + McpStorePage | MCP Server 注册与 Store |
| org-domains/OrgDomainsPage | 组织域名 |

### 审计与运维

| GLIAM 页面 | 说明 |
|---|---|
| actions/ (ActionList/ActionLogs/ActionVersions) | 审计增强：操作日志 + 版本链 |
| notifications/NotificationListPage | 消息中心 |
| i18n/ (Languages/Translations) | 国际化后台管理 |
| migration/MigrationPage | 数据迁移工具 |
| settings/ (General/Security/Theme Tabs) | 系统设置 |
| swagger/SwaggerPage | API 文档（Scalar） |
| users/UserImportPage | 批量导入（在 Import 按钮之外提供独立路径） |
| payments/PaymentsChannelConfigsPage + ReconPage | 支付渠道配置 + 对账 |
| payments/PaymentsCallbackReceiptsPage | 支付回单调据独立页 |

***

## §9 Casdoor 独有页面（GLIAM 没有 — 全部 REJECT）

| Casdoor 页面 | 决策 | 理由 |
|---|---|---|
| MarketplacePage | **REJECT** | 企业 IAM 不做公共应用市场 |
| IntegrationEditPage + IntegrationListPage | **REJECT** | GLIAM 已分散为 LDAP/Syncers/Webhooks，更清晰 |
| ServerStorePage | **REJECT** | Casdoor AI Agent Server Store 产品方向；GLIAM 走独立 ai-gateway |

***
